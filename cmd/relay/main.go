package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/crypto"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/pool"
	"github.com/yansircc/llm-broker/internal/relay"
	"github.com/yansircc/llm-broker/internal/server"
	"github.com/yansircc/llm-broker/internal/store"
	"github.com/yansircc/llm-broker/internal/tokens"
	"github.com/yansircc/llm-broker/internal/transport"
)

var version = "dev"

func main() {
	cfg := config.Load()
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := store.Migrate(cfg.DBPath); err != nil {
			slog.Error("database migration failed", "path", cfg.DBPath, "error", err)
			os.Exit(1)
		}
		slog.Info("database migration complete", "path", cfg.DBPath)
		return
	}

	if err := cfg.Validate(); err != nil {
		slog.Error("config validation failed", "error", err)
		os.Exit(1)
	}

	// Setup logging with ring buffer handler
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logHandler := events.NewLogHandler(level, 1000)
	slog.SetDefault(slog.New(logHandler))
	slog.Info("llm-broker starting", "version", version)

	// Open SQLite database
	s, err := store.New(cfg.DBPath)
	if err != nil {
		slog.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer s.Close()
	slog.Info("database ready", "path", cfg.DBPath)

	// Initialize crypto
	c := crypto.New(cfg.EncryptionKey)
	if _, err := c.DeriveKey("salt"); err != nil {
		slog.Error("key derivation failed", "error", err)
		os.Exit(1)
	}
	slog.Info("encryption key derived")

	// Initialize event bus
	bus := events.NewBus(200)

	// Initialize shared transport pool
	transportPool := transport.NewPool(cfg.RequestTimeout)
	defer transportPool.Close()

	// Initialize pool (loads all accounts from DB)
	p, err := pool.New(s, bus)
	if err != nil {
		slog.Error("pool init failed", "error", err)
		os.Exit(1)
	}

	// Initialize auth middleware
	authMw := auth.NewMiddleware(cfg.StaticToken, s)

	// Initialize drivers
	pauses := driver.ErrorPauses{
		Pause401:        cfg.ErrorPause401,
		Pause401Refresh: cfg.ErrorPause401Refresh,
		Pause403:        cfg.ErrorPause403,
		Pause429:        cfg.ErrorPause429,
		Pause529:        cfg.ErrorPause529,
	}
	drivers := map[domain.Provider]driver.Driver{
		domain.ProviderClaude: driver.NewClaudeDriver(driver.ClaudeConfig{
			APIURL:        cfg.ClaudeAPIURL,
			APIVersion:    cfg.ClaudeAPIVersion,
			BetaHeader:    cfg.ClaudeBetaHeader,
			Pauses:        pauses,
			PromptEnvHome: cfg.PromptEnvHome,
		}, p, cfg.MaxCacheControls),
		domain.ProviderCodex: driver.NewCodexDriver(driver.CodexConfig{
			APIURL: cfg.CodexAPIURL,
			Pauses: pauses,
		}),
	}
	if cfg.GeminiEnabled() {
		drivers[domain.ProviderGemini] = driver.NewGeminiDriver(driver.GeminiConfig{
			APIURL:            cfg.GeminiAPIURL,
			OAuthClientID:     cfg.GeminiOAuthClientID,
			OAuthClientSecret: cfg.GeminiOAuthClientSecret,
			OAuthRedirectURI:  cfg.GeminiOAuthRedirectURI,
			Pauses:            pauses,
		})
	}

	executionDrivers := make(map[domain.Provider]driver.ExecutionDriver, len(drivers))
	schedulerDrivers := make(map[domain.Provider]driver.SchedulerDriver, len(drivers))
	refreshDrivers := make(map[domain.Provider]driver.RefreshDriver, len(drivers))
	catalogDrivers := make(map[domain.Provider]driver.Descriptor, len(drivers))
	oauthDrivers := make(map[domain.Provider]driver.OAuthDriver, len(drivers))
	adminDrivers := make(map[domain.Provider]driver.AdminDriver, len(drivers))
	for provider, drv := range drivers {
		executionDrivers[provider] = drv
		schedulerDrivers[provider] = drv
		refreshDrivers[provider] = drv
		catalogDrivers[provider] = drv
		oauthDrivers[provider] = drv
		adminDrivers[provider] = drv
	}

	// Initialize token manager
	tokMgr := tokens.NewManager(p, c, transportPool, cfg.TokenRefreshAdvance, cfg.CellErrorPause, refreshDrivers)
	p.SetDrivers(schedulerDrivers)

	// Wire 401 → background token refresh
	p.SetOnAuthFailure(func(accountID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := tokMgr.ForceRefresh(ctx, accountID); err != nil {
			slog.Error("401 background refresh failed", "accountId", accountID, "error", err)
			bus.Publish(events.Event{
				Type:      events.EventRefresh,
				AccountID: accountID,
				Message:   "background refresh failed: " + err.Error(),
			})
		} else {
			bus.Publish(events.Event{
				Type:      events.EventRecover,
				AccountID: accountID,
				Message:   "background refresh succeeded",
			})
		}
	})

	// Initialize relay
	blobDir := ""
	if cfg.LogBlobs {
		blobDir = filepath.Join(filepath.Dir(cfg.DBPath), "request-log-blobs")
	}
	r := relay.New(p, tokMgr, s, relay.Config{
		MaxRequestBodyMB:  cfg.MaxRequestBodyMB,
		MaxRetryAccounts:  cfg.MaxRetryAccounts,
		SessionBindingTTL: cfg.SessionBindingTTL,
		CellErrorPause:    cfg.CellErrorPause,
		TraceCompat:       cfg.TraceCompat,
		RequestLogBlobDir: blobDir,
	}, transportPool, bus, executionDrivers)

	// Start server
	ctx := context.Background()
	srv := server.New(cfg, s, p, tokMgr, r, transportPool, authMw, bus, version, server.DriverViews{
		Catalog: catalogDrivers,
		OAuth:   oauthDrivers,
		Admin:   adminDrivers,
	})
	if err := srv.Run(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/yansircc/llm-broker/internal/admission"
	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/billing"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/crypto"
	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
	"github.com/yansircc/llm-broker/internal/events"
	"github.com/yansircc/llm-broker/internal/pool"
	"github.com/yansircc/llm-broker/internal/relay"
	"github.com/yansircc/llm-broker/internal/requestlog"
	"github.com/yansircc/llm-broker/internal/server"
	"github.com/yansircc/llm-broker/internal/settings"
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

	settingsSvc := settings.NewService(s, c, cfg.EncryptionKey)
	startupCtx := context.Background()
	if err := settingsSvc.SeedFromConfig(startupCtx, cfg); err != nil {
		slog.Error("settings seed failed", "error", err)
		os.Exit(1)
	}
	if err := settingsSvc.ApplyToConfig(startupCtx, cfg); err != nil {
		slog.Error("settings apply failed", "error", err)
		os.Exit(1)
	}

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
	codexDriver := driver.NewCodexDriver(driver.CodexConfig{
		APIURL: cfg.CodexAPIURL,
		Pauses: pauses,
	})
	openAICompatibleDriver := driver.NewOpenAICompatibleDriver(pauses)

	executionDrivers := map[domain.Provider]driver.ExecutionDriver{
		domain.ProviderCodex:            codexDriver,
		domain.ProviderOpenAICompatible: openAICompatibleDriver,
	}
	schedulerDrivers := map[domain.Provider]driver.SchedulerDriver{
		domain.ProviderCodex:            codexDriver,
		domain.ProviderOpenAICompatible: openAICompatibleDriver,
	}
	refreshDrivers := map[domain.Provider]driver.RefreshDriver{
		domain.ProviderCodex: codexDriver,
	}
	catalogDrivers := map[domain.Provider]driver.Descriptor{
		domain.ProviderCodex: codexDriver,
	}
	oauthDrivers := map[domain.Provider]driver.OAuthDriver{
		domain.ProviderCodex: codexDriver,
	}
	adminDrivers := map[domain.Provider]driver.AdminDriver{
		domain.ProviderCodex:            codexDriver,
		domain.ProviderOpenAICompatible: openAICompatibleDriver,
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
	blobDir := requestlog.ResolveBlobDir(cfg.DBPath, cfg.LogBlobsMode)
	s.SetLogBlobDir(blobDir)
	r := relay.New(p, tokMgr, s, relay.Config{
		MaxRequestBodyMB:   cfg.MaxRequestBodyMB,
		MaxRetryAccounts:   cfg.MaxRetryAccounts,
		SessionBindingTTL:  cfg.SessionBindingTTL,
		CellErrorPause:     cfg.CellErrorPause,
		TraceCompat:        cfg.TraceCompat,
		RequestLogBlobDir:  blobDir,
		RequestLogBlobMode: cfg.LogBlobsMode,
		FallbackProviders: map[domain.Provider][]domain.Provider{
			domain.ProviderCodex: {domain.ProviderOpenAICompatible},
		},
	}, transportPool, bus, executionDrivers)
	billingSvc := billing.NewService(s)
	admissionSvc := admission.NewService(s, billingSvc)
	r.SetCommercialServices(billingSvc, admissionSvc)

	// Start server
	ctx := context.Background()
	srv := server.New(cfg, s, p, tokMgr, r, transportPool, authMw, bus, version, settingsSvc, server.DriverViews{
		Catalog: catalogDrivers,
		OAuth:   oauthDrivers,
		Admin:   adminDrivers,
	})
	if err := srv.Run(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

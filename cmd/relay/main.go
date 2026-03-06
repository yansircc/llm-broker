package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/yansir/cc-relayer/internal/auth"
	"github.com/yansir/cc-relayer/internal/config"
	"github.com/yansir/cc-relayer/internal/crypto"
	"github.com/yansir/cc-relayer/internal/events"
	"github.com/yansir/cc-relayer/internal/identity"
	"github.com/yansir/cc-relayer/internal/oauth"
	"github.com/yansir/cc-relayer/internal/pool"
	"github.com/yansir/cc-relayer/internal/relay"
	"github.com/yansir/cc-relayer/internal/server"
	"github.com/yansir/cc-relayer/internal/store"
	"github.com/yansir/cc-relayer/internal/transport"
)

var version = "dev"

func main() {
	// Load configuration
	cfg := config.Load()
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
	slog.Info("cc-relayer starting", "version", version)

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

	// Initialize transport manager
	tm := transport.NewManager(cfg.RequestTimeout)
	defer tm.Close()

	// Initialize pool (loads all accounts from DB)
	p, err := pool.New(s, bus, pool.ErrorPauses{
		Pause401:        cfg.ErrorPause401,
		Pause401Refresh: cfg.ErrorPause401Refresh,
		Pause403:        cfg.ErrorPause403,
		Pause429:        cfg.ErrorPause429,
		Pause529:        cfg.ErrorPause529,
	})
	if err != nil {
		slog.Error("pool init failed", "error", err)
		os.Exit(1)
	}

	// Initialize token manager
	tokMgr := oauth.NewTokenManager(p, c, tm, cfg.TokenRefreshAdvance)

	// Wire 401 → background token refresh
	p.SetOnAuthFailure(func(accountID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := tokMgr.ForceRefresh(ctx, accountID); err != nil {
			slog.Error("401 background refresh failed", "accountId", accountID, "error", err)
		}
	})

	// Initialize auth middleware
	authMw := auth.NewMiddleware(cfg.StaticToken, s)

	// Initialize identity transformer
	trans := identity.NewTransformer(p, cfg.MaxCacheControls)

	// Initialize relay
	r := relay.New(p, tokMgr, trans, s, relay.Config{
		ClaudeAPIURL:      cfg.ClaudeAPIURL,
		ClaudeAPIVersion:  cfg.ClaudeAPIVersion,
		ClaudeBetaHeader:  cfg.ClaudeBetaHeader,
		CodexAPIURL:       cfg.CodexAPIURL,
		MaxRequestBodyMB:  cfg.MaxRequestBodyMB,
		MaxRetryAccounts:  cfg.MaxRetryAccounts,
		SessionBindingTTL: cfg.SessionBindingTTL,
	}, tm, bus)

	// Start server
	ctx := context.Background()
	srv := server.New(cfg, s, p, tokMgr, r, tm, authMw, bus, version)
	if err := srv.Run(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

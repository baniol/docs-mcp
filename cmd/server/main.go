package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/baniol/docs-mcp/internal/config"
	"github.com/baniol/docs-mcp/internal/handlers"
	"github.com/baniol/docs-mcp/internal/repo"
	"github.com/baniol/docs-mcp/internal/search"
	"github.com/baniol/docs-mcp/internal/server"
	"github.com/baniol/docs-mcp/internal/syncer"
	"github.com/baniol/docs-mcp/internal/utils"
	"github.com/joho/godotenv"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	// Load .env file if present (silently ignored if missing)
	if err := godotenv.Load(); err == nil {
		slog.Info("loaded .env file")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	cfg.Version = version

	// Configure structured logging
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	slog.Info("starting docs-mcp",
		"repo", cfg.GithubRepo,
		"branch", cfg.GithubBranch,
		"port", cfg.Port,
		"sync_interval", cfg.SyncInterval,
	)

	// Initialize repo client
	repoClient := repo.NewClient(cfg)
	if err := repoClient.Initialize(); err != nil {
		slog.Error("repo init failed", "err", err)
		os.Exit(1)
	}

	// Build BM25 search index
	searcher := search.NewBM25Index()
	cache := utils.NewCache(time.Duration(cfg.CacheTTL)*time.Second, cfg.CacheMaxEntries)

	handler := handlers.New(cfg, repoClient, searcher, cache)

	slog.Info("building initial search index...")
	if err := handler.BuildIndex(); err != nil {
		slog.Error("index build failed", "err", err)
		os.Exit(1)
	}

	// Start background syncer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	syncCallback := func(newHash string) {
		slog.Info("repo updated, rebuilding index", "hash", newHash)
		if err := handler.BuildIndex(); err != nil {
			slog.Error("index rebuild failed", "err", err)
		}
		handler.InvalidateCache()
	}
	s := syncer.New(repoClient, time.Duration(cfg.SyncInterval)*time.Second, syncCallback)
	go s.Start(ctx)

	// Start HTTP server
	srv := server.New(cfg, handler)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	select {
	case <-quit:
	case err := <-errCh:
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
	slog.Info("shutdown signal received")
	cache.Stop()
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("server stopped")
}

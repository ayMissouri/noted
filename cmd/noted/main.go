package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ahmedmissouri/noted/internal/config"
	"github.com/ahmedmissouri/noted/internal/markdown"
	"github.com/ahmedmissouri/noted/internal/notes"
	"github.com/ahmedmissouri/noted/internal/server"
	"github.com/ahmedmissouri/noted/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "noted:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	_ = args
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	logger := newLogger(cfg)

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir %s: %w", cfg.DataDir, err)
	}
	dbPath := filepath.Join(cfg.DataDir, "noted.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if cfg.AutoMigrate {
		ran, err := storage.Migrate(context.Background(), db, storage.Migrations())
		if err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		if ran > 0 {
			logger.Info("applied migrations", "count", ran)
		}
	}
	logger.Info("database ready", "path", dbPath)

	notesSvc := notes.NewService(db)
	vault, err := notesSvc.EnsureDefaultVault(context.Background())
	if err != nil {
		return err
	}
	logger.Info("vault ready", "id", vault.ID, "name", vault.Name)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := server.New(cfg, logger, notesSvc, markdown.NewRenderer())
	return srv.Run(ctx)
}

func newLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ahmedmissouri/noted/internal/config"
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
	fmt.Println("noted: database ready; no server yet")
	return nil
}

func newLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

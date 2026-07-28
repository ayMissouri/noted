package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ayMissouri/noted/internal/auth"
	"github.com/ayMissouri/noted/internal/buildinfo"
	"github.com/ayMissouri/noted/internal/config"
	"github.com/ayMissouri/noted/internal/markdown"
	"github.com/ayMissouri/noted/internal/notes"
	"github.com/ayMissouri/noted/internal/server"
	"github.com/ayMissouri/noted/internal/storage"
	"github.com/ayMissouri/noted/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "noted:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "serve":
		return serve()
	case "demo":
		return demo()
	case "version":
		info := buildinfo.Get()
		fmt.Printf("noted %s", info.Version)
		if info.Commit != "" {
			fmt.Printf(" commit %.12s", info.Commit)
		}
		if info.BuiltAt != "" {
			fmt.Printf(" built %s", info.BuiltAt)
		}
		fmt.Println()
		return nil
	default:
		return fmt.Errorf("unknown command %q, want serve, demo, or version", cmd)
	}
}

type app struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *sql.DB
	notes  *notes.Service
	render *markdown.Renderer
}

func setup() (*app, func(), error) {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return nil, nil, err
	}
	logger := newLogger(cfg)

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create data dir %s: %w", cfg.DataDir, err)
	}
	dbPath := filepath.Join(cfg.DataDir, "noted.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, nil, err
	}
	if cfg.AutoMigrate {
		ran, err := storage.Migrate(context.Background(), db, storage.Migrations())
		if err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("migrate: %w", err)
		}
		if ran > 0 {
			logger.Info("applied migrations", "count", ran)
		}
	}
	logger.Info("database ready", "path", dbPath)
	renderer := markdown.NewRenderer()
	return &app{
		cfg: cfg, logger: logger, db: db,
		notes: notes.NewService(db, renderer), render: renderer,
	}, func() { db.Close() }, nil
}

func serve() error {
	a, cleanup, err := setup()
	if err != nil {
		return err
	}
	defer cleanup()

	vault, err := a.notes.EnsureDefaultVault(context.Background())
	if err != nil {
		return err
	}
	a.logger.Info("vault ready", "id", vault.ID, "name", vault.Name)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := server.New(a.cfg, a.logger, a.notes, auth.NewService(a.db), a.render, web.Dist())
	return srv.Run(ctx)
}

func newLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

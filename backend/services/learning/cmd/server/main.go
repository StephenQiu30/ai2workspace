// Command server starts the learning HTTP service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	learning "github.com/StephenQiu30/ai2workspace/backend/services/learning"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/courses"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/media"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/progress"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/server"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// @title learning API
// @version 0.1.0
// @description learning service API, generated from Go source.
// @BasePath /
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("service stopped", "service", "learning", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		coursesStore  courses.Store
		mediaStore    media.Store
		progressStore progress.Store
		pool          *pgxpool.Pool
	)

	if cfg.DatabaseURL != "" {
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		pool, err = pgxpool.New(dbCtx, cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("connect postgres: %w", err)
		}
		defer pool.Close()

		if err := pool.Ping(dbCtx); err != nil {
			return fmt.Errorf("ping postgres: %w", err)
		}
		if _, err := pool.Exec(dbCtx, learning.SchemaSQL); err != nil {
			return fmt.Errorf("initialize learning schema: %w", err)
		}

		coursesStore = courses.NewPGStore(pool)
		mediaStore = media.NewPGStore(pool)
		progressStore = progress.NewPGStore(pool)
		slog.Info("learning service connected to PostgreSQL")
	} else {
		coursesStore = courses.NewMemoryStore()
		mediaStore = media.NewMemoryStore()
		progressStore = progress.NewMemoryStore()
		slog.Info("learning service using in-memory store (DATABASE_URL unset)")
	}

	gin.SetMode(gin.ReleaseMode)
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Handler:           server.NewFromConfig(cfg, coursesStore, mediaStore, progressStore),
		ReadHeaderTimeout: cfg.HTTPTimeout,
		ReadTimeout:       cfg.HTTPTimeout,
		WriteTimeout:      cfg.HTTPTimeout,
		IdleTimeout:       cfg.HTTPTimeout,
	}

	stopped := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if shutdownErr := httpServer.Shutdown(shutdownCtx); shutdownErr != nil {
			slog.Error("graceful shutdown failed", "service", "learning", "error", shutdownErr)
		}
		close(stopped)
	}()

	slog.Info("learning service listening", "addr", cfg.HTTPAddr)
	if serveErr := httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	<-stopped
	return nil
}

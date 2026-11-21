// Command server starts the commerce HTTP service.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/server"
	"github.com/gin-gonic/gin"
)

// @title commerce API
// @version 0.1.0
// @description commerce service API, generated from Go source.
// @BasePath /
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("service stopped", "service", "commerce", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	gin.SetMode(gin.ReleaseMode)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	router, cleanup, err := server.NewFromConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTPTimeout,
		ReadTimeout:       cfg.HTTPTimeout,
		WriteTimeout:      cfg.HTTPTimeout,
		IdleTimeout:       cfg.HTTPTimeout,
	}
	stopped := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "service", "commerce", "error", err)
		}
		close(stopped)
	}()
	slog.Info("commerce service starting", "addr", cfg.HTTPAddr)
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-stopped
	return nil
}

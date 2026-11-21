// Command server starts the assistant HTTP service.
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
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/server"
	"github.com/gin-gonic/gin"
)

// @title assistant API
// @version 0.2.0
// @description assistant service API (Knowledge Base, AI Governance, Agent & Search), generated from Go source.
// @BasePath /
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("service stopped", "service", "assistant", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	gin.SetMode(gin.ReleaseMode)
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg)
	defer srv.Close()

	// Ensure WriteTimeout accommodates 18s evidence + 60s model generation.
	writeTimeout := cfg.HTTPTimeout
	if writeTimeout < 90*time.Second {
		writeTimeout = 90 * time.Second
	}

	httpServer := &http.Server{
		Handler:           srv.Router(),
		ReadHeaderTimeout: cfg.HTTPTimeout,
		ReadTimeout:       cfg.HTTPTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       writeTimeout,
	}
	stopped := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "service", "assistant", "error", err)
		}
		close(stopped)
	}()

	slog.Info("assistant service started", "addr", cfg.HTTPAddr)
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-stopped
	return nil
}

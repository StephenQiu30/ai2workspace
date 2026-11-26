// Command server runs the ai2workspace modular backend.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/StephenQiu30/ai2workspace/backend/internal/platform"
	"github.com/StephenQiu30/ai2workspace/backend/internal/server"
)

// @title ai2workspace API
// @version 0.1.0
// @description Modular backend for ai2workspace.
// @BasePath /
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("STUDIO_DATABASE_URL")
	if dsn == "" {
		return errors.New("STUDIO_DATABASE_URL is required")
	}
	db, pool, err := platform.OpenDatabase(dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	session, err := identity.NewSessionClient(os.Getenv("STUDIO_IDENTITY_ORIGIN"))
	if err != nil {
		return err
	}
	api := server.NewRouter(identity.NewHandler(db, session), os.Getenv("STUDIO_SWAGGER_ENABLED") == "true")
	addr := os.Getenv("STUDIO_HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:3128"
	}
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           api,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       time.Minute,
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			_ = httpServer.Close()
		}
	}()
	slog.Info("server starting", "address", addr)
	err = httpServer.ListenAndServe()
	stop()
	<-stopped
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

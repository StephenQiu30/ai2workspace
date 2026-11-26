// Command ai2workspace runs the ai2workspace modular backend.
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

	_ "github.com/StephenQiu30/ai2workspace/backend/api" // Register the generated Swagger document.
	"github.com/StephenQiu30/ai2workspace/backend/internal/agent"
	"github.com/StephenQiu30/ai2workspace/backend/internal/app"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/StephenQiu30/ai2workspace/backend/internal/infra/config"
	"github.com/StephenQiu30/ai2workspace/backend/internal/infra/postgres"
	"github.com/gin-gonic/gin"
)

// @title ai2workspace API
// @version 0.1.0
// @description Modular backend for ai2workspace.
// @BasePath /
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		slog.Error("Server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load server configuration: %w", err)
	}
	agentCfg, err := agent.LoadConfig()
	if err != nil {
		return fmt.Errorf("load agent configuration: %w", err)
	}
	startupCtx, cancelStartup := context.WithTimeout(ctx, 15*time.Second)
	defer cancelStartup()
	pool, err := postgres.Open(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	accounts := identity.New(pool, identity.Config{
		CookieSecure:    cfg.CookieSecure,
		DevExposeTokens: cfg.DevExposeTokens,
		AuthLimit:       cfg.RateLimitAuthMax,
		MailLimit:       cfg.RateLimitMailMax,
		RateLimitWindow: cfg.RateLimitWindow,
		SMTP: identity.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			User:     cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
			Timeout:  cfg.HTTPTimeout,
		},
	})
	if cfg.BootstrapAdminEmail != "" {
		if _, err := accounts.BootstrapAdmin(startupCtx, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword); err != nil {
			return fmt.Errorf("initialize admin account: %w", err)
		}
	}
	cancelStartup()

	gin.SetMode(gin.ReleaseMode)
	router := app.New(pool, accounts, app.Options{
		Agent:                 *agentCfg,
		PlaybackSigningSecret: cfg.PlaybackSigningSecret,
		PaymentWebhookSecret:  cfg.PaymentWebhookSecret,
		SwaggerEnabled:        cfg.SwaggerEnabled,
	})

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTPTimeout,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       time.Minute,
	}
	return serve(ctx, httpServer, cfg.ShutdownTimeout)
}

func serve(ctx context.Context, httpServer *http.Server, shutdownTimeout time.Duration) error {
	var listenerConfig net.ListenConfig
	listener, err := listenerConfig.Listen(ctx, "tcp", httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", httpServer.Addr, err)
	}
	serving, stop := context.WithCancel(ctx)
	shutdown := make(chan error, 1)
	go func() {
		<-serving.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			shutdown <- errors.Join(fmt.Errorf("shut down HTTP server: %w", err), httpServer.Close())
			return
		}
		shutdown <- nil
	}()

	slog.Info("HTTP server listening", "address", listener.Addr())
	err = httpServer.Serve(listener)
	stop()
	shutdownErr := <-shutdown
	if !errors.Is(err, http.ErrServerClosed) {
		return errors.Join(fmt.Errorf("serve HTTP: %w", err), shutdownErr)
	}
	return shutdownErr
}

// Command server starts the identity HTTP service.
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

	identity "github.com/StephenQiu30/ai2workspace/backend/services/identity"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/access"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/accounts"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/server"
	"github.com/StephenQiu30/ai2workspace/backend/services/identity/internal/sessions"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// @title identity API
// @version 0.1.0
// @description identity service API, generated from Go source.
// @BasePath /
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("service stopped", "service", "identity", "error", err)
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
		accStore  accounts.Store
		sessStore sessions.Store
		invStore  access.Store
		pool      *pgxpool.Pool
	)

	if cfg.DatabaseURL != "" {
		initCtx, initCancel := context.WithTimeout(ctx, 15*time.Second)
		defer initCancel()

		pool, err = pgxpool.New(initCtx, cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("connect postgres: %w", err)
		}
		defer pool.Close()

		if err := pool.Ping(initCtx); err != nil {
			return fmt.Errorf("ping postgres: %w", err)
		}
		if _, err := pool.Exec(initCtx, identity.SchemaSQL); err != nil {
			return fmt.Errorf("apply identity schema: %w", err)
		}

		accStore = accounts.NewPostgresStore(pool)
		sessStore = sessions.NewPostgresStore(pool)
		invStore = access.NewPostgresStore(pool)
		slog.Info("storage initialized", "service", "identity", "backend", "postgres")
	} else {
		accStore = accounts.NewMemoryStore()
		sessStore = sessions.NewMemoryStore()
		invStore = access.NewMemoryStore()
		slog.Info("storage initialized", "service", "identity", "backend", "memory")
	}

	if cfg.BootstrapAdminEmail != "" && cfg.BootstrapAdminPassword != "" {
		bootCtx, bootCancel := context.WithTimeout(ctx, 10*time.Second)
		defer bootCancel()
		adminAcc, err := accStore.EnsureBootstrapAdmin(bootCtx, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword)
		if err != nil {
			return fmt.Errorf("bootstrap admin account: %w", err)
		}
		slog.Info("bootstrap admin ensured", "service", "identity", "userId", adminAcc.ID, "email", adminAcc.Email)
	}

	var mailer accounts.Mailer
	if cfg.SMTPHost != "" {
		mailer = accounts.NewSMTPMailer(accounts.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			User:     cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
			Timeout:  cfg.HTTPTimeout,
		})
	}

	authLimiter := accounts.NewRateLimiter(cfg.RateLimitAuthMax, cfg.RateLimitWindow)
	mailLimiter := accounts.NewRateLimiter(cfg.RateLimitMailMax, cfg.RateLimitWindow)

	gin.SetMode(gin.ReleaseMode)
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}

	router := server.NewWithOptions(accStore, sessStore, invStore, server.Options{
		SwaggerEnabled:  cfg.SwaggerEnabled,
		ServiceToken:    cfg.ServiceToken,
		CookieSecure:    cfg.CookieSecure,
		DevExposeTokens: cfg.DevExposeTokens,
		Mailer:          mailer,
		AuthLimiter:     authLimiter,
		MailLimiter:     mailLimiter,
	})

	httpServer := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTPTimeout,
		ReadTimeout:       cfg.HTTPTimeout,
		WriteTimeout:      cfg.HTTPTimeout,
		IdleTimeout:       cfg.HTTPTimeout,
	}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			_ = httpServer.Close()
		}
	}()
	slog.Info("service started", "service", "identity", "address", listener.Addr().String())
	err = httpServer.Serve(listener)
	stop()
	<-stopped
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

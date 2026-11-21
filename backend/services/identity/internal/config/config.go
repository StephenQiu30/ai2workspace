// Package config reads and validates ai2workspace service configuration.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains only process configuration, never business state.
type Config struct {
	HTTPAddr               string
	HTTPTimeout            time.Duration
	ShutdownTimeout        time.Duration
	SwaggerEnabled         bool
	DatabaseURL            string
	ServiceToken           string
	CookieSecure           bool
	DevExposeTokens        bool
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
	SMTPHost               string
	SMTPPort               string
	SMTPUser               string
	SMTPPassword           string
	SMTPFrom               string
	RateLimitAuthMax       int
	RateLimitMailMax       int
	RateLimitWindow        time.Duration
}

// Load reads environment variables and rejects invalid startup configuration.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:               value("HTTP_ADDR", ":3123"),
		DatabaseURL:            strings.TrimSpace(os.Getenv("DATABASE_URL")),
		ServiceToken:           value("SERVICE_TOKEN", "identity-service-secret"),
		BootstrapAdminEmail:    strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL")),
		BootstrapAdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		SMTPHost:               strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:               value("SMTP_PORT", "587"),
		SMTPUser:               strings.TrimSpace(os.Getenv("SMTP_USER")),
		SMTPPassword:           os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:               value("SMTP_FROM", "no-reply@ai2workspace.local"),
	}
	_, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR must be host:port: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("HTTP_ADDR requires a port between 1 and 65535")
	}
	if cfg.HTTPTimeout, err = duration("HTTP_TIMEOUT", "5s"); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", "10s"); err != nil {
		return Config{}, err
	}
	if cfg.SwaggerEnabled, err = parseBool("SWAGGER_ENABLED", "false"); err != nil {
		return Config{}, err
	}
	if cfg.CookieSecure, err = parseBool("COOKIE_SECURE", "false"); err != nil {
		return Config{}, err
	}
	if cfg.DevExposeTokens, err = parseBool("DEV_EXPOSE_TOKENS", "false"); err != nil {
		return Config{}, err
	}
	if (cfg.BootstrapAdminEmail == "") != (cfg.BootstrapAdminPassword == "") {
		return Config{}, fmt.Errorf("BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD must be configured together")
	}
	if cfg.BootstrapAdminPassword != "" && len(cfg.BootstrapAdminPassword) < 8 {
		return Config{}, fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD must be at least 8 characters")
	}
	if cfg.RateLimitAuthMax, err = nonNegativeInt("RATE_LIMIT_AUTH_MAX", "20"); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitMailMax, err = nonNegativeInt("RATE_LIMIT_MAIL_MAX", "10"); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitWindow, err = duration("RATE_LIMIT_WINDOW", "1m"); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBool(key, fallback string) (bool, error) {
	v, err := strconv.ParseBool(value(key, fallback))
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return v, nil
}

func nonNegativeInt(key, fallback string) (int, error) {
	v, err := strconv.Atoi(value(key, fallback))
	if err != nil || v < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return v, nil
}

func duration(key, fallback string) (time.Duration, error) {
	d, err := time.ParseDuration(value(key, fallback))
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return d, nil
}

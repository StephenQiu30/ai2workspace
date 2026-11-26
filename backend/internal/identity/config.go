package identity

import (
	"time"

	accountshttp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/http/accounts"
	smtp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/adapter/smtp/accounts"
	accountsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/accounts"
)

// Config configures identity's cookies, mail delivery, and rate limits.
type Config struct {
	CookieSecure    bool
	DevExposeTokens bool
	SMTP            SMTPConfig
	AuthLimit       int
	MailLimit       int
	RateLimitWindow time.Duration
}

// SMTPConfig configures delivery of identity emails.
type SMTPConfig = smtp.SMTPConfig

// Options supplies the collaborators used by identity handlers.
type Options struct {
	CookieSecure    bool
	DevExposeTokens bool
	Mailer          accountsapp.Mailer
	AuthLimiter     *accountshttp.RateLimiter
	MailLimiter     *accountshttp.RateLimiter
}

func (cfg Config) options() Options {
	opts := Options{
		CookieSecure:    cfg.CookieSecure,
		DevExposeTokens: cfg.DevExposeTokens,
		AuthLimiter:     accountshttp.NewRateLimiter(cfg.AuthLimit, cfg.RateLimitWindow),
		MailLimiter:     accountshttp.NewRateLimiter(cfg.MailLimit, cfg.RateLimitWindow),
	}
	if cfg.SMTP.Host != "" {
		opts.Mailer = smtp.NewSMTPMailer(cfg.SMTP)
	}
	return opts
}

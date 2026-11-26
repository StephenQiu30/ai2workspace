package accounts

import (
	"context"
)

// MailMessage represents an outbound email payload.
type MailMessage struct {
	To      string
	Subject string
	Body    string
}

// Mailer delivers outbound transactional emails.
type Mailer interface {
	Send(ctx context.Context, msg MailMessage) error
}

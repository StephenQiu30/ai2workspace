package accounts

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// StatusActive indicates an operational account.
	StatusActive = "active"
	// StatusSuspended indicates an administratively suspended account.
	StatusSuspended = "suspended"
)

var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrAccountSuspended     = errors.New("account is suspended")
	ErrDuplicateEmail       = errors.New("email already registered")
	ErrInvalidPassword      = errors.New("invalid password")
	ErrInvalidStatus        = errors.New("invalid account status")
	ErrInvalidNickname      = errors.New("invalid nickname")
	ErrTokenNotFound        = errors.New("token not found")
	ErrTokenExpired         = errors.New("token expired")
	ErrTokenConsumed        = errors.New("token already consumed")
	ErrTokenPurposeMismatch = errors.New("token purpose mismatch")
	ErrVersionConflict      = errors.New("version conflict")
)

// TokenPurpose scopes a one-time token to a single workflow.
type TokenPurpose string

const (
	PurposeEmailVerification TokenPurpose = "email_verification"
	PurposePasswordReset     TokenPurpose = "password_reset"
	PurposeInstructorInvite  TokenPurpose = "instructor_invitation"
)

// Account represents a user identity record.
type Account struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Nickname      string    `json:"nickname"`
	PasswordHash  string    `json:"-"`
	EmailVerified bool      `json:"emailVerified"`
	Status        string    `json:"status"` // "active", "suspended"
	Roles         []string  `json:"roles"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ListAccountsFilter defines optional filters and pagination for admin account listing.
type ListAccountsFilter struct {
	Status   string
	Role     string
	Query    string
	Page     int
	PageSize int
}

// StoredToken tracks one-time tokens for email verification or password reset.
type StoredToken struct {
	TokenHash string
	Purpose   TokenPurpose
	Target    string // email or userId
	ExpiresAt time.Time
	Consumed  bool
	CreatedAt time.Time
}

// MailTask records an outbound email notification.
type MailTask struct {
	ID        string
	Recipient string
	Subject   string
	Purpose   TokenPurpose
	Token     string
	Status    string
	CreatedAt time.Time
}

// NormalizeEmail converts email to lowercase and trims whitespace.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// DefaultNickname returns the trimmed nickname or falls back to the email prefix before "@".
func DefaultNickname(email, nickname string) string {
	trimmed := strings.TrimSpace(nickname)
	if trimmed != "" {
		if utf8.RuneCountInString(trimmed) > 64 {
			return string([]rune(trimmed)[:64])
		}
		return trimmed
	}
	norm := NormalizeEmail(email)
	if prefix, _, ok := strings.Cut(norm, "@"); ok && prefix != "" {
		if utf8.RuneCountInString(prefix) > 64 {
			return string([]rune(prefix)[:64])
		}
		return prefix
	}
	return "user"
}

// ValidateNickname checks that a user-provided nickname is non-empty and within length bounds.
func ValidateNickname(nickname string) (string, error) {
	trimmed := strings.TrimSpace(nickname)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > 64 {
		return "", ErrInvalidNickname
	}
	return trimmed, nil
}

// ValidateStatus checks that an account status is an allowed value.
func ValidateStatus(status string) error {
	if status != StatusActive && status != StatusSuspended {
		return ErrInvalidStatus
	}
	return nil
}

const SessionCookieName = "ai2_session"

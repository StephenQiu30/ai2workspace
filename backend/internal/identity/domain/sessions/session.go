package sessions

import (
	"errors"
	"time"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
	ErrSessionRevoked  = errors.New("session revoked")
)

// Session represents an active or revoked user login session.
type Session struct {
	ID        string    `json:"id"` // token hash
	UserID    string    `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	Revoked   bool      `json:"revoked"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
}

package sessions

import (
	"context"
	"errors"

	accountsapp "github.com/StephenQiu30/ai2workspace/backend/internal/identity/application/accounts"
	accountsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/accounts"
	sessionsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/identity/domain/sessions"
)

// Resolve loads the current account for an opaque session token.
func Resolve(ctx context.Context, store Store, users accountsapp.Store, token string) (*accountsdomain.Account, error) {
	if token == "" {
		return nil, sessionsdomain.ErrInvalidSession
	}
	session, err := store.GetSession(ctx, token)
	if err != nil {
		if errors.Is(err, sessionsdomain.ErrSessionNotFound) || errors.Is(err, sessionsdomain.ErrSessionExpired) || errors.Is(err, sessionsdomain.ErrSessionRevoked) {
			return nil, sessionsdomain.ErrInvalidSession
		}
		return nil, err
	}
	account, err := users.GetAccountByID(ctx, session.UserID)
	if errors.Is(err, accountsdomain.ErrAccountNotFound) {
		return nil, sessionsdomain.ErrInvalidSession
	}
	if err != nil {
		return nil, err
	}
	if account.Status != accountsdomain.StatusActive {
		return nil, sessionsdomain.ErrInactiveAccount
	}
	return account, nil
}

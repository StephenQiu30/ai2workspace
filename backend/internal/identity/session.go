// Package identity handles accounts and workspace access in the modular backend.
package identity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const sessionCookieName = "ai2_session"

// SessionClient checks the current session with the legacy identity service during migration.
type SessionClient struct {
	endpoint string
	client   http.Client
}

// NewSessionClient accepts the origin of the existing identity service.
func NewSessionClient(origin string) (*SessionClient, error) {
	if origin == "" {
		origin = "http://127.0.0.1:3123"
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") ||
		u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, errors.New("STUDIO_IDENTITY_ORIGIN must be an HTTP(S) origin")
	}
	u.Path = "/api/identity/me"
	return &SessionClient{
		endpoint: u.String(),
		client: http.Client{
			Timeout: 3 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (s *SessionClient) userID(ctx context.Context, cookie *http.Cookie) (string, int) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint, nil)
	if err != nil {
		return "", http.StatusServiceUnavailable
	}
	req.AddCookie(cookie)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", http.StatusServiceUnavailable
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", resp.StatusCode
	case http.StatusOK:
		var actor struct {
			UserID        string `json:"userId"`
			AccountStatus string `json:"accountStatus"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&actor); err != nil || strings.TrimSpace(actor.UserID) == "" {
			return "", http.StatusServiceUnavailable
		}
		if actor.AccountStatus != "active" {
			return "", http.StatusForbidden
		}
		return actor.UserID, http.StatusOK
	default:
		return "", http.StatusServiceUnavailable
	}
}

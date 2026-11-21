package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/courses"
)

const defaultHTTPClientTimeout = 5 * time.Second

// NewIdentityAuthenticator builds a fail-closed authenticator that resolves the ai2_session
// cookie against identity's internal session resolution endpoint.
// Plaintext identity headers such as X-User-ID or X-Actor-Id are never trusted.
func NewIdentityAuthenticator(identityServiceURL, serviceToken string, httpClient *http.Client) auth.Authenticator {
	baseURL := strings.TrimRight(strings.TrimSpace(identityServiceURL), "/")
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPClientTimeout}
	}

	return func(ctx context.Context, r *http.Request) (*auth.Principal, error) {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			return nil, nil
		}
		if baseURL == "" {
			return nil, auth.ErrUnauthenticated
		}

		payload, err := json.Marshal(struct {
			OpaqueSession string `json:"opaqueSession"`
		}{OpaqueSession: cookie.Value})
		if err != nil {
			return nil, auth.ErrAuthUnavailable
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/internal/identity/sessions/resolve", bytes.NewReader(payload))
		if err != nil {
			return nil, auth.ErrAuthUnavailable
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Service-Token", serviceToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, auth.ErrAuthUnavailable
		}
		defer func() { _ = resp.Body.Close() }()

		switch resp.StatusCode {
		case http.StatusOK:
			// Continue to decode below.
		case http.StatusUnauthorized:
			return nil, auth.ErrUnauthenticated
		case http.StatusForbidden:
			return nil, auth.ErrAccountSuspended
		default:
			return nil, auth.ErrAuthUnavailable
		}

		var resolved struct {
			UserID         string   `json:"userId"`
			Email          string   `json:"email"`
			Nickname       string   `json:"nickname"`
			EmailVerified  bool     `json:"emailVerified"`
			Roles          []string `json:"roles"`
			AccountStatus  string   `json:"accountStatus"`
			SessionVersion int      `json:"sessionVersion"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&resolved); err != nil {
			return nil, auth.ErrAuthUnavailable
		}
		if resolved.UserID == "" {
			return nil, auth.ErrUnauthenticated
		}
		if strings.EqualFold(strings.TrimSpace(resolved.AccountStatus), "suspended") {
			return nil, auth.ErrAccountSuspended
		}

		return &auth.Principal{
			UserID:         resolved.UserID,
			Email:          resolved.Email,
			Nickname:       resolved.Nickname,
			EmailVerified:  resolved.EmailVerified,
			Roles:          resolved.Roles,
			Status:         resolved.AccountStatus,
			SessionVersion: resolved.SessionVersion,
		}, nil
	}
}

// CommerceClient queries the commerce microservice over internal HTTP APIs.
type CommerceClient struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

// NewCommerceClient constructs an internal HTTP client for commerce entitlements.
func NewCommerceClient(commerceServiceURL, serviceToken string, httpClient *http.Client) *CommerceClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPClientTimeout}
	}
	return &CommerceClient{
		baseURL:      strings.TrimRight(strings.TrimSpace(commerceServiceURL), "/"),
		serviceToken: serviceToken,
		httpClient:   httpClient,
	}
}

// CheckEntitlement checks whether userID holds an active purchase grant for courseID.
func (c *CommerceClient) CheckEntitlement(ctx context.Context, userID, courseID, purpose string) (bool, string, error) {
	if c == nil || c.baseURL == "" {
		return false, "entitlement_required", nil
	}
	if purpose == "" {
		purpose = "playback"
	}

	payload, err := json.Marshal(struct {
		UserID   string `json:"userId"`
		CourseID string `json:"courseId"`
		Purpose  string `json:"purpose"`
	}{
		UserID:   userID,
		CourseID: courseID,
		Purpose:  purpose,
	})
	if err != nil {
		return false, "entitlement_service_unavailable", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/commerce/entitlements/check", bytes.NewReader(payload))
	if err != nil {
		return false, "entitlement_service_unavailable", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", c.serviceToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, "entitlement_service_unavailable", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, "entitlement_service_unavailable", fmt.Errorf("commerce entitlement check status %d", resp.StatusCode)
	}

	var result struct {
		Allowed      bool   `json:"allowed"`
		GrantVersion int    `json:"grantVersion"`
		Reason       string `json:"reason"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return false, "entitlement_service_unavailable", err
	}
	return result.Allowed, result.Reason, nil
}

// ListUserEntitlements fetches all entitlements belonging to userID from commerce.
func (c *CommerceClient) ListUserEntitlements(ctx context.Context, userID string) ([]courses.UserEntitlement, error) {
	if c == nil || c.baseURL == "" {
		return nil, nil
	}

	endpoint := fmt.Sprintf("%s/internal/commerce/users/%s/entitlements", c.baseURL, url.PathEscape(userID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Service-Token", c.serviceToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("commerce list entitlements status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if trimmed[0] == '[' {
		var list []courses.UserEntitlement
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, err
		}
		return list, nil
	}

	var wrapped struct {
		Entitlements []courses.UserEntitlement `json:"entitlements"`
		Items        []courses.UserEntitlement `json:"items"`
	}
	if err := json.Unmarshal(trimmed, &wrapped); err != nil {
		return nil, err
	}
	if len(wrapped.Entitlements) > 0 {
		return wrapped.Entitlements, nil
	}
	return wrapped.Items, nil
}

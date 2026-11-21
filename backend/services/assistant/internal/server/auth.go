package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const sessionCookieName = "ai2_session"

var (
	ErrUnauthorized     = errors.New("valid authenticated session required")
	ErrAccountSuspended = errors.New("account is suspended")
	ErrForbidden        = errors.New("insufficient permissions")
	ErrEmailNotVerified = errors.New("verified email is required to use assistant chat")
)

// Actor represents a verified caller resolved from an opaque session via the identity service.
type Actor struct {
	UserID        string   `json:"userId"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"emailVerified"`
	Roles         []string `json:"roles"`
	AccountStatus string   `json:"accountStatus"`
}

// HasRole checks whether the actor holds the given role.
func (a *Actor) HasRole(role string) bool {
	if a == nil {
		return false
	}
	for _, r := range a.Roles {
		if strings.EqualFold(strings.TrimSpace(r), role) {
			return true
		}
	}
	return false
}

// IsAdmin returns true when the actor holds the admin role.
func (a *Actor) IsAdmin() bool {
	return a.HasRole("admin")
}

// IsInstructor returns true when the actor holds the instructor role.
func (a *Actor) IsInstructor() bool {
	return a.HasRole("instructor")
}

// CanManageKnowledge returns true when the actor is an admin or instructor.
func (a *Actor) CanManageKnowledge() bool {
	return a.IsAdmin() || a.IsInstructor()
}

// SessionResolver resolves an opaque ai2_session cookie into a verified Actor.
type SessionResolver func(ctx context.Context, opaqueSession string) (*Actor, error)

type identityResolveRequest struct {
	OpaqueSession string `json:"opaqueSession"`
}

type identityResolveResponse struct {
	UserID         string   `json:"userId"`
	Email          string   `json:"email"`
	EmailVerified  bool     `json:"emailVerified"`
	Roles          []string `json:"roles"`
	AccountStatus  string   `json:"accountStatus"`
	SessionVersion int      `json:"sessionVersion"`
}

func newHTTPSessionResolver(identityServiceURL, serviceToken string, httpClient *http.Client) SessionResolver {
	baseURL := strings.TrimRight(strings.TrimSpace(identityServiceURL), "/")
	if baseURL == "" {
		return func(ctx context.Context, opaqueSession string) (*Actor, error) {
			return nil, ErrUnauthorized
		}
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	endpoint := baseURL + "/internal/identity/sessions/resolve"

	return func(ctx context.Context, opaqueSession string) (*Actor, error) {
		body, err := json.Marshal(identityResolveRequest{OpaqueSession: opaqueSession})
		if err != nil {
			return nil, ErrUnauthorized
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, ErrUnauthorized
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Service-Token", serviceToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("identity service unreachable: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusForbidden {
			return nil, ErrAccountSuspended
		}
		if resp.StatusCode != http.StatusOK {
			return nil, ErrUnauthorized
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, ErrUnauthorized
		}
		var parsed identityResolveResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil || strings.TrimSpace(parsed.UserID) == "" {
			return nil, ErrUnauthorized
		}
		if strings.EqualFold(parsed.AccountStatus, "suspended") {
			return nil, ErrAccountSuspended
		}
		return &Actor{
			UserID:        parsed.UserID,
			Email:         parsed.Email,
			EmailVerified: parsed.EmailVerified,
			Roles:         parsed.Roles,
			AccountStatus: parsed.AccountStatus,
		}, nil
	}
}

type learningCourseAccessRequest struct {
	UserID   string `json:"userId"`
	CourseID string `json:"courseId"`
	Purpose  string `json:"purpose"`
}

type learningCourseAccessResponse struct {
	Allowed        bool   `json:"allowed"`
	Reason         string `json:"reason"`
	AccessVersion  int    `json:"accessVersion"`
	ContentVersion int    `json:"contentVersion"`
}

func newHTTPCourseAccessChecker(learningServiceURL, serviceToken string, httpClient *http.Client) func(ctx context.Context, userID, courseID string) (bool, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(learningServiceURL), "/")
	if baseURL == "" {
		return nil // Causes knowledge.NewService to fail closed
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	endpoint := baseURL + "/internal/learning/course-access/check"

	return func(ctx context.Context, userID, courseID string) (bool, error) {
		payload := learningCourseAccessRequest{
			UserID:   userID,
			CourseID: courseID,
			Purpose:  "knowledge",
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return false, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Service-Token", serviceToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			return false, fmt.Errorf("learning service unreachable: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("learning course-access check returned HTTP %d", resp.StatusCode)
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return false, err
		}
		var parsed learningCourseAccessResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return false, fmt.Errorf("invalid learning course-access response: %w", err)
		}
		return parsed.Allowed, nil
	}
}

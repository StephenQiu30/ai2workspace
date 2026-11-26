package learning_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/StephenQiu30/ai2workspace/backend/internal/learning"
	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/adapter/http/auth"
	coursesapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/courses"
	mediaapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/media"
	progressapp "github.com/StephenQiu30/ai2workspace/backend/internal/learning/application/progress"
	"github.com/gin-gonic/gin"
)

const (
	testSigningSecret = "test-playback-secret"

	sessAdmin     = "tok_admin"
	sessInst1     = "tok_inst_1"
	sessInst2     = "tok_inst_2"
	sessStudent1  = "tok_student_1"
	sessStudent2  = "tok_student_2"
	sessSuspended = "tok_suspended"
)

type mockEntitlements struct {
	mu     sync.RWMutex
	grants map[string]map[string]bool // userID -> courseID -> entitlement
	err    error
}

func newMockEntitlements() *mockEntitlements {
	return &mockEntitlements{
		grants: make(map[string]map[string]bool),
	}
}

func (m *mockEntitlements) Grant(userID, courseID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.grants[userID] == nil {
		m.grants[userID] = make(map[string]bool)
	}
	m.grants[userID][courseID] = true
}

func (m *mockEntitlements) Revoke(userID, courseID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.grants[userID] != nil {
		delete(m.grants[userID], courseID)
	}
}

func (m *mockEntitlements) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *mockEntitlements) CheckEntitlement(ctx context.Context, userID, courseID, purpose string) (bool, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return false, "entitlement_service_unavailable", m.err
	}
	if coursesMap, ok := m.grants[userID]; ok {
		if coursesMap[courseID] {
			return true, "entitlement_active", nil
		}
	}
	return false, "entitlement_not_found", nil
}

func (m *mockEntitlements) ListActiveCourseIDs(ctx context.Context, userID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return nil, m.err
	}
	var out []string
	for courseID := range m.grants[userID] {
		out = append(out, courseID)
	}
	return out, nil
}

type testEnv struct {
	router        *gin.Engine
	coursesStore  *coursesapp.MemoryStore
	mediaStore    *mediaapp.MemoryStore
	progressStore *progressapp.MemoryStore
	entitlements  *mockEntitlements
}

func newTestEnv() *testEnv {
	gin.SetMode(gin.TestMode)
	principals := map[string]*authhttp.Principal{
		sessAdmin: {
			UserID:        "admin_1",
			Email:         "admin@example.com",
			Nickname:      "系统管理员",
			EmailVerified: true,
			Roles:         []string{"admin"},
			Status:        "active",
		},
		sessInst1: {
			UserID:        "instructor_1",
			Email:         "inst1@example.com",
			Nickname:      "林讲师",
			EmailVerified: true,
			Roles:         []string{"instructor"},
			Status:        "active",
		},
		sessInst2: {
			UserID:        "instructor_2",
			Email:         "inst2@example.com",
			Nickname:      "周讲师",
			EmailVerified: true,
			Roles:         []string{"instructor"},
			Status:        "active",
		},
		sessStudent1: {
			UserID:        "student_1",
			Email:         "stu1@example.com",
			Nickname:      "学员甲",
			EmailVerified: true,
			Roles:         []string{"student"},
			Status:        "active",
		},
		sessStudent2: {
			UserID:        "student_2",
			Email:         "stu2@example.com",
			Nickname:      "学员乙",
			EmailVerified: true,
			Roles:         []string{"student"},
			Status:        "active",
		},
		sessSuspended: {
			UserID:        "suspended_1",
			Email:         "sus@example.com",
			Nickname:      "停用账号",
			EmailVerified: true,
			Roles:         []string{"student"},
			Status:        "suspended",
		},
	}

	coursesStore := coursesapp.NewMemoryStore()
	mediaStore := mediaapp.NewMemoryStore()
	progressStore := progressapp.NewMemoryStore()
	entitlements := newMockEntitlements()

	router := gin.New()
	learning.NewWithDeps(learning.Deps{

		PlaybackSigningSecret: testSigningSecret,
		CoursesStore:          coursesStore,
		MediaStore:            mediaStore,
		ProgressStore:         progressStore,
		Entitlements:          entitlements,
		Authenticator: func(ctx context.Context, r *http.Request) (*authhttp.Principal, error) {
			cookie, err := r.Cookie(identity.SessionCookieName)
			if err != nil || cookie.Value == "" {
				return nil, nil
			}
			p, ok := principals[cookie.Value]
			if !ok {
				return nil, authhttp.ErrUnauthenticated
			}
			if p.Status == "suspended" {
				return nil, authhttp.ErrAccountSuspended
			}
			cp := *p
			return &cp, nil
		},
	}).RegisterRoutes(router)

	return &testEnv{
		router:        router,
		coursesStore:  coursesStore,
		mediaStore:    mediaStore,
		progressStore: progressStore,
		entitlements:  entitlements,
	}
}

func doRequest(t *testing.T, handler http.Handler, method, path string, body any, sessionToken string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: sessionToken})
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

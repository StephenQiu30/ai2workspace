package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/courses"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/media"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/progress"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/server"
	"github.com/gin-gonic/gin"
)

const (
	testServiceToken  = "test-service-token"
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
	grants map[string]map[string]courses.UserEntitlement // userID -> courseID -> entitlement
	err    error
}

func newMockEntitlements() *mockEntitlements {
	return &mockEntitlements{
		grants: make(map[string]map[string]courses.UserEntitlement),
	}
}

func (m *mockEntitlements) Grant(userID, courseID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.grants[userID] == nil {
		m.grants[userID] = make(map[string]courses.UserEntitlement)
	}
	m.grants[userID][courseID] = courses.UserEntitlement{
		ID:          "ent_" + userID + "_" + courseID,
		UserID:      userID,
		CourseID:    courseID,
		OrderID:     "ord_" + courseID,
		Status:      "active",
		ActivatedAt: time.Now().UTC(),
	}
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
		if ent, ok := coursesMap[courseID]; ok && ent.Status == "active" {
			return true, "entitlement_active", nil
		}
	}
	return false, "entitlement_not_found", nil
}

func (m *mockEntitlements) ListUserEntitlements(ctx context.Context, userID string) ([]courses.UserEntitlement, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return nil, m.err
	}
	var out []courses.UserEntitlement
	for _, ent := range m.grants[userID] {
		out = append(out, ent)
	}
	return out, nil
}

type testEnv struct {
	router        *gin.Engine
	coursesStore  *courses.MemoryStore
	mediaStore    *media.MemoryStore
	progressStore *progress.MemoryStore
	entitlements  *mockEntitlements
}

func newTestEnv() *testEnv {
	gin.SetMode(gin.TestMode)
	principals := map[string]*auth.Principal{
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

	coursesStore := courses.NewMemoryStore()
	mediaStore := media.NewMemoryStore()
	progressStore := progress.NewMemoryStore()
	entitlements := newMockEntitlements()

	router := server.NewWithDeps(server.Deps{
		SwaggerEnabled:        false,
		ServiceToken:          testServiceToken,
		PlaybackSigningSecret: testSigningSecret,
		CoursesStore:          coursesStore,
		MediaStore:            mediaStore,
		ProgressStore:         progressStore,
		Entitlements:          entitlements,
		Authenticator: func(ctx context.Context, r *http.Request) (*auth.Principal, error) {
			cookie, err := r.Cookie(auth.SessionCookieName)
			if err != nil || cookie.Value == "" {
				return nil, nil
			}
			p, ok := principals[cookie.Value]
			if !ok {
				return nil, auth.ErrUnauthenticated
			}
			if p.Status == "suspended" {
				return nil, auth.ErrAccountSuspended
			}
			cp := *p
			return &cp, nil
		},
	})

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
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionToken})
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/auth"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/payments"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/refunds"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/server"
)

func TestHealth(t *testing.T) {
	router := server.New(false)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/commerce/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("health status = %d", response.Code)
	}
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["service"] != "commerce" || result["status"] != "ok" {
		t.Fatalf("unexpected health response: %v", result)
	}
}

func TestSwaggerMatchesAPIRoutes(t *testing.T) {
	router := server.New(true)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/commerce/doc.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("swagger status = %d: %s", response.Code, response.Body)
	}
	var document struct {
		Swagger string `json:"swagger"`
		Paths   map[string]map[string]struct {
			ID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.Swagger != "2.0" {
		t.Fatalf("unexpected swagger version: %+v", document)
	}
	paramRe := regexp.MustCompile(`:([a-zA-Z0-9_]+)`)
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/api/commerce") || strings.HasPrefix(route.Path, "/internal/commerce") {
			swaggerPath := paramRe.ReplaceAllString(route.Path, "{$1}")
			if _, ok := document.Paths[swaggerPath][strings.ToLower(route.Method)]; !ok {
				t.Errorf("route missing from Swagger: %s %s (checked %s)", route.Method, route.Path, swaggerPath)
			}
		}
	}
	ids := make(map[string]bool)
	for path, methods := range document.Paths {
		for method, operation := range methods {
			if operation.ID == "" || ids[operation.ID] {
				t.Errorf("missing or duplicate operationId for %s %s (%s)", method, path, operation.ID)
			}
			ids[operation.ID] = true
		}
	}
}

func TestFailClosedAuthNeverTrustsPlaintextUserHeader(t *testing.T) {
	router := server.New(false)

	// Attempting to access protected endpoints with only X-User-ID header must fail with 401.
	req := httptest.NewRequest(http.MethodGet, "/api/commerce/orders", nil)
	req.Header.Set("X-User-ID", "spoofed_buyer")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when identity resolver is not configured, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/commerce/me/entitlements", nil)
	req.Header.Set("X-User-ID", "spoofed_buyer")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on /me/entitlements without session, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/commerce/refunds", nil)
	req.Header.Set("X-User-ID", "admin_mock")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on /refunds without session, got %d", w.Code)
	}
}

func TestCrossServiceHTTPClientsEndToEnd(t *testing.T) {
	const serviceToken = "test-inter-service-token"
	const webhookSecret = "test-webhook-secret"

	// 1. Mock Identity Service
	identitySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/identity/sessions/resolve" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get(auth.ServiceTokenHeader) != serviceToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body struct {
			OpaqueSession string `json:"opaqueSession"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch body.OpaqueSession {
		case "sess_admin":
			_ = json.NewEncoder(w).Encode(auth.Principal{
				UserID: "usr_admin", Email: "admin@example.com", EmailVerified: true,
				Roles: []string{"admin"}, AccountStatus: "active", SessionVersion: 1,
			})
		case "sess_buyer":
			_ = json.NewEncoder(w).Encode(auth.Principal{
				UserID: "usr_buyer", Email: "buyer@example.com", EmailVerified: true,
				Roles: []string{"student"}, AccountStatus: "active", SessionVersion: 1,
			})
		case "sess_unverified":
			_ = json.NewEncoder(w).Encode(auth.Principal{
				UserID: "usr_unverified", Email: "unverified@example.com", EmailVerified: false,
				Roles: []string{"student"}, AccountStatus: "active", SessionVersion: 1,
			})
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer identitySrv.Close()

	// 2. Mock Learning Service
	var deliveryFail bool
	var learningMu sync.Mutex
	learningSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(auth.ServiceTokenHeader) != serviceToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/internal/learning/courses/crs_on_sale/sale-snapshot":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"courseId":         "crs_on_sale",
				"publishedVersion": 2,
				"title":            "Go 微服务实战",
				"saleState":        "on_sale",
				"deliverable":      true,
				"isFree":           false,
			})
		case r.URL.Path == "/internal/learning/courses/crs_closed/sale-snapshot":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"courseId":         "crs_closed",
				"publishedVersion": 1,
				"title":            "已停售课程",
				"saleState":        "closed",
				"deliverable":      true,
				"isFree":           false,
			})
		case r.URL.Path == "/internal/learning/courses/crs_free/sale-snapshot":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"courseId":         "crs_free",
				"publishedVersion": 1,
				"title":            "免费入门课",
				"saleState":        "on_sale",
				"deliverable":      true,
				"isFree":           true,
			})
		case r.URL.Path == "/internal/learning/courses/crs_on_sale/delivery-evidence":
			learningMu.Lock()
			fail := deliveryFail
			learningMu.Unlock()
			if fail {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			now := time.Now().UTC()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"courseId":           "crs_on_sale",
				"firstDeliverableAt": now,
				"evidenceVersion":    1,
				"contentVersion":     2,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer learningSrv.Close()

	// 3. Mock Community Service
	var notifMu sync.Mutex
	var notifications []map[string]any
	var failCommunity bool
	communitySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/community/notifications" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get(auth.ServiceTokenHeader) != serviceToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		notifMu.Lock()
		shouldFail := failCommunity
		notifMu.Unlock()
		if shouldFail {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		notifMu.Lock()
		notifications = append(notifications, body)
		notifMu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer communitySrv.Close()

	cfg := &config.Config{
		SwaggerEnabled:       false,
		ServiceToken:         serviceToken,
		IdentityServiceURL:   identitySrv.URL,
		LearningServiceURL:   learningSrv.URL,
		CommunityServiceURL:  communitySrv.URL,
		PaymentWebhookSecret: webhookSecret,
		HTTPTimeout:          10 * time.Second,
	}
	router, cleanup, err := server.NewFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	defer cleanup()

	// Set course prices as admin (including 0 for free course).
	for _, tc := range []struct {
		courseID string
		amount   int64
	}{
		{"crs_on_sale", 19900},
		{"crs_closed", 9900},
		{"crs_free", 0},
	} {
		amt := tc.amount
		body, _ := json.Marshal(orders.SetPriceRequest{AmountMinor: &amt, Currency: "CNY"})
		req := httptest.NewRequest(http.MethodPost, "/api/commerce/courses/"+tc.courseID+"/price", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess_admin"})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("set price %s failed: %d %s", tc.courseID, w.Code, w.Body.String())
		}
	}

	// Unverified user cannot create order -> 403.
	ordPayload, _ := json.Marshal(orders.CreateOrderRequest{CourseID: "crs_on_sale", IdempotencyKey: "k_unv"})
	req := httptest.NewRequest(http.MethodPost, "/api/commerce/orders", bytes.NewReader(ordPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess_unverified"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unverified email, got %d: %s", w.Code, w.Body.String())
	}

	// Closed course cannot be ordered -> 409.
	closedPayload, _ := json.Marshal(orders.CreateOrderRequest{CourseID: "crs_closed", IdempotencyKey: "k_closed"})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/orders", bytes.NewReader(closedPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess_buyer"})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for closed course, got %d: %s", w.Code, w.Body.String())
	}

	// Free course cannot create paid order -> 400.
	freePayload, _ := json.Marshal(orders.CreateOrderRequest{CourseID: "crs_free", IdempotencyKey: "k_free"})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/orders", bytes.NewReader(freePayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess_buyer"})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for free course order, got %d: %s", w.Code, w.Body.String())
	}

	// Verified user creates order for on_sale course -> 201 with CourseTitle snapshot!
	ordPayload, _ = json.Marshal(orders.CreateOrderRequest{CourseID: "crs_on_sale", IdempotencyKey: "k_buy_1"})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/orders", bytes.NewReader(ordPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess_buyer"})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for valid order, got %d: %s", w.Code, w.Body.String())
	}
	var ordResp orders.OrderResponse
	_ = json.Unmarshal(w.Body.Bytes(), &ordResp)
	if ordResp.CourseTitle != "Go 微服务实战" || ordResp.Status != string(orders.StatusPendingPayment) {
		t.Fatalf("unexpected order response: %+v", ordResp)
	}

	// Simulate community outage during payment webhook: payment & entitlement MUST STILL SUCCEED!
	notifMu.Lock()
	failCommunity = true
	notifMu.Unlock()

	sig := payments.ComputeWebhookSignature(webhookSecret, ordResp.ID, "wechat", "tx_e2e_1", 19900, "CNY")
	whPayload, _ := json.Marshal(payments.WebhookConfirmationRequest{
		OrderID: ordResp.ID, Channel: "wechat", ChannelOrderID: "tx_e2e_1",
		AmountMinor: 19900, Currency: "CNY", Signature: sig,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(whPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even when community notification fails, got %d: %s", w.Code, w.Body.String())
	}

	// Restore community for refund notification check.
	notifMu.Lock()
	failCommunity = false
	notifMu.Unlock()

	// Delivery evidence outage -> refund apply MUST return 503 (never silently shorten window).
	learningMu.Lock()
	deliveryFail = true
	learningMu.Unlock()
	refPayload, _ := json.Marshal(refunds.ApplyRefundRequest{Reason: "申请退款"})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ordResp.ID+"/refunds", bytes.NewReader(refPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess_buyer"})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when delivery evidence fails, got %d: %s", w.Code, w.Body.String())
	}

	// Restore delivery evidence -> apply refund -> admin approve -> channel confirm success -> community notified!
	learningMu.Lock()
	deliveryFail = false
	learningMu.Unlock()

	req = httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ordResp.ID+"/refunds", bytes.NewReader(refPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess_buyer"})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for refund apply, got %d: %s", w.Code, w.Body.String())
	}
	var refResp refunds.RefundResponse
	_ = json.Unmarshal(w.Body.Bytes(), &refResp)

	revPayload, _ := json.Marshal(refunds.ReviewRefundRequest{Decision: "approved", ExpectedVersion: 1})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/review", bytes.NewReader(revPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess_admin"})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for refund review, got %d: %s", w.Code, w.Body.String())
	}

	confirmPayload, _ := json.Marshal(refunds.ChannelConfirmRefundRequest{Status: "success", ChannelRefundID: "ch_ref_e2e_1"})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/channel-confirm", bytes.NewReader(confirmPayload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess_admin"})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for channel confirm, got %d: %s", w.Code, w.Body.String())
	}

	notifMu.Lock()
	defer notifMu.Unlock()
	if len(notifications) != 1 || notifications[0]["type"] != "refund_succeeded" {
		t.Fatalf("expected 1 refund_succeeded notification in community, got %+v", notifications)
	}
}

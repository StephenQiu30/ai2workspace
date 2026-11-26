package commerce_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/internal/commerce"
	authhttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/auth"
	ordershttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/orders"
	paymentshttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/payments"
	entitlementsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/entitlements"
	ordersapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/orders"
	paymentsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/payments"
	refundsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/refunds"
	ordersdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/orders"
	paymentsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/payments"
	"github.com/StephenQiu30/ai2workspace/backend/internal/identity"
	"github.com/gin-gonic/gin"
)

const (
	testWebhookSecret = "test-hmac-secret"
)

type testCommerceEnv struct {
	orderStore  *ordersapp.MemoryStore
	payStore    *paymentsapp.MemoryStore
	entStore    *entitlementsapp.MemoryStore
	refundStore *refundsapp.MemoryStore
	router      http.Handler
	principals  map[string]*authhttp.Principal
}

func setupTestCommerce(
	saleVerifier ordersapp.SaleSnapshotVerifier,
	evidenceProvider refundsapp.DeliveryEvidenceProvider,
	querier paymentsapp.ChannelStatusQuerier,
	notifier paymentsapp.Notifier,
) *testCommerceEnv {
	principals := map[string]*authhttp.Principal{
		"tok_admin": {
			UserID: "admin_1", Email: "admin@test.local", EmailVerified: true,
			Roles: []string{"admin"}, AccountStatus: "active",
		},
		"tok_buyer_1": {
			UserID: "buyer_1", Email: "buyer1@test.local", EmailVerified: true,
			Roles: []string{"student"}, AccountStatus: "active",
		},
		"tok_buyer_2": {
			UserID: "buyer_2", Email: "buyer2@test.local", EmailVerified: true,
			Roles: []string{"student"}, AccountStatus: "active",
		},
		"tok_unverified": {
			UserID: "unverified_1", Email: "unv@test.local", EmailVerified: false,
			Roles: []string{"student"}, AccountStatus: "active",
		},
	}

	resolver := func(c *gin.Context) (*authhttp.Principal, error) {
		tok, err := c.Cookie(identity.SessionCookieName)
		if err != nil || tok == "" {
			return nil, authhttp.ErrUnauthenticated
		}
		p, ok := principals[tok]
		if !ok {
			return nil, authhttp.ErrUnauthenticated
		}
		return p, nil
	}

	entStore := entitlementsapp.NewMemoryStore()
	orderStore := ordersapp.NewMemoryStore(
		saleVerifier,
		func(ctx context.Context, userID, courseID string) (bool, error) {
			allowed, _, _, err := entStore.CheckEntitlement(ctx, userID, courseID, "order_check")
			return allowed, err
		},
	)
	payStore := paymentsapp.NewMemoryStore(
		orderStore,
		func(ctx context.Context, userID, courseID, orderID string) error {
			_, err := entStore.GrantEntitlement(ctx, userID, courseID, orderID)
			return err
		},
		paymentsapp.StoreOptions{
			WebhookSecret: testWebhookSecret,
			Notifier:      notifier,
			Querier:       querier,
		},
	)
	refundStore := refundsapp.NewMemoryStore(orderStore, entStore, evidenceProvider, refundsapp.Notifier(notifier))

	router := gin.New()
	commerce.NewWithDeps(commerce.Deps{

		WebhookSecret:    testWebhookSecret,
		AuthResolver:     resolver,
		OrderStore:       orderStore,
		PaymentStore:     payStore,
		EntitlementStore: entStore,
		RefundStore:      refundStore,
	}).RegisterRoutes(router)

	return &testCommerceEnv{
		orderStore:  orderStore,
		payStore:    payStore,
		entStore:    entStore,
		refundStore: refundStore,
		router:      router,
		principals:  principals,
	}
}

func withCookie(req *http.Request, session string) *http.Request {
	req.AddCookie(&http.Cookie{Name: identity.SessionCookieName, Value: session})
	return req
}

func TestPriceValidationAndAdminOnly(t *testing.T) {
	env := setupTestCommerce(nil, nil, nil, nil)

	// Non-admin cannot set course price -> 403.
	amt := int64(9900)
	body, _ := json.Marshal(ordershttp.SetPriceRequest{AmountMinor: &amt, Currency: "CNY"})
	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/courses/crs_1/price", bytes.NewReader(body)), "tok_buyer_1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for student setting price, got %d", w.Code)
	}

	// Admin can set 0 (free course).
	zero := int64(0)
	body, _ = json.Marshal(ordershttp.SetPriceRequest{AmountMinor: &zero, Currency: "CNY"})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/courses/crs_free/price", bytes.NewReader(body)), "tok_admin")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for setting price=0, got %d: %s", w.Code, w.Body.String())
	}

	// Negative amount rejected -> 400.
	neg := int64(-100)
	body, _ = json.Marshal(ordershttp.SetPriceRequest{AmountMinor: &neg, Currency: "CNY"})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/courses/crs_1/price", bytes.NewReader(body)), "tok_admin")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for negative price, got %d", w.Code)
	}

	// Non-CNY currency rejected -> 400.
	body, _ = json.Marshal(ordershttp.SetPriceRequest{AmountMinor: &amt, Currency: "USD"})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/courses/crs_1/price", bytes.NewReader(body)), "tok_admin")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for USD currency, got %d", w.Code)
	}
}

func TestOrderOwnershipIsolationSortingAndCancel(t *testing.T) {
	env := setupTestCommerce(nil, nil, nil, nil)
	_, _ = env.orderStore.SetCoursePrice(context.Background(), "crs_iso_1", 9900, "CNY", 0)
	_, _ = env.orderStore.SetCoursePrice(context.Background(), "crs_iso_2", 19900, "CNY", 0)

	ord1, _ := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_iso_1", "k1")
	time.Sleep(2 * time.Millisecond)
	ord2, _ := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_iso_2", "k2")

	// buyer_2 MUST NOT be able to view buyer_1's order -> 403!
	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/commerce/orders/"+ord1.ID, nil), "tok_buyer_2")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when reading another user's order, got %d", w.Code)
	}

	// buyer_2 MUST NOT be able to create/list payment attempts or query status for buyer_1's order -> 403!
	attBody, _ := json.Marshal(paymentshttp.CreatePaymentAttemptRequest{Channel: "wechat"})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord1.ID+"/payment-attempts", bytes.NewReader(attBody)), "tok_buyer_2")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner creating payment attempt, got %d", w.Code)
	}

	req = withCookie(httptest.NewRequest(http.MethodGet, "/api/commerce/orders/"+ord1.ID+"/payment-attempts", nil), "tok_buyer_2")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner listing payment attempts, got %d", w.Code)
	}

	// Admin CAN view buyer_1's order -> 200.
	req = withCookie(httptest.NewRequest(http.MethodGet, "/api/commerce/orders/"+ord1.ID, nil), "tok_admin")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin reading order, got %d", w.Code)
	}

	// buyer_1 lists orders -> sorted CreatedAt DESC (ord2 before ord1).
	req = withCookie(httptest.NewRequest(http.MethodGet, "/api/commerce/orders?page=1&limit=10", nil), "tok_buyer_1")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 listing orders, got %d", w.Code)
	}
	var listResp ordershttp.OrderListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Total != 2 || len(listResp.Items) != 2 || listResp.Items[0].ID != ord2.ID || listResp.Items[1].ID != ord1.ID {
		t.Fatalf("expected ord2 then ord1 (CreatedAt DESC), got %+v", listResp.Items)
	}

	// buyer_2 cannot cancel buyer_1's order -> 403.
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord1.ID+"/cancel", nil), "tok_buyer_2")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner cancelling order, got %d", w.Code)
	}

	// buyer_1 cancels pending_payment order -> 200 cancelled.
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord1.ID+"/cancel", nil), "tok_buyer_1")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 cancelling pending_payment order, got %d: %s", w.Code, w.Body.String())
	}
	var cancelled ordershttp.OrderResponse
	_ = json.Unmarshal(w.Body.Bytes(), &cancelled)
	if cancelled.Status != string(ordersdomain.StatusCancelled) {
		t.Fatalf("expected cancelled status, got %s", cancelled.Status)
	}

	// Late payment webhook on cancelled order MUST be rejected (409) and NEVER overwrite to paid!
	sig := paymentsapp.ComputeWebhookSignature(testWebhookSecret, ord1.ID, "wechat", "tx_late_cancel", 9900, "CNY")
	wh, _ := json.Marshal(paymentshttp.WebhookConfirmationRequest{
		OrderID: ord1.ID, Channel: "wechat", ChannelOrderID: "tx_late_cancel",
		AmountMinor: 9900, Currency: "CNY", Signature: sig,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(wh))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for late callback on cancelled order, got %d: %s", w.Code, w.Body.String())
	}
	after, _ := env.orderStore.GetOrder(context.Background(), ord1.ID)
	if after.Status != ordersdomain.StatusCancelled {
		t.Fatalf("cancelled order was overwritten to %s", after.Status)
	}
	anomalies, _ := env.payStore.ListAnomalies(context.Background(), ord1.ID)
	if len(anomalies) != 1 {
		t.Fatalf("expected 1 recorded anomaly for late callback on cancelled order, got %d", len(anomalies))
	}
}

func TestPaymentRejectsInvalidHMACAndMismatchedAmount(t *testing.T) {
	env := setupTestCommerce(nil, nil, nil, nil)
	_, _ = env.orderStore.SetCoursePrice(context.Background(), "crs_pay_1", 9900, "CNY", 0)
	ord, err := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_pay_1", "idemp-pay-1")
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// 1. Valid HMAC signature for wrong amount (5000 vs 9900) -> 422.
	badAmtSig := paymentsapp.ComputeWebhookSignature(testWebhookSecret, ord.ID, "mock_pay", "tx_mock_1", 5000, "CNY")
	mismatchedPayload, _ := json.Marshal(paymentshttp.WebhookConfirmationRequest{
		OrderID: ord.ID, Channel: "mock_pay", ChannelOrderID: "tx_mock_1",
		AmountMinor: 5000, Currency: "CNY", Signature: badAmtSig,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(mismatchedPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for mismatched payment amount, got %d: %s", w.Code, w.Body.String())
	}

	allowed, _, _, _ := env.entStore.CheckEntitlement(context.Background(), "buyer_1", "crs_pay_1", "playback")
	if allowed {
		t.Fatalf("entitlement must NOT be granted when payment facts mismatch")
	}

	// 2. Invalid HMAC signature -> 422.
	invalidSigPayload, _ := json.Marshal(paymentshttp.WebhookConfirmationRequest{
		OrderID: ord.ID, Channel: "mock_pay", ChannelOrderID: "tx_mock_1",
		AmountMinor: 9900, Currency: "CNY", Signature: "not-a-valid-hmac",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(invalidSigPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid HMAC signature, got %d", w.Code)
	}
}

func TestPaymentIdempotencyUnknownRecoveryAndInternalEntitlements(t *testing.T) {
	var queryShouldFail bool
	querier := func(_ context.Context, att *paymentsdomain.PaymentAttempt) (*paymentsdomain.ChannelQueryResult, error) {
		if queryShouldFail {
			return nil, errors.New("gateway timeout")
		}
		return &paymentsdomain.ChannelQueryResult{
			Status:         paymentsdomain.StatusSuccess,
			ChannelOrderID: "tx_recovered_1",
			AmountMinor:    att.AmountMinor,
			Currency:       att.Currency,
		}, nil
	}

	env := setupTestCommerce(nil, nil, querier, nil)
	_, _ = env.orderStore.SetCoursePrice(context.Background(), "crs_pay_2", 19900, "CNY", 0)
	_, _ = env.orderStore.SetCoursePrice(context.Background(), "crs_pay_3", 29900, "CNY", 0)

	// 1. Standard HMAC webhook + idempotent replay
	ord, _ := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_pay_2", "idemp-pay-2")
	validSig := paymentsapp.ComputeWebhookSignature(testWebhookSecret, ord.ID, "mock_pay", "tx_mock_success", 19900, "CNY")
	validPayload, _ := json.Marshal(paymentshttp.WebhookConfirmationRequest{
		OrderID: ord.ID, Channel: "mock_pay", ChannelOrderID: "tx_mock_success",
		AmountMinor: 19900, Currency: "CNY", Signature: validSig,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(validPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid payment, got %d: %s", w.Code, w.Body.String())
	}

	// Duplicate replay with same channelOrderId -> idempotent 200 OK.
	req2 := httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(validPayload))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	env.router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for replayed payment, got %d", w2.Code)
	}

	// Duplicate payment with DIFFERENT channelOrderId -> 409 conflict + anomaly recorded.
	dupSig := paymentsapp.ComputeWebhookSignature(testWebhookSecret, ord.ID, "mock_pay", "tx_extra_charge", 19900, "CNY")
	dupPayload, _ := json.Marshal(paymentshttp.WebhookConfirmationRequest{
		OrderID: ord.ID, Channel: "mock_pay", ChannelOrderID: "tx_extra_charge",
		AmountMinor: 19900, Currency: "CNY", Signature: dupSig,
	})
	reqDup := httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(dupPayload))
	reqDup.Header.Set("Content-Type", "application/json")
	wDup := httptest.NewRecorder()
	env.router.ServeHTTP(wDup, reqDup)
	if wDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 for extra payment with different channelOrderId, got %d", wDup.Code)
	}

	// 2. Active payment-status recovery for unknown attempt
	ord3, _ := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_pay_3", "idemp-pay-3")
	att, _ := env.payStore.CreatePaymentAttempt(context.Background(), ord3.ID, "wechat", 29900, "CNY")
	_, _ = env.payStore.MarkAttemptUnknown(context.Background(), att.ID)

	// When channel query fails -> remains unknown, does NOT fabricate success.
	queryShouldFail = true
	req = withCookie(httptest.NewRequest(http.MethodGet, "/api/commerce/orders/"+ord3.ID+"/payment-status", nil), "tok_buyer_1")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for payment-status query, got %d", w.Code)
	}
	var statusResp paymentshttp.PaymentStatusResponse
	_ = json.Unmarshal(w.Body.Bytes(), &statusResp)
	if statusResp.PaymentStatus != string(paymentsdomain.StatusUnknown) || statusResp.OrderStatus != string(ordersdomain.StatusPendingPayment) {
		t.Fatalf("expected unknown/pending_payment when gateway unreachable, got %+v", statusResp)
	}

	// When channel query succeeds -> recovers to paid & grants entitlement!
	queryShouldFail = false
	req = withCookie(httptest.NewRequest(http.MethodGet, "/api/commerce/orders/"+ord3.ID+"/payment-status", nil), "tok_buyer_1")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for payment-status recovery, got %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &statusResp)
	if !statusResp.Recovered || statusResp.OrderStatus != string(ordersdomain.StatusPaid) || statusResp.PaymentStatus != string(paymentsdomain.StatusSuccess) {
		t.Fatalf("expected recovered paid status, got %+v", statusResp)
	}

	grants, err := env.entStore.ListUserEntitlements(t.Context(), "buyer_1")
	if err != nil || len(grants) != 2 {
		t.Fatalf("expected two grants, got %d, %v", len(grants), err)
	}
}

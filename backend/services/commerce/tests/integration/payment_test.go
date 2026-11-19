package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/entitlements"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/payments"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/refunds"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/server"
)

func setupTestCommerce() (*orders.MemoryStore, *payments.MemoryStore, *entitlements.MemoryStore, *refunds.MemoryStore, http.Handler) {
	entStore := entitlements.NewMemoryStore()

	orderStore := orders.NewMemoryStore(
		nil,
		func(userID, courseID string) (bool, error) {
			allowed, _, _, err := entStore.CheckEntitlement(context.Background(), userID, courseID, "order_check")
			return allowed, err
		},
	)

	payStore := payments.NewMemoryStore(
		orderStore,
		func(ctx context.Context, userID, courseID, orderID string) error {
			_, err := entStore.GrantEntitlement(ctx, userID, courseID, orderID)
			return err
		},
	)

	refundStore := refunds.NewMemoryStore(orderStore, entStore, nil)
	r := server.NewWithStores(false, orderStore, payStore, entStore, refundStore)
	return orderStore, payStore, entStore, refundStore, r
}

func TestPaymentRejectsInvalidFacts(t *testing.T) {
	orderStore, _, entStore, _, router := setupTestCommerce()

	// 1. Set price: 9900 cents (99.00 CNY)
	_, _ = orderStore.SetCoursePrice(context.Background(), "crs_pay_1", 9900, "CNY", 0)

	// 2. Create order
	ord, err := orderStore.CreateOrder(context.Background(), "user_1", "crs_pay_1", "idemp-pay-1")
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// 3. Webhook callback with mismatched amount (e.g. 5000 cents instead of 9900)
	mismatchedPayload, _ := json.Marshal(payments.WebhookConfirmationRequest{
		OrderID:        ord.ID,
		Channel:        "mock_pay",
		ChannelOrderID: "tx_mock_1",
		AmountMinor:    5000,
		Currency:       "CNY",
		Signature:      "valid_mock_signature",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(mismatchedPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for mismatched payment amount, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Verify no entitlement granted
	allowed, _, _, _ := entStore.CheckEntitlement(context.Background(), "user_1", "crs_pay_1", "playback")
	if allowed {
		t.Fatalf("entitlement must NOT be granted when payment facts mismatch")
	}

	// 5. Webhook callback with invalid signature
	invalidSigPayload, _ := json.Marshal(payments.WebhookConfirmationRequest{
		OrderID:        ord.ID,
		Channel:        "mock_pay",
		ChannelOrderID: "tx_mock_1",
		AmountMinor:    9900,
		Currency:       "CNY",
		Signature:      "invalid_sig",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(invalidSigPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid signature, got %d", w.Code)
	}
}

func TestPaymentIdempotencyAndUnknown(t *testing.T) {
	orderStore, _, entStore, _, router := setupTestCommerce()

	_, _ = orderStore.SetCoursePrice(context.Background(), "crs_pay_2", 19900, "CNY", 0)
	ord, _ := orderStore.CreateOrder(context.Background(), "user_2", "crs_pay_2", "idemp-pay-2")

	validPayload, _ := json.Marshal(payments.WebhookConfirmationRequest{
		OrderID:        ord.ID,
		Channel:        "mock_pay",
		ChannelOrderID: "tx_mock_success",
		AmountMinor:    19900,
		Currency:       "CNY",
		Signature:      "sig_ok",
	})

	// First callback
	req := httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(validPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid payment, got %d: %s", w.Code, w.Body.String())
	}

	// Verify entitlement active
	allowed, grantVer, _, _ := entStore.CheckEntitlement(context.Background(), "user_2", "crs_pay_2", "playback")
	if !allowed || grantVer != 1 {
		t.Fatalf("expected active entitlement grant version 1, got %v / %d", allowed, grantVer)
	}

	// Duplicate second callback (replayed webhook)
	req2 := httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(validPayload))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for replayed payment, got %d", w2.Code)
	}

	// Entitlement should remain exactly 1, no duplicate grants
	userGrants, _ := entStore.ListUserEntitlements(context.Background(), "user_2")
	if len(userGrants) != 1 {
		t.Fatalf("expected exactly 1 entitlement grant for user, got %d", len(userGrants))
	}

	// 3. Attempting to purchase the same course again must be rejected (already entitled)
	_, err := orderStore.CreateOrder(context.Background(), "user_2", "crs_pay_2", "new_idemp_key")
	if err != orders.ErrAlreadyEntitled {
		t.Fatalf("expected ErrAlreadyEntitled for duplicate order creation, got %v", err)
	}
}

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/orders"
	"github.com/StephenQiu30/ai2workspace/backend/services/commerce/internal/refunds"
)

func TestRefundWindowAndExceptions(t *testing.T) {
	orderStore, payStore, entStore, _, router := setupTestCommerce()

	_, _ = orderStore.SetCoursePrice(context.Background(), "crs_ref_1", 8800, "CNY", 0)
	ord, _ := orderStore.CreateOrder(context.Background(), "buyer_ref_1", "crs_ref_1", "idemp_ref_1")

	// Confirm payment
	_, err := payStore.ConfirmPayment(context.Background(), ord.ID, "mock_pay", "tx_ref_1", 8800, "CNY", "sig_valid")
	if err != nil {
		t.Fatalf("payment failed: %v", err)
	}

	// 1. Submit refund application within 7x24h
	refReq, _ := json.Marshal(refunds.ApplyRefundRequest{
		Reason: "课程内容与预期不符",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord.ID+"/refunds", bytes.NewReader(refReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "buyer_ref_1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for refund application, got %d: %s", w.Code, w.Body.String())
	}

	var refResp refunds.RefundResponse
	_ = json.Unmarshal(w.Body.Bytes(), &refResp)
	if refResp.ApplicationState != "pending" || refResp.AmountMinor != 8800 {
		t.Fatalf("unexpected refund response: %+v", refResp)
	}

	// 2. Admin approves refund -> entitlement revoked
	revReq, _ := json.Marshal(refunds.ReviewRefundRequest{
		Decision:        "approved",
		Reason:          "符合7天无理由退款政策",
		ExpectedVersion: 1,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/review", bytes.NewReader(revReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin_1")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for admin approval, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Verify entitlement is revoked
	allowed, _, reason, _ := entStore.CheckEntitlement(context.Background(), "buyer_ref_1", "crs_ref_1", "playback")
	if allowed {
		t.Fatalf("entitlement should be REVOKED after refund approval, reason=%s", reason)
	}

	// 4. Verify order marked as refunded
	currentOrd, _ := orderStore.GetOrder(context.Background(), ord.ID)
	if currentOrd.Status != orders.StatusRefunded {
		t.Fatalf("order status should be refunded, got %s", currentOrd.Status)
	}
}

func TestRefundReviewRejection(t *testing.T) {
	orderStore, payStore, entStore, _, router := setupTestCommerce()

	_, _ = orderStore.SetCoursePrice(context.Background(), "crs_ref_2", 5000, "CNY", 0)
	ord, _ := orderStore.CreateOrder(context.Background(), "buyer_ref_2", "crs_ref_2", "idemp_ref_2")
	_, _ = payStore.ConfirmPayment(context.Background(), ord.ID, "mock_pay", "tx_ref_2", 5000, "CNY", "sig_valid")

	refReq, _ := json.Marshal(refunds.ApplyRefundRequest{Reason: "恶意退款申请"})
	req := httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord.ID+"/refunds", bytes.NewReader(refReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "buyer_ref_2")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var refResp refunds.RefundResponse
	_ = json.Unmarshal(w.Body.Bytes(), &refResp)

	// Admin rejects refund
	revReq, _ := json.Marshal(refunds.ReviewRefundRequest{
		Decision:        "rejected",
		Reason:          "不符合退款条件",
		ExpectedVersion: 1,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/review", bytes.NewReader(revReq))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for rejection, got %d", w.Code)
	}

	// Entitlement remains intact
	allowed, _, _, _ := entStore.CheckEntitlement(context.Background(), "buyer_ref_2", "crs_ref_2", "playback")
	if !allowed {
		t.Fatalf("entitlement must remain ACTIVE after refund rejection")
	}
}

func TestUnpaidOrderCannotBeRefunded(t *testing.T) {
	orderStore, _, _, _, router := setupTestCommerce()

	_, _ = orderStore.SetCoursePrice(context.Background(), "crs_unpaid", 3000, "CNY", 0)
	ord, _ := orderStore.CreateOrder(context.Background(), "buyer_unpaid", "crs_unpaid", "idemp_unpaid")

	refReq, _ := json.Marshal(refunds.ApplyRefundRequest{Reason: "未付款就要退款"})
	req := httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord.ID+"/refunds", bytes.NewReader(refReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "buyer_unpaid")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for refunding unpaid order, got %d", w.Code)
	}
}

func TestRefundWindowExpired(t *testing.T) {
	orderStore, payStore, _, _, router := setupTestCommerce()

	_, _ = orderStore.SetCoursePrice(context.Background(), "crs_expired", 3000, "CNY", 0)
	ord, _ := orderStore.CreateOrder(context.Background(), "buyer_old", "crs_expired", "idemp_old")

	// Set paidAt to 10 days ago (past the 7x24h deadline)
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	_, _ = payStore.ConfirmPayment(context.Background(), ord.ID, "mock_pay", "tx_old", 3000, "CNY", "sig_valid")
	ord.PaidAt = &tenDaysAgo
	ord.CreatedAt = tenDaysAgo

	refReq, _ := json.Marshal(refunds.ApplyRefundRequest{Reason: "买了太久想退款"})
	req := httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord.ID+"/refunds", bytes.NewReader(refReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "buyer_old")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for expired refund window, got %d", w.Code)
	}
}

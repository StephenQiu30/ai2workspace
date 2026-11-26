package commerce_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	paymentshttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/payments"
	refundshttp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/http/refunds"
	paymentsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/payments"
	refundsapp "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/application/refunds"
	ordersdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/orders"
	refundsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/refunds"
)

func payOrderForTest(t *testing.T, env *testCommerceEnv, ord *ordersdomain.Order, channelOrderID string) {
	t.Helper()
	sig := paymentsapp.ComputeWebhookSignature(testWebhookSecret, ord.ID, "wechat", channelOrderID, ord.AmountMinor, ord.Currency)
	_, err := env.payStore.ConfirmPayment(context.Background(), ord.ID, "wechat", channelOrderID, ord.AmountMinor, ord.Currency, sig)
	if err != nil {
		t.Fatalf("confirm payment failed: %v", err)
	}
}

func TestRefundApprovalKeepsEntitlementUntilChannelConfirmSuccess(t *testing.T) {
	var notifiedEvents []string
	notifier := func(_ context.Context, eventID, _, _, _, _ string) error {
		notifiedEvents = append(notifiedEvents, eventID)
		return nil
	}

	env := setupTestCommerce(nil, nil, nil, notifier)
	_, _ = env.orderStore.SetCoursePrice(context.Background(), "crs_ref_1", 8800, "CNY", 0)
	ord, _ := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_ref_1", "idemp_ref_1")
	payOrderForTest(t, env, ord, "tx_ref_1")

	// 1. Submit refund application within 7x24h -> pending_review
	refReq, _ := json.Marshal(refundshttp.ApplyRefundRequest{Reason: "课程内容与预期不符"})
	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord.ID+"/refunds", bytes.NewReader(refReq)), "tok_buyer_1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for refund application, got %d: %s", w.Code, w.Body.String())
	}
	var refResp refundshttp.RefundResponse
	_ = json.Unmarshal(w.Body.Bytes(), &refResp)
	if refResp.ApplicationState != string(refundsdomain.StatePendingReview) || refResp.ChannelState != string(refundsdomain.ChannelNone) {
		t.Fatalf("unexpected initial refund state: %+v", refResp)
	}

	// Non-admin cannot list all refunds or review -> 403.
	req = withCookie(httptest.NewRequest(http.MethodGet, "/api/commerce/refunds", nil), "tok_buyer_1")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin listing refunds, got %d", w.Code)
	}

	// Other user cannot view buyer_1's refund or order refunds -> 403.
	req = withCookie(httptest.NewRequest(http.MethodGet, "/api/commerce/refunds/"+refResp.ID, nil), "tok_buyer_2")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for other user viewing refund, got %d", w.Code)
	}

	req = withCookie(httptest.NewRequest(http.MethodGet, "/api/commerce/orders/"+ord.ID+"/refunds", nil), "tok_buyer_2")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for other user viewing order refunds, got %d", w.Code)
	}

	// Owner CAN list refunds for their own order -> 200.
	req = withCookie(httptest.NewRequest(http.MethodGet, "/api/commerce/orders/"+ord.ID+"/refunds", nil), "tok_buyer_1")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for owner listing order refunds, got %d", w.Code)
	}

	// Invalid review decision rejected -> 400.
	badRev, _ := json.Marshal(refundshttp.ReviewRefundRequest{Decision: "invalid_decision"})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/review", bytes.NewReader(badRev)), "tok_admin")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid decision, got %d", w.Code)
	}

	// 2. Admin approves refund -> approved + submitting; entitlement MUST REMAIN ACTIVE!
	revReq, _ := json.Marshal(refundshttp.ReviewRefundRequest{
		Decision:        "approved",
		Reason:          "符合7天无理由退款政策",
		ExpectedVersion: 1,
	})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/review", bytes.NewReader(revReq)), "tok_admin")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for admin approval, got %d: %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &refResp)
	if refResp.ApplicationState != string(refundsdomain.StateApproved) || refResp.ChannelState != string(refundsdomain.ChannelSubmitting) || refResp.ReviewerID != "admin_1" {
		t.Fatalf("unexpected refund state after approval: %+v", refResp)
	}

	allowed, _, _, _ := env.entStore.CheckEntitlement(context.Background(), "buyer_1", "crs_ref_1", "playback")
	if !allowed {
		t.Fatalf("entitlement MUST remain ACTIVE after admin approval while channelState=submitting")
	}
	currentOrd, _ := env.orderStore.GetOrder(context.Background(), ord.ID)
	if currentOrd.Status != ordersdomain.StatusPaid {
		t.Fatalf("order MUST remain paid until channel refund succeeds, got %s", currentOrd.Status)
	}

	// Owner cannot cancel an already-approved refund -> 409.
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/cancel", nil), "tok_buyer_1")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 cancelling approved refund, got %d", w.Code)
	}

	// 3. Channel reports unknown -> entitlement STILL ACTIVE!
	unkReq, _ := json.Marshal(refundshttp.ChannelConfirmRefundRequest{Status: "unknown", FailureReason: "gateway timeout"})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/channel-confirm", bytes.NewReader(unkReq)), "tok_admin")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown channel status, got %d", w.Code)
	}
	allowed, _, _, _ = env.entStore.CheckEntitlement(context.Background(), "buyer_1", "crs_ref_1", "playback")
	if !allowed {
		t.Fatalf("entitlement MUST remain ACTIVE while channelState=unknown")
	}

	// Non-admin user cannot confirm channel refund -> 403.
	okReq, _ := json.Marshal(refundshttp.ChannelConfirmRefundRequest{Status: "success", ChannelRefundID: "ch_ref_1"})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/channel-confirm", bytes.NewReader(okReq)), "tok_buyer_1")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for student calling channel-confirm, got %d", w.Code)
	}

	// 4. Channel confirms success via HMAC signature -> NOW entitlement is revoked & order is refunded!
	refSig := refundsapp.ComputeChannelConfirmSignature(testWebhookSecret, refResp.ID, "ch_ref_1", "success")
	okSignedReq, _ := json.Marshal(refundshttp.ChannelConfirmRefundRequest{
		Status: "success", ChannelRefundID: "ch_ref_1", Signature: refSig,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/channel-confirm", bytes.NewReader(okSignedReq))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for signed channel-confirm success, got %d: %s", w.Code, w.Body.String())
	}

	allowed, _, reason, _ := env.entStore.CheckEntitlement(context.Background(), "buyer_1", "crs_ref_1", "playback")
	if allowed {
		t.Fatalf("entitlement MUST be REVOKED after channel-confirm success, reason=%s", reason)
	}
	currentOrd, _ = env.orderStore.GetOrder(context.Background(), ord.ID)
	if currentOrd.Status != ordersdomain.StatusRefunded {
		t.Fatalf("order status must be refunded after channel-confirm success, got %s", currentOrd.Status)
	}

	// 5. Late channel-confirm failed on already-succeeded refund -> 409 terminal state protection!
	lateFail, _ := json.Marshal(refundshttp.ChannelConfirmRefundRequest{Status: "failed"})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/channel-confirm", bytes.NewReader(lateFail))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", refundsapp.ComputeChannelConfirmSignature(testWebhookSecret, refResp.ID, "", "failed"))
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 rejecting terminal state regression, got %d", w.Code)
	}

	// 6. Late payment webhook on refunded order MUST be rejected (409) and NOT restore paid!
	latePaySig := paymentsapp.ComputeWebhookSignature(testWebhookSecret, ord.ID, "wechat", "tx_ref_1_late", 8800, "CNY")
	latePayBody, _ := json.Marshal(paymentshttp.WebhookConfirmationRequest{
		OrderID: ord.ID, Channel: "wechat", ChannelOrderID: "tx_ref_1_late",
		AmountMinor: 8800, Currency: "CNY", Signature: latePaySig,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/commerce/payments/webhook", bytes.NewReader(latePayBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for late payment webhook on refunded order, got %d: %s", w.Code, w.Body.String())
	}
	currentOrd, _ = env.orderStore.GetOrder(context.Background(), ord.ID)
	if currentOrd.Status != ordersdomain.StatusRefunded {
		t.Fatalf("refunded order was overwritten to %s by late payment callback", currentOrd.Status)
	}
	allowed, _, _, _ = env.entStore.CheckEntitlement(context.Background(), "buyer_1", "crs_ref_1", "playback")
	if allowed {
		t.Fatalf("late payment callback on refunded order must NOT re-grant entitlement")
	}

	// Verify both order_paid and refund_succeeded deduplicated notifications fired.
	if len(notifiedEvents) != 2 {
		t.Fatalf("expected 2 notifications (order_paid + refund_succeeded), got %v", notifiedEvents)
	}
}

func TestCancelPendingRefundApplication(t *testing.T) {
	env := setupTestCommerce(nil, nil, nil, nil)
	_, _ = env.orderStore.SetCoursePrice(context.Background(), "crs_cancel_ref", 5000, "CNY", 0)
	ord, _ := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_cancel_ref", "idemp_cancel_ref")
	payOrderForTest(t, env, ord, "tx_cancel_ref")

	refReq, _ := json.Marshal(refundshttp.ApplyRefundRequest{Reason: "误操作申请退款"})
	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord.ID+"/refunds", bytes.NewReader(refReq)), "tok_buyer_1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	var refResp refundshttp.RefundResponse
	_ = json.Unmarshal(w.Body.Bytes(), &refResp)

	// Other user cannot cancel buyer_1's refund -> 403.
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/cancel", nil), "tok_buyer_2")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner cancelling refund, got %d", w.Code)
	}

	// Owner withdraws pending_review refund -> 200 cancelled.
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/cancel", nil), "tok_buyer_1")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 withdrawing pending refund, got %d: %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &refResp)
	if refResp.ApplicationState != string(refundsdomain.StateCancelled) {
		t.Fatalf("expected cancelled state, got %s", refResp.ApplicationState)
	}

	// Admin attempting to review already-cancelled refund -> 409 conflict!
	revReq, _ := json.Marshal(refundshttp.ReviewRefundRequest{Decision: "approved"})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+refResp.ID+"/review", bytes.NewReader(revReq)), "tok_admin")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 reviewing cancelled refund, got %d", w.Code)
	}
}

func TestOutOfWindowExceptionAndDelayedDeliveryEvidence(t *testing.T) {
	twoDaysAgo := time.Now().UTC().Add(-2 * 24 * time.Hour)
	tenDaysAgo := time.Now().UTC().Add(-10 * 24 * time.Hour)

	var evidenceMode string
	evidenceProvider := func(_ context.Context, _ string, _ time.Time) (*time.Time, error) {
		switch evidenceMode {
		case "error":
			return nil, errors.New("learning service unreachable")
		case "delayed":
			return &twoDaysAgo, nil
		default:
			return &tenDaysAgo, nil
		}
	}

	env := setupTestCommerce(nil, evidenceProvider, nil, nil)
	_, _ = env.orderStore.SetCoursePrice(context.Background(), "crs_win", 3000, "CNY", 0)
	ord, _ := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_win", "idemp_win")
	payOrderForTest(t, env, ord, "tx_win")
	_ = env.orderStore.SetTimestampsForTest(ord.ID, tenDaysAgo, &tenDaysAgo)

	// 1. Evidence provider error -> MUST return 503 (never silently shorten window!).
	evidenceMode = "error"
	body, _ := json.Marshal(refundshttp.ApplyRefundRequest{Reason: "测试证据异常"})
	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord.ID+"/refunds", bytes.NewReader(body)), "tok_buyer_1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when delivery evidence fails, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Normal expired (delivered 10 days ago) without exceptionType -> 400 rejected!
	evidenceMode = "old"
	body, _ = json.Marshal(refundshttp.ApplyRefundRequest{Reason: "买了太久想退款"})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord.ID+"/refunds", bytes.NewReader(body)), "tok_buyer_1")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for normal out-of-window refund, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Out-of-window WITH valid exceptionType -> 201 pending_review + isOutOfWindow=true!
	body, _ = json.Marshal(refundshttp.ApplyRefundRequest{
		Reason:        "视频严重缺帧无法观看",
		ExceptionType: refundsdomain.ExceptionQualityIssue,
	})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord.ID+"/refunds", bytes.NewReader(body)), "tok_buyer_1")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for out-of-window quality_issue exception, got %d: %s", w.Code, w.Body.String())
	}
	var exResp refundshttp.RefundResponse
	_ = json.Unmarshal(w.Body.Bytes(), &exResp)
	if !exResp.IsOutOfWindow || exResp.ApplicationState != string(refundsdomain.StatePendingReview) || exResp.ExceptionType != refundsdomain.ExceptionQualityIssue {
		t.Fatalf("unexpected out-of-window exception response: %+v", exResp)
	}

	// 4. Delayed delivery: paid 10 days ago, first deliverable 2 days ago -> window starts 2 days ago -> within 7x24h!
	evidenceMode = "delayed"
	ord2, _ := env.orderStore.CreateOrder(context.Background(), "buyer_2", "crs_win", "idemp_win_2")
	payOrderForTest(t, env, ord2, "tx_win_2")
	_ = env.orderStore.SetTimestampsForTest(ord2.ID, tenDaysAgo, &tenDaysAgo)

	body, _ = json.Marshal(refundshttp.ApplyRefundRequest{Reason: "延迟开课后7天内申请退款"})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+ord2.ID+"/refunds", bytes.NewReader(body)), "tok_buyer_2")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for delayed delivery within 7x24h of firstDeliverableAt, got %d: %s", w.Code, w.Body.String())
	}
	var delResp refundshttp.RefundResponse
	_ = json.Unmarshal(w.Body.Bytes(), &delResp)
	if delResp.IsOutOfWindow {
		t.Fatalf("delayed delivery within 7x24h must NOT be marked out of window")
	}
}

func TestMultiOrderAndRepurchaseRevocationIsolation(t *testing.T) {
	env := setupTestCommerce(nil, nil, nil, nil)
	_, _ = env.orderStore.SetCoursePrice(context.Background(), "crs_repurchase", 9900, "CNY", 0)

	// 1. First purchase -> pay -> refund -> approve -> channel confirm success.
	ord1, _ := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_repurchase", "idemp_buy_1")
	payOrderForTest(t, env, ord1, "tx_buy_1")

	ref1, err := env.refundStore.ApplyRefund(context.Background(), "buyer_1", ord1.ID, "先退一次", "")
	if err != nil {
		t.Fatalf("apply refund 1 failed: %v", err)
	}
	_, err = env.refundStore.ReviewRefund(context.Background(), "admin_1", ref1.ID, "approved", "", 1)
	if err != nil {
		t.Fatalf("review refund 1 failed: %v", err)
	}
	_, err = env.refundStore.ConfirmChannelRefund(context.Background(), ref1.ID, refundsdomain.ChannelSuccess, "ch_ref_1", "", 2)
	if err != nil {
		t.Fatalf("channel confirm 1 failed: %v", err)
	}

	allowed, _, _, _ := env.entStore.CheckEntitlement(context.Background(), "buyer_1", "crs_repurchase", "playback")
	if allowed {
		t.Fatalf("entitlement should be revoked after first refund succeeded")
	}

	// 2. Repurchase same course -> new Order 2 -> pay -> entitlement active again!
	ord2, err := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_repurchase", "idemp_buy_2")
	if err != nil {
		t.Fatalf("repurchase failed: %v", err)
	}
	payOrderForTest(t, env, ord2, "tx_buy_2")

	// Replay old refund channel-confirm success for ord1 -> MUST NOT revoke ord2's active entitlement!
	_, err = env.refundStore.ConfirmChannelRefund(context.Background(), ref1.ID, refundsdomain.ChannelSuccess, "ch_ref_1", "", 0)
	if err != nil {
		t.Fatalf("idempotent replay of old refund failed: %v", err)
	}
	allowed, _, _, _ = env.entStore.CheckEntitlement(context.Background(), "buyer_1", "crs_repurchase", "playback")
	if !allowed {
		t.Fatalf("repurchased order 2 entitlement MUST stay active after old refund replay")
	}

	// 3. Duplicate-charge isolation: two active grants on same (user, course); revoking one keeps the other active!
	_, err = env.entStore.GrantEntitlement(context.Background(), "buyer_1", "crs_repurchase", "ord_dup_extra")
	if err != nil {
		t.Fatalf("grant extra failed: %v", err)
	}
	if err := env.entStore.RevokeEntitlement(context.Background(), "ord_dup_extra"); err != nil {
		t.Fatalf("revoke extra failed: %v", err)
	}
	allowed, _, _, _ = env.entStore.CheckEntitlement(context.Background(), "buyer_1", "crs_repurchase", "playback")
	if !allowed {
		t.Fatalf("revoking duplicate order grant must NOT revoke legitimate ord2 grant")
	}
}

func TestRefundRejectionUnpaidOrderAndConcurrentCancelVsReview(t *testing.T) {
	env := setupTestCommerce(nil, nil, nil, nil)
	_, _ = env.orderStore.SetCoursePrice(context.Background(), "crs_conc", 6600, "CNY", 0)

	// 1. Unpaid order cannot be refunded -> 400.
	unpaidOrd, _ := env.orderStore.CreateOrder(context.Background(), "buyer_1", "crs_conc", "idemp_unpaid")
	body, _ := json.Marshal(refundshttp.ApplyRefundRequest{Reason: "未付款申请退款"})
	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/orders/"+unpaidOrd.ID+"/refunds", bytes.NewReader(body)), "tok_buyer_1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for refunding unpaid order, got %d", w.Code)
	}

	// 2. Admin rejection keeps entitlement intact.
	payOrderForTest(t, env, unpaidOrd, "tx_conc_1")
	ref, _ := env.refundStore.ApplyRefund(context.Background(), "buyer_1", unpaidOrd.ID, "测试驳回", "")
	revBody, _ := json.Marshal(refundshttp.ReviewRefundRequest{Decision: "rejected", Reason: "不符合条件", ExpectedVersion: 1})
	req = withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+ref.ID+"/review", bytes.NewReader(revBody)), "tok_admin")
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for rejection, got %d", w.Code)
	}
	allowed, _, _, _ := env.entStore.CheckEntitlement(context.Background(), "buyer_1", "crs_conc", "playback")
	if !allowed {
		t.Fatalf("entitlement must remain active after refund rejection")
	}

	// 3. Concurrent cancel vs admin review: exactly ONE succeeds, the other gets 409 conflict.
	ref2, err := env.refundStore.ApplyRefund(context.Background(), "buyer_1", unpaidOrd.ID, "再次申请竞争测试", "")
	if err != nil {
		t.Fatalf("re-apply after rejection failed: %v", err)
	}

	var wg sync.WaitGroup
	codes := make([]int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		r := withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+ref2.ID+"/cancel", nil), "tok_buyer_1")
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, r)
		codes[0] = rec.Code
	}()
	go func() {
		defer wg.Done()
		rb, _ := json.Marshal(refundshttp.ReviewRefundRequest{Decision: "approved", ExpectedVersion: 1})
		r := withCookie(httptest.NewRequest(http.MethodPost, "/api/commerce/refunds/"+ref2.ID+"/review", bytes.NewReader(rb)), "tok_admin")
		r.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		env.router.ServeHTTP(rec, r)
		codes[1] = rec.Code
	}()
	wg.Wait()

	okCount := 0
	conflictCount := 0
	for _, code := range codes {
		if code == http.StatusOK {
			okCount++
		} else if code == http.StatusConflict {
			conflictCount++
		}
	}
	if okCount != 1 || conflictCount != 1 {
		t.Fatalf("expected exactly one 200 and one 409 in cancel-vs-review race, got codes=%v", codes)
	}
}

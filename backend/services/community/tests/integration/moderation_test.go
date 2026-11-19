package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/moderation"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/notification"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/posts"
)

func TestHiddenContentDoesNotLeak(t *testing.T) {
	postStore, _, _, _, router := setupTestCommunity()

	// 1. Create a post
	p, _ := postStore.CreatePost(context.Background(), posts.ScopeGlobal, "", "violator", "Bad Content", "Summary", "Violating Body Text")

	// 2. Submit report against this post
	repPayload, _ := json.Marshal(moderation.CreateReportRequest{
		TargetType: "post",
		TargetID:   p.ID,
		Reason:     "广告垃圾信息与违规内容",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/community/reports", bytes.NewReader(repPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "reporter_user")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for report creation, got %d", w.Code)
	}

	var rep moderation.Report
	_ = json.Unmarshal(w.Body.Bytes(), &rep)

	// 3. Admin moderates and approves report -> hides post
	revPayload, _ := json.Marshal(moderation.ModerateReportRequest{
		Decision: "approved",
		Reason:   "核实属违规推广，予以隐藏",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/community/reports/"+rep.ID+"/review", bytes.NewReader(revPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "admin_mod")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for moderation review, got %d", w.Code)
	}

	// 4. Directly query hidden post -> must return 404, never leak full body
	req = httptest.NewRequest(http.MethodGet, "/api/community/posts/"+p.ID, nil)
	req.Header.Set("X-User-ID", "any_user")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for hidden post, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotificationDeliveryAndIsolation(t *testing.T) {
	_, _, _, notifStore, router := setupTestCommunity()

	// 1. Internal event notification delivery
	payload, _ := json.Marshal(notification.AppendNotificationRequest{
		EventID:     "evt_order_123",
		RecipientID: "buyer_1",
		Type:        "order",
		Title:       "课程购买成功",
		Summary:     "您已成功支付课程，立即开始学习吧！",
	})

	req := httptest.NewRequest(http.MethodPost, "/internal/community/notifications", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for internal notification append, got %d: %s", w.Code, w.Body.String())
	}

	var n1 notification.Notification
	_ = json.Unmarshal(w.Body.Bytes(), &n1)

	// 2. Duplicate event re-delivery -> deduplicated, exactly notificationCount == 1
	req2 := httptest.NewRequest(http.MethodPost, "/internal/community/notifications", bytes.NewReader(payload))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("expected 201 for deduplicated notification append, got %d", w2.Code)
	}

	userNotifs, total, _ := notifStore.ListNotifications(context.Background(), "buyer_1", 1, 20)
	if total != 1 || len(userNotifs) != 1 {
		t.Fatalf("duplicate event must not create duplicate notifications, total=%d", total)
	}

	// 3. Another user cannot view or mark buyer_1's notification as read
	req = httptest.NewRequest(http.MethodPost, "/api/community/notifications/"+n1.ID+"/read", nil)
	req.Header.Set("X-User-ID", "other_user") // Mismatched recipient
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when marking another user's notification as read, got %d", w.Code)
	}

	// 4. Authorized user successfully marks read
	req = httptest.NewRequest(http.MethodPost, "/api/community/notifications/"+n1.ID+"/read", nil)
	req.Header.Set("X-User-ID", "buyer_1")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for marking own notification as read, got %d", w.Code)
	}
}

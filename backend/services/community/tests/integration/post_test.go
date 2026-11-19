package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/interaction"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/moderation"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/notification"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/posts"
	"github.com/StephenQiu30/ai2workspace/backend/services/community/internal/server"
)

func setupTestCommunity() (*posts.MemoryStore, *interaction.MemoryStore, *moderation.MemoryStore, *notification.MemoryStore, http.Handler) {
	postStore := posts.NewMemoryStore(func(courseID, userID string) (bool, error) {
		return userID != "unauthorized_user", nil
	})
	interactStore := interaction.NewMemoryStore()
	modStore := moderation.NewMemoryStore(postStore)
	notifStore := notification.NewMemoryStore()

	r := server.NewWithStores(false, postStore, interactStore, modStore, notifStore)
	return postStore, interactStore, modStore, notifStore, r
}

func TestReactionIsSingleState(t *testing.T) {
	postStore, _, _, _, router := setupTestCommunity()

	// 1. Create a post
	p, err := postStore.CreatePost(context.Background(), posts.ScopeGlobal, "", "user_author", "Test Reaction Post", "Summary", "Full body text")
	if err != nil {
		t.Fatalf("failed to create post: %v", err)
	}

	// 2. User 1 sends "like"
	likePayload, _ := json.Marshal(interaction.SetReactionRequest{
		TargetType: "post",
		TargetID:   p.ID,
		Value:      "like",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/community/reactions", bytes.NewReader(likePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for like, got %d: %s", w.Code, w.Body.String())
	}

	var sum1 interaction.ReactionSummary
	_ = json.Unmarshal(w.Body.Bytes(), &sum1)
	if sum1.Likes != 1 || sum1.Dislikes != 0 || sum1.MyReaction != "like" {
		t.Fatalf("unexpected summary after first like: %+v", sum1)
	}

	// 3. User 1 sends duplicate "like" -> idempotent, still likes == 1, dislikes == 0
	req = httptest.NewRequest(http.MethodPost, "/api/community/reactions", bytes.NewReader(likePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_1")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	_ = json.Unmarshal(w.Body.Bytes(), &sum1)
	if sum1.Likes != 1 || sum1.Dislikes != 0 {
		t.Fatalf("duplicate like must not increment count: %+v", sum1)
	}

	// 4. User 1 switches to "dislike" -> mutually exclusive: likes == 0, dislikes == 1
	dislikePayload, _ := json.Marshal(interaction.SetReactionRequest{
		TargetType: "post",
		TargetID:   p.ID,
		Value:      "dislike",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/community/reactions", bytes.NewReader(dislikePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_1")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var sum2 interaction.ReactionSummary
	_ = json.Unmarshal(w.Body.Bytes(), &sum2)
	if sum2.Likes != 0 || sum2.Dislikes != 1 || sum2.MyReaction != "dislike" {
		t.Fatalf("switching to dislike must result in likes=0, dislikes=1: %+v", sum2)
	}

	// 5. User 1 cancels reaction (value == "none") -> likes == 0, dislikes == 0
	cancelPayload, _ := json.Marshal(interaction.SetReactionRequest{
		TargetType: "post",
		TargetID:   p.ID,
		Value:      "none",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/community/reactions", bytes.NewReader(cancelPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_1")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var sum3 interaction.ReactionSummary
	_ = json.Unmarshal(w.Body.Bytes(), &sum3)
	if sum3.Likes != 0 || sum3.Dislikes != 0 || sum3.MyReaction != "none" {
		t.Fatalf("cancelling reaction must result in likes=0, dislikes=0: %+v", sum3)
	}
}

func TestPostOwnershipAndReplyDepth(t *testing.T) {
	postStore, _, _, _, router := setupTestCommunity()

	p, _ := postStore.CreatePost(context.Background(), posts.ScopeGlobal, "", "author_alice", "Alice's Post", "Summary", "Alice Body")

	// 1. Bob tries to update Alice's post -> rejected 403 Forbidden
	updatePayload, _ := json.Marshal(posts.UpdatePostRequest{
		ExpectedVersion: 1,
		Title:           "Bob's Malicious Edit",
		PublicSummary:   "Hacked Summary",
		Body:            "Hacked Body",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/community/posts/"+p.ID, bytes.NewReader(updatePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "author_bob")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-author post edit, got %d", w.Code)
	}

	// 2. Add first level comment
	c1, err := postStore.CreateComment(context.Background(), p.ID, "user_charlie", "First Level Comment", "")
	if err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}

	// 3. Add valid single-level reply referring to c1
	replyPayload, _ := json.Marshal(posts.CreateCommentRequest{
		Body:            "Reply to Charlie",
		ParentCommentID: c1.ID,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/community/posts/"+p.ID+"/comments", bytes.NewReader(replyPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_david")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for single-level reply, got %d: %s", w.Code, w.Body.String())
	}

	var c2 posts.CommentResponse
	_ = json.Unmarshal(w.Body.Bytes(), &c2)

	// 4. Try to reply to c2 (nested reply beyond single level) -> rejected 400
	deepReplyPayload, _ := json.Marshal(posts.CreateCommentRequest{
		Body:            "Deep Nested Reply (Not Allowed)",
		ParentCommentID: c2.ID,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/community/posts/"+p.ID+"/comments", bytes.NewReader(deepReplyPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user_eve")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 rejecting nested replies beyond single level, got %d: %s", w.Code, w.Body.String())
	}
}

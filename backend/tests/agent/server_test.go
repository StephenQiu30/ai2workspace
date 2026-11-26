package agent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/internal/agent"
	"github.com/StephenQiu30/ai2workspace/backend/internal/agent/adapter/http/assistant"
	knowledgeapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/knowledge"
	usageapp "github.com/StephenQiu30/ai2workspace/backend/internal/agent/application/usage"
	agentdomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/agent"
	knowledgedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/knowledge"
	"github.com/gin-gonic/gin"
)

func TestServer_TrustedAuthRBACAndCoursePermissions(t *testing.T) {
	sessionsMap := map[string]assistant.Actor{
		"tok_admin":      {UserID: "usr_admin", EmailVerified: true, Roles: []string{"admin"}, AccountStatus: "active"},
		"tok_instructor": {UserID: "usr_inst", EmailVerified: true, Roles: []string{"instructor"}, AccountStatus: "active"},
		"tok_student":    {UserID: "usr_stu", EmailVerified: true, Roles: []string{"student"}, AccountStatus: "active"},
		"tok_unverified": {UserID: "usr_unv", AccountStatus: "active"},
	}
	var accessMu sync.Mutex
	studentCourseAccess := true
	router := gin.New()
	checkCourse := func(_ context.Context, userID, courseID string) (bool, error) {
		accessMu.Lock()
		defer accessMu.Unlock()
		return studentCourseAccess && userID == "usr_stu" && courseID == "crs_go_101", nil
	}
	usageSvc := usageapp.NewService()
	knowledgeSvc := knowledgeapp.NewService(checkCourse, usageSvc)
	agent.NewWithDeps(agent.Config{AllowLocalSimulation: true}, agent.Deps{
		UsageService:     usageSvc,
		KnowledgeService: knowledgeSvc,
		SessionResolver: func(_ context.Context, token string) (*assistant.Actor, error) {
			actor, ok := sessionsMap[token]
			if !ok {
				return nil, assistant.ErrUnauthorized
			}
			return &actor, nil
		},
	}).RegisterRoutes(router)

	doReq := func(method, path, cookie string, body any, headers map[string]string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: "ai2_session", Value: cookie})
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	// 2. Spoofed X-User-ID without ai2_session MUST return 401; short cookie MUST NOT panic!
	if w := doReq(http.MethodGet, "/api/assistant/governance/summary", "", nil, map[string]string{"X-User-ID": "usr_admin"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when X-User-ID spoofed without cookie, got %d", w.Code)
	}
	if w := doReq(http.MethodGet, "/api/assistant/governance/summary", "ab", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for short invalid cookie without panic, got %d", w.Code)
	}

	// 3. R-28 RBAC: Instructor & Student MUST get 403 when creating/managing Agents or AI Governance!
	agentPayload := map[string]any{
		"name":            "Unauthorized Agent",
		"slug":            "unauth_bot",
		"roleInstruction": "prompt",
		"providerId":      "deepseek",
		"modelId":         "deepseek-chat",
	}
	if w := doReq(http.MethodPost, "/api/assistant/agents", "tok_student", agentPayload, nil); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for student creating agent, got %d", w.Code)
	}
	if w := doReq(http.MethodPost, "/api/assistant/agents", "tok_instructor", agentPayload, nil); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for instructor creating agent (R-28), got %d", w.Code)
	}
	if w := doReq(http.MethodPost, "/api/assistant/governance/pause", "tok_instructor", map[string]any{"paused": true}, nil); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin pausing AI, got %d", w.Code)
	}
	if w := doReq(http.MethodPatch, "/api/assistant/models/openrouter/anthropic/claude-3.5-haiku", "tok_instructor", map[string]any{"enabled": false}, nil); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin updating model, got %d", w.Code)
	}

	// Admin CAN update OpenRouter model containing slash in modelId!
	if w := doReq(http.MethodPatch, "/api/assistant/models/openrouter/anthropic/claude-3.5-haiku", "tok_admin", map[string]any{"priceVersion": "v2-2026-12"}, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin updating OpenRouter model, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Knowledge Base RBAC: Instructor creates private KB; Student cannot create or read private KB!
	wKB := doReq(http.MethodPost, "/api/assistant/knowledge-bases", "tok_instructor", map[string]any{
		"name":        "Go 付费课知识库",
		"description": "私有课程资料",
		"visibility":  "private",
	}, nil)
	if wKB.Code != http.StatusCreated {
		t.Fatalf("expected 201 for instructor creating KB, got %d: %s", wKB.Code, wKB.Body.String())
	}
	var kb knowledgedomain.KnowledgeBase
	_ = json.Unmarshal(wKB.Body.Bytes(), &kb)

	// Student sees 0 private KBs in list (no hardcoded isAdmin=true!)
	wListKB := doReq(http.MethodGet, "/api/assistant/knowledge-bases", "tok_student", nil, nil)
	var stuKBs []knowledgedomain.KnowledgeBase
	_ = json.Unmarshal(wListKB.Body.Bytes(), &stuKBs)
	if len(stuKBs) != 0 {
		t.Fatalf("expected student to see 0 private KBs, got %d", len(stuKBs))
	}

	if _, _, err := knowledgeSvc.SyncCourseKnowledge(
		t.Context(), kb.ID, "crs_go_101", "Go 调度器原理",
		"GMP 模型包含 Goroutine、Machine 和 Processor 三大核心组件。",
	); err != nil {
		t.Fatal(err)
	}

	// 6. Admin creates & publishes an Agent bound to this KB
	wAg := doReq(http.MethodPost, "/api/assistant/agents", "tok_admin", map[string]any{
		"name":             "Go 课程专家",
		"slug":             "go_expert",
		"roleInstruction":  "内部秘密提示词：结合课程资料解答 Go 原理。",
		"providerId":       "deepseek",
		"modelId":          "deepseek-chat",
		"knowledgeBaseIds": []string{kb.ID},
	}, nil)
	if wAg.Code != http.StatusCreated {
		t.Fatalf("expected 201 for admin creating agent, got %d: %s", wAg.Code, wAg.Body.String())
	}
	var ag agentdomain.Configuration
	_ = json.Unmarshal(wAg.Body.Bytes(), &ag)
	if w := doReq(http.MethodPost, "/api/assistant/agents/"+ag.ID+"/publish", "tok_admin", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 publishing agent, got %d: %s", w.Code, w.Body.String())
	}

	// Non-admin GET /agents hides RoleInstruction
	wStuAgents := doReq(http.MethodGet, "/api/assistant/agents", "tok_student", nil, nil)
	var stuAgents []agentdomain.Configuration
	_ = json.Unmarshal(wStuAgents.Body.Bytes(), &stuAgents)
	for _, a := range stuAgents {
		if a.RoleInstruction != "" {
			t.Fatalf("expected systemPrompt stripped for student, got %q", a.RoleInstruction)
		}
	}

	// 7. Unverified email user blocked from creating session (403)
	if w := doReq(http.MethodPost, "/api/assistant/sessions", "tok_unverified", map[string]any{"agentSlug": "go_expert"}, nil); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unverified email user, got %d", w.Code)
	}

	// 8. Verified student creates session, asks question (gets RAG + Web citations), then loses course access -> historical messages redacted!
	wSess := doReq(http.MethodPost, "/api/assistant/sessions", "tok_student", map[string]any{
		"agentSlug": "go_expert",
		"courseId":  "crs_go_101",
		"title":     "GMP 问题",
	}, nil)
	if wSess.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating session, got %d: %s", wSess.Code, wSess.Body.String())
	}
	var sess agentdomain.Session
	_ = json.Unmarshal(wSess.Body.Bytes(), &sess)

	wAsk := doReq(http.MethodPost, "/api/assistant/sessions/"+sess.ID+"/ask", "tok_student", map[string]any{
		"question":       "GMP 模型包含哪些组件？",
		"idempotencyKey": "http_idem_1",
	}, nil)
	if wAsk.Code != http.StatusOK {
		t.Fatalf("expected 200 asking question, got %d: %s", wAsk.Code, wAsk.Body.String())
	}
	var askMsg agentdomain.Message
	_ = json.Unmarshal(wAsk.Body.Bytes(), &askMsg)
	if len(askMsg.Citations) == 0 || askMsg.Citations[0].CourseID != "crs_go_101" {
		t.Fatalf("expected course citation from learning-authorized student, got %+v", askMsg.Citations)
	}

	// GET /sessions lists user's sessions
	wMySessions := doReq(http.MethodGet, "/api/assistant/sessions", "tok_student", nil, nil)
	var mySessions []agentdomain.Session
	_ = json.Unmarshal(wMySessions.Body.Bytes(), &mySessions)
	if len(mySessions) != 1 || mySessions[0].ID != sess.ID {
		t.Fatalf("expected 1 session in GET /sessions, got %+v", mySessions)
	}

	// Student refunds / loses course access in Learning service -> historical messages MUST be redacted!
	accessMu.Lock()
	studentCourseAccess = false
	accessMu.Unlock()

	wMsgs := doReq(http.MethodGet, "/api/assistant/sessions/"+sess.ID+"/messages", "tok_student", nil, nil)
	if wMsgs.Code != http.StatusOK {
		t.Fatalf("expected 200 listing messages, got %d: %s", wMsgs.Code, wMsgs.Body.String())
	}
	var msgs []agentdomain.Message
	_ = json.Unmarshal(wMsgs.Body.Bytes(), &msgs)
	if len(msgs) != 2 || !msgs[1].Redacted || !strings.Contains(msgs[1].Content, "受保护内容已隐藏") {
		t.Fatalf("expected redacted historical message after course access lost, got %+v", msgs)
	}

	// 9. Verify full HTTP lifecycle for KB update, source register/update/index/job/publish/revoke, Agent update/disable, Governance budget/pause/reconciliations
	if w := doReq(http.MethodPatch, "/api/assistant/knowledge-bases/"+kb.ID, "tok_instructor", map[string]any{
		"expectedVersion": 1,
		"name":            "Go 付费课知识库 v2",
	}, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 updating KB, got %d: %s", w.Code, w.Body.String())
	}

	wReg := doReq(http.MethodPost, "/api/assistant/knowledge-bases/"+kb.ID+"/sources", "tok_instructor", map[string]any{
		"kind":    "manual_doc",
		"title":   "FAQ 文档",
		"content": "初始 FAQ 内容",
	}, nil)
	if wReg.Code != http.StatusCreated {
		t.Fatalf("expected 201 registering source, got %d: %s", wReg.Code, wReg.Body.String())
	}
	var docSrc knowledgedomain.KnowledgeSource
	_ = json.Unmarshal(wReg.Body.Bytes(), &docSrc)

	wUpdSrc := doReq(http.MethodPatch, "/api/assistant/knowledge-bases/"+kb.ID+"/sources/"+docSrc.ID, "tok_instructor", map[string]any{
		"title":   "FAQ 文档 v2",
		"content": "更新后的 FAQ 内容",
	}, nil)
	if wUpdSrc.Code != http.StatusOK {
		t.Fatalf("expected 200 updating source, got %d: %s", wUpdSrc.Code, wUpdSrc.Body.String())
	}

	wJob := doReq(http.MethodPost, "/api/assistant/knowledge-sources/"+docSrc.ID+"/index-jobs", "tok_instructor", nil, nil)
	if wJob.Code != http.StatusOK {
		t.Fatalf("expected 200 starting index job, got %d: %s", wJob.Code, wJob.Body.String())
	}
	var job knowledgedomain.IndexJob
	_ = json.Unmarshal(wJob.Body.Bytes(), &job)

	if w := doReq(http.MethodGet, "/api/assistant/index-jobs/"+job.ID, "tok_instructor", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 getting index job, got %d: %s", w.Code, w.Body.String())
	}
	if w := doReq(http.MethodPost, "/api/assistant/knowledge-sources/"+docSrc.ID+"/publish", "tok_instructor", map[string]any{"expectedVersion": 2}, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 publishing source v2, got %d: %s", w.Code, w.Body.String())
	}
	if w := doReq(http.MethodPost, "/api/assistant/knowledge-sources/"+docSrc.ID+"/revoke", "tok_instructor", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 revoking source, got %d: %s", w.Code, w.Body.String())
	}
	// Republishing revoked source -> 409 Conflict
	if w := doReq(http.MethodPost, "/api/assistant/knowledge-sources/"+docSrc.ID+"/publish", "tok_instructor", map[string]any{"expectedVersion": 2}, nil); w.Code != http.StatusConflict {
		t.Fatalf("expected 409 republishing revoked source, got %d: %s", w.Code, w.Body.String())
	}

	// Admin updates & disables agent
	if w := doReq(http.MethodPatch, "/api/assistant/agents/"+ag.ID, "tok_admin", map[string]any{
		"expectedVersion": 1,
		"name":            "Go 课程专家 v2",
	}, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 updating agent, got %d: %s", w.Code, w.Body.String())
	}
	if w := doReq(http.MethodPost, "/api/assistant/agents/"+ag.ID+"/disable", "tok_admin", map[string]any{"reason": "offline"}, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 disabling agent, got %d: %s", w.Code, w.Body.String())
	}

	// Admin governance endpoints: budget update, pause/resume, reconciliations list
	if w := doReq(http.MethodPatch, "/api/assistant/governance/budget", "tok_admin", map[string]any{"alertThresholdPct": 75}, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 updating budget, got %d: %s", w.Code, w.Body.String())
	}
	if w := doReq(http.MethodPost, "/api/assistant/governance/pause", "tok_admin", map[string]any{"paused": true, "reason": "check"}, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 pausing AI, got %d: %s", w.Code, w.Body.String())
	}
	if w := doReq(http.MethodGet, "/api/assistant/governance/reconciliations", "tok_admin", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 listing reconciliations, got %d: %s", w.Code, w.Body.String())
	}
	if w := doReq(http.MethodPost, "/api/assistant/sessions/"+sess.ID+"/runs/"+askMsg.RunID+"/cancel", "tok_student", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 cancelling completed run idempotently, got %d: %s", w.Code, w.Body.String())
	}
}

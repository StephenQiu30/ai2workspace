package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/agent"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/config"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/knowledge"
	"github.com/StephenQiu30/ai2workspace/backend/services/assistant/internal/server"
)

func TestServer_TrustedAuthRBACAndCrossServiceContracts(t *testing.T) {
	// Mock Identity service: POST /internal/identity/sessions/resolve
	sessionsMap := map[string]map[string]any{
		"tok_admin": {
			"userId":        "usr_admin",
			"email":         "admin@learnis.test",
			"emailVerified": true,
			"roles":         []string{"admin"},
			"accountStatus": "active",
		},
		"tok_instructor": {
			"userId":        "usr_inst",
			"email":         "inst@learnis.test",
			"emailVerified": true,
			"roles":         []string{"instructor"},
			"accountStatus": "active",
		},
		"tok_student": {
			"userId":        "usr_stu",
			"email":         "stu@learnis.test",
			"emailVerified": true,
			"roles":         []string{"student"},
			"accountStatus": "active",
		},
		"tok_unverified": {
			"userId":        "usr_unv",
			"email":         "unv@learnis.test",
			"emailVerified": false,
			"roles":         []string{"student"},
			"accountStatus": "active",
		},
	}

	const svcToken = "test-service-secret"
	identitySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/identity/sessions/resolve" || r.Header.Get("X-Service-Token") != svcToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			OpaqueSession string `json:"opaqueSession"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		actor, ok := sessionsMap[req.OpaqueSession]
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(actor)
	}))
	defer identitySrv.Close()

	// Mock Learning service: POST /internal/learning/course-access/check
	var accessMu sync.Mutex
	studentCourseAccess := true
	learningSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/learning/course-access/check" || r.Header.Get("X-Service-Token") != svcToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			UserID   string `json:"userId"`
			CourseID string `json:"courseId"`
			Purpose  string `json:"purpose"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		accessMu.Lock()
		allowed := studentCourseAccess && req.UserID == "usr_stu" && req.CourseID == "crs_go_101" && req.Purpose == "knowledge"
		accessMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"allowed":        allowed,
			"reason":         "checked",
			"accessVersion":  1,
			"contentVersion": 1,
		})
	}))
	defer learningSrv.Close()

	cfg := &config.Config{
		HTTPAddr:             ":3127",
		SwaggerEnabled:       false,
		ServiceToken:         svcToken,
		IdentityServiceURL:   identitySrv.URL,
		LearningServiceURL:   learningSrv.URL,
		AllowLocalSimulation: true,
	}
	srv := server.New(cfg)
	router := srv.Router()

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

	// 1. Health endpoints public
	if w := doReq(http.MethodGet, "/healthz", "", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 on /healthz, got %d", w.Code)
	}
	if w := doReq(http.MethodGet, "/api/assistant/health", "", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/assistant/health, got %d", w.Code)
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
	var kb knowledge.KnowledgeBase
	_ = json.Unmarshal(wKB.Body.Bytes(), &kb)

	// Student sees 0 private KBs in list (no hardcoded isAdmin=true!)
	wListKB := doReq(http.MethodGet, "/api/assistant/knowledge-bases", "tok_student", nil, nil)
	var stuKBs []knowledge.KnowledgeBase
	_ = json.Unmarshal(wListKB.Body.Bytes(), &stuKBs)
	if len(stuKBs) != 0 {
		t.Fatalf("expected student to see 0 private KBs, got %d", len(stuKBs))
	}

	// 5. Internal course sync: requires valid X-Service-Token
	syncPayload := map[string]any{
		"courseId":        "crs_go_101",
		"knowledgeBaseId": kb.ID,
		"title":           "Go 调度器原理",
		"content":         "GMP 模型包含 Goroutine、Machine 和 Processor 三大核心组件。",
	}
	if w := doReq(http.MethodPost, "/internal/assistant/knowledge/sync-course", "", syncPayload, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without X-Service-Token, got %d", w.Code)
	}
	if w := doReq(http.MethodPost, "/internal/assistant/knowledge/sync-course", "", syncPayload, map[string]string{"X-Service-Token": svcToken}); w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid X-Service-Token, got %d: %s", w.Code, w.Body.String())
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
	var ag agent.Configuration
	_ = json.Unmarshal(wAg.Body.Bytes(), &ag)
	if w := doReq(http.MethodPost, "/api/assistant/agents/"+ag.ID+"/publish", "tok_admin", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 publishing agent, got %d: %s", w.Code, w.Body.String())
	}

	// Non-admin GET /agents hides RoleInstruction
	wStuAgents := doReq(http.MethodGet, "/api/assistant/agents", "tok_student", nil, nil)
	var stuAgents []agent.Configuration
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
	var sess agent.Session
	_ = json.Unmarshal(wSess.Body.Bytes(), &sess)

	wAsk := doReq(http.MethodPost, "/api/assistant/sessions/"+sess.ID+"/ask", "tok_student", map[string]any{
		"question":       "GMP 模型包含哪些组件？",
		"idempotencyKey": "http_idem_1",
	}, nil)
	if wAsk.Code != http.StatusOK {
		t.Fatalf("expected 200 asking question, got %d: %s", wAsk.Code, wAsk.Body.String())
	}
	var askMsg agent.Message
	_ = json.Unmarshal(wAsk.Body.Bytes(), &askMsg)
	if len(askMsg.Citations) == 0 || askMsg.Citations[0].CourseID != "crs_go_101" {
		t.Fatalf("expected course citation from learning-authorized student, got %+v", askMsg.Citations)
	}

	// GET /sessions lists user's sessions
	wMySessions := doReq(http.MethodGet, "/api/assistant/sessions", "tok_student", nil, nil)
	var mySessions []agent.Session
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
	var msgs []agent.Message
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
	var docSrc knowledge.KnowledgeSource
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
	var job knowledge.IndexJob
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

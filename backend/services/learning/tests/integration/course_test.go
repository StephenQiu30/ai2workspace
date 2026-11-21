package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/courses"
	"github.com/StephenQiu30/ai2workspace/backend/services/learning/internal/progress"
)

func TestCourseOwnershipReviewAndStateTransitions(t *testing.T) {
	env := newTestEnv()

	// 1. Unauthenticated and student cannot create a course
	w := doRequest(t, env.router, http.MethodPost, "/api/learning/courses", courses.CreateCourseRequest{Title: "Unauthorized"}, "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("guest create course status = %d, want 401", w.Code)
	}
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses", courses.CreateCourseRequest{Title: "Student Attempt"}, sessStudent1, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("student create course status = %d, want 403", w.Code)
	}

	// 2. Instructor creates a draft course
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses", courses.CreateCourseRequest{
		Title:       "Next.js Fullstack Course",
		Description: "Comprehensive Next.js and TypeScript guide",
		CoverURL:    "https://example.com/cover.png",
		Category:    "engineering",
		IsFree:      false,
	}, sessInst1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("create course status = %d: %s", w.Code, w.Body.String())
	}
	var created courses.CourseMutationResponse
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	courseID := created.CourseID
	if courseID == "" || created.State != "draft" || created.Version != 1 {
		t.Fatalf("unexpected create result: %+v", created)
	}

	// 3. Draft is hidden from public list AND public detail (404 for guest, student, other instructor)
	listW := doRequest(t, env.router, http.MethodGet, "/api/learning/courses", nil, "", nil)
	var listRes courses.CourseListResponse
	_ = json.Unmarshal(listW.Body.Bytes(), &listRes)
	if listRes.Total != 0 {
		t.Fatalf("expected 0 public courses, got %d", listRes.Total)
	}
	for _, tok := range []string{"", sessStudent1, sessInst2} {
		detW := doRequest(t, env.router, http.MethodGet, "/api/learning/courses/"+courseID, nil, tok, nil)
		if detW.Code != http.StatusNotFound {
			t.Fatalf("draft detail for token %q status = %d, want 404", tok, detW.Code)
		}
	}
	// Owner and admin CAN inspect the draft
	for _, tok := range []string{sessInst1, sessAdmin} {
		detW := doRequest(t, env.router, http.MethodGet, "/api/learning/courses/"+courseID, nil, tok, nil)
		if detW.Code != http.StatusOK {
			t.Fatalf("draft detail for owner/admin %q status = %d, want 200", tok, detW.Code)
		}
		var detail courses.CourseResponse
		_ = json.Unmarshal(detW.Body.Bytes(), &detail)
		if detail.AuthorNickname != "林讲师" || detail.Category != "engineering" {
			t.Fatalf("unexpected draft metadata: %+v", detail)
		}
	}

	// 4. Another instructor cannot update or submit
	w = doRequest(t, env.router, http.MethodPatch, "/api/learning/courses/"+courseID, courses.UpdateCourseRequest{
		Title:           "Hijack",
		ExpectedVersion: created.Version,
	}, sessInst2, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("other instructor update status = %d, want 403", w.Code)
	}

	// 5. Admin cannot review before submission (state must be pending_review)
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+courseID+"/reviews", courses.ReviewCourseRequest{
		Decision:        "approved",
		Reason:          "too early",
		ExpectedVersion: created.Version,
	}, sessAdmin, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("reviewing draft status = %d, want 409", w.Code)
	}

	// 6. Owner submits for review
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+courseID+"/submissions", courses.SubmitCourseRequest{
		ExpectedVersion: created.Version,
	}, sessInst1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("submit status = %d: %s", w.Code, w.Body.String())
	}
	var submitted courses.CourseMutationResponse
	_ = json.Unmarshal(w.Body.Bytes(), &submitted)
	if submitted.State != "pending_review" {
		t.Fatalf("expected pending_review, got %s", submitted.State)
	}

	// 7. Non-admin cannot review, close sales, or withdraw
	for _, tok := range []string{sessInst1, sessStudent1} {
		revW := doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+courseID+"/reviews", courses.ReviewCourseRequest{
			Decision:        "approved",
			ExpectedVersion: submitted.Version,
		}, tok, nil)
		if revW.Code != http.StatusForbidden {
			t.Fatalf("non-admin review (%q) status = %d, want 403", tok, revW.Code)
		}
		closeW := doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+courseID+"/sales-closure", courses.CloseSalesRequest{
			ExpectedVersion: submitted.Version,
		}, tok, nil)
		if closeW.Code != http.StatusForbidden {
			t.Fatalf("non-admin close-sales (%q) status = %d, want 403", tok, closeW.Code)
		}
		wdW := doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+courseID+"/withdrawals", courses.WithdrawCourseRequest{
			Reason:          "attempt",
			ExpectedVersion: submitted.Version,
		}, tok, nil)
		if wdW.Code != http.StatusForbidden {
			t.Fatalf("non-admin withdraw (%q) status = %d, want 403", tok, wdW.Code)
		}
	}

	// 8. Admin rejects with reason -> reviewReason saved, still hidden from public
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+courseID+"/reviews", courses.ReviewCourseRequest{
		Decision:        "rejected",
		Reason:          "请补充章节大纲与试看课时",
		ExpectedVersion: submitted.Version,
	}, sessAdmin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("reject status = %d: %s", w.Code, w.Body.String())
	}
	var rejected courses.CourseMutationResponse
	_ = json.Unmarshal(w.Body.Bytes(), &rejected)
	if rejected.State != "rejected" {
		t.Fatalf("expected rejected, got %s", rejected.State)
	}
	ownerDet := doRequest(t, env.router, http.MethodGet, "/api/learning/courses/"+courseID, nil, sessInst1, nil)
	var rejDetail courses.CourseResponse
	_ = json.Unmarshal(ownerDet.Body.Bytes(), &rejDetail)
	if rejDetail.ReviewReason != "请补充章节大纲与试看课时" {
		t.Fatalf("expected reviewReason saved, got %q", rejDetail.ReviewReason)
	}

	// 9. Owner re-submits from rejected -> admin approves -> public catalog lists it
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+courseID+"/submissions", courses.SubmitCourseRequest{
		ExpectedVersion: rejected.Version,
	}, sessInst1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("resubmit status = %d: %s", w.Code, w.Body.String())
	}
	var resub courses.CourseMutationResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resub)

	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+courseID+"/reviews", courses.ReviewCourseRequest{
		Decision:        "approved",
		Reason:          "审核通过",
		ExpectedVersion: resub.Version,
	}, sessAdmin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("approve status = %d: %s", w.Code, w.Body.String())
	}
	var approved courses.CourseMutationResponse
	_ = json.Unmarshal(w.Body.Bytes(), &approved)
	if approved.State != "approved" {
		t.Fatalf("expected approved, got %s", approved.State)
	}

	// Submitting an already approved course returns 409
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+courseID+"/submissions", courses.SubmitCourseRequest{
		ExpectedVersion: approved.Version,
	}, sessInst1, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("submitting approved course status = %d, want 409", w.Code)
	}

	// Optimistic concurrency conflict on stale version
	w = doRequest(t, env.router, http.MethodPatch, "/api/learning/courses/"+courseID, courses.UpdateCourseRequest{
		Title:           "Stale",
		ExpectedVersion: 1,
	}, sessInst1, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("stale update status = %d, want 409", w.Code)
	}

	// Published course cannot be deleted (409)
	w = doRequest(t, env.router, http.MethodDelete, fmt.Sprintf("/api/learning/courses/%s?expectedVersion=%d", courseID, approved.Version), nil, sessInst1, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("delete published course status = %d, want 409", w.Code)
	}

	// Unpublished draft course CAN be deleted by owner (204), but not by another instructor (403)
	draftW := doRequest(t, env.router, http.MethodPost, "/api/learning/courses", courses.CreateCourseRequest{
		Title: "Temp Draft",
	}, sessInst1, nil)
	var draftRes courses.CourseMutationResponse
	_ = json.Unmarshal(draftW.Body.Bytes(), &draftRes)

	if w := doRequest(t, env.router, http.MethodDelete, fmt.Sprintf("/api/learning/courses/%s?expectedVersion=%d", draftRes.CourseID, draftRes.Version), nil, sessInst2, nil); w.Code != http.StatusForbidden {
		t.Fatalf("other instructor delete draft status = %d, want 403", w.Code)
	}
	if w := doRequest(t, env.router, http.MethodDelete, fmt.Sprintf("/api/learning/courses/%s?expectedVersion=%d", draftRes.CourseID, draftRes.Version), nil, sessInst1, nil); w.Code != http.StatusNoContent {
		t.Fatalf("owner delete draft status = %d, want 204", w.Code)
	}
}

func TestPublicCourseDetailSanitizesProtectedArticleContent(t *testing.T) {
	env := newTestEnv()

	c, _ := env.coursesStore.CreateCourse(t.Context(), "instructor_1", "林讲师", "Go 进阶专栏", "付费图文课程", "", "engineering", false)
	sec, _ := env.coursesStore.AddSection(t.Context(), c.ID, "第一章", 1)
	_, _ = env.coursesStore.AddLesson(t.Context(), c.ID, sec.ID, "试看导读", "article", true, "公开试看正文", "", 1)
	_, _ = env.coursesStore.AddLesson(t.Context(), c.ID, sec.ID, "核心源码剖析", "article", false, "仅已购可见的机密正文", "", 2)
	cur, _ := env.coursesStore.GetCourse(t.Context(), c.ID)
	sub, _ := env.coursesStore.SubmitCourse(t.Context(), c.ID, "instructor_1", false, cur.Version)
	app, _ := env.coursesStore.ReviewCourse(t.Context(), c.ID, "approved", "ok", sub.Version)

	checkContent := func(token string, wantPaidContent string) {
		t.Helper()
		w := doRequest(t, env.router, http.MethodGet, "/api/learning/courses/"+c.ID, nil, token, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("get course (%q) status = %d: %s", token, w.Code, w.Body.String())
		}
		var resp courses.CourseResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.LessonCount != 2 || len(resp.Sections) != 1 || len(resp.Sections[0].Lessons) != 2 {
			t.Fatalf("unexpected course structure: %+v", resp)
		}
		if resp.Sections[0].Lessons[0].Content != "公开试看正文" {
			t.Fatalf("trial content lost for %q: %q", token, resp.Sections[0].Lessons[0].Content)
		}
		if resp.Sections[0].Lessons[1].Content != wantPaidContent {
			t.Fatalf("paid content for %q = %q, want %q", token, resp.Sections[0].Lessons[1].Content, wantPaidContent)
		}
	}

	// Guest and unpurchased student MUST NOT receive non-trial article content
	checkContent("", "")
	checkContent(sessStudent1, "")

	// Public listing must also never leak non-trial content
	listW := doRequest(t, env.router, http.MethodGet, "/api/learning/courses", nil, "", nil)
	var listRes courses.CourseListResponse
	_ = json.Unmarshal(listW.Body.Bytes(), &listRes)
	if len(listRes.Courses) != 1 || listRes.Courses[0].Sections[0].Lessons[1].Content != "" {
		t.Fatalf("public list leaked paid content: %+v", listRes.Courses)
	}

	// Owner and admin receive full content
	checkContent(sessInst1, "仅已购可见的机密正文")
	checkContent(sessAdmin, "仅已购可见的机密正文")

	// Purchased student receives full content
	env.entitlements.Grant("student_1", c.ID)
	checkContent(sessStudent1, "仅已购可见的机密正文")

	// After refund (entitlement revoked), student loses paid content
	env.entitlements.Revoke("student_1", c.ID)
	checkContent(sessStudent1, "")

	// After admin withdraws course, guest & student (even if entitled) get 404; owner still gets 200
	env.entitlements.Grant("student_1", c.ID)
	wdW := doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+c.ID+"/withdrawals", courses.WithdrawCourseRequest{
		Reason:          "合规撤回",
		ExpectedVersion: app.Version,
	}, sessAdmin, nil)
	if wdW.Code != http.StatusOK {
		t.Fatalf("withdraw status = %d", wdW.Code)
	}
	for _, tok := range []string{"", sessStudent1} {
		w := doRequest(t, env.router, http.MethodGet, "/api/learning/courses/"+c.ID, nil, tok, nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("withdrawn course for %q status = %d, want 404", tok, w.Code)
		}
	}
	wOwner := doRequest(t, env.router, http.MethodGet, "/api/learning/courses/"+c.ID, nil, sessInst1, nil)
	if wOwner.Code != http.StatusOK {
		t.Fatalf("withdrawn course for owner status = %d, want 200", wOwner.Code)
	}
}

func TestSearchCategoryPaginationAndManageList(t *testing.T) {
	env := newTestEnv()

	publish := func(owner, nick, title, desc, cat string) *courses.Course {
		c, _ := env.coursesStore.CreateCourse(t.Context(), owner, nick, title, desc, "", cat, false)
		sub, _ := env.coursesStore.SubmitCourse(t.Context(), c.ID, owner, false, c.Version)
		app, _ := env.coursesStore.ReviewCourse(t.Context(), c.ID, "approved", "ok", sub.Version)
		time.Sleep(2 * time.Millisecond)
		return app
	}

	c1 := publish("instructor_1", "林讲师", "Go 工程实践", "后端架构课程", "engineering")
	c2 := publish("instructor_1", "林讲师", "Agent 应用开发", "AI 实战指南", "ai")
	c3 := publish("instructor_2", "周讲师", "设计系统精讲", "UI 与无障碍", "design")
	// Withdraw c3 so it must be excluded from public catalog
	_, _ = env.coursesStore.WithdrawCourse(t.Context(), c3.ID, "admin_1", "撤回", c3.Version)
	// Draft by instructor_2
	_, _ = env.coursesStore.CreateCourse(t.Context(), "instructor_2", "周讲师", "未发布草稿", "", "", "product", true)

	// 1. Public list excludes draft & withdrawn, sorted UpdatedAt DESC (c2 before c1)
	w := doRequest(t, env.router, http.MethodGet, "/api/learning/courses", nil, "", nil)
	var allPub courses.CourseListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &allPub)
	if allPub.Total != 2 || len(allPub.Courses) != 2 || allPub.Courses[0].ID != c2.ID || allPub.Courses[1].ID != c1.ID {
		t.Fatalf("unexpected public list ordering/filtering: %+v", allPub)
	}

	// 2. Keyword search q
	w = doRequest(t, env.router, http.MethodGet, "/api/learning/courses?q=实战", nil, "", nil)
	var searchRes courses.CourseListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &searchRes)
	if searchRes.Total != 1 || searchRes.Courses[0].ID != c2.ID {
		t.Fatalf("unexpected search result: %+v", searchRes)
	}

	// 3. Category filter
	w = doRequest(t, env.router, http.MethodGet, "/api/learning/courses?category=engineering", nil, "", nil)
	var catRes courses.CourseListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &catRes)
	if catRes.Total != 1 || catRes.Courses[0].ID != c1.ID {
		t.Fatalf("unexpected category result: %+v", catRes)
	}

	// 4. Pagination page/pageSize
	w = doRequest(t, env.router, http.MethodGet, "/api/learning/courses?page=2&pageSize=1", nil, "", nil)
	var pageRes courses.CourseListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &pageRes)
	if pageRes.Total != 2 || len(pageRes.Courses) != 1 || pageRes.Courses[0].ID != c1.ID || pageRes.Page != 2 || pageRes.PageSize != 1 {
		t.Fatalf("unexpected pagination result: %+v", pageRes)
	}

	// 5. Manage courses list: student forbidden, instructor sees only own, admin sees all + state filter
	if w := doRequest(t, env.router, http.MethodGet, "/api/learning/manage/courses", nil, sessStudent1, nil); w.Code != http.StatusForbidden {
		t.Fatalf("student manage list status = %d, want 403", w.Code)
	}
	w = doRequest(t, env.router, http.MethodGet, "/api/learning/manage/courses", nil, sessInst1, nil)
	var inst1List courses.CourseListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &inst1List)
	if inst1List.Total != 2 {
		t.Fatalf("instructor_1 manage total = %d, want 2", inst1List.Total)
	}

	w = doRequest(t, env.router, http.MethodGet, "/api/learning/manage/courses", nil, sessAdmin, nil)
	var adminList courses.CourseListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &adminList)
	if adminList.Total != 4 {
		t.Fatalf("admin manage total = %d, want 4", adminList.Total)
	}

	w = doRequest(t, env.router, http.MethodGet, "/api/learning/manage/courses?state=withdrawn", nil, sessAdmin, nil)
	var wdList courses.CourseListResponse
	_ = json.Unmarshal(w.Body.Bytes(), &wdList)
	if wdList.Total != 1 || wdList.Courses[0].ID != c3.ID {
		t.Fatalf("admin withdrawn filter unexpected: %+v", wdList)
	}
}

func TestSectionAndLessonCRUD(t *testing.T) {
	env := newTestEnv()
	c, _ := env.coursesStore.CreateCourse(t.Context(), "instructor_1", "林讲师", "目录编排测试", "", "", "engineering", true)

	// Non-owner cannot add section
	w := doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+c.ID+"/sections", courses.AddSectionRequest{
		Title:     "入侵章节",
		SortOrder: 1,
	}, sessInst2, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner add section status = %d, want 403", w.Code)
	}

	// Owner adds two sections
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+c.ID+"/sections", courses.AddSectionRequest{Title: "第一章", SortOrder: 1}, sessInst1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("add section1 status = %d", w.Code)
	}
	var sec1 courses.Section
	_ = json.Unmarshal(w.Body.Bytes(), &sec1)

	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+c.ID+"/sections", courses.AddSectionRequest{Title: "第二章", SortOrder: 2}, sessInst1, nil)
	var sec2 courses.Section
	_ = json.Unmarshal(w.Body.Bytes(), &sec2)

	// Update section 1
	newSecTitle := "第一章：基础准备"
	newOrder := 0
	w = doRequest(t, env.router, http.MethodPatch, fmt.Sprintf("/api/learning/courses/%s/sections/%s", c.ID, sec1.ID), courses.UpdateSectionRequest{
		Title:     &newSecTitle,
		SortOrder: &newOrder,
	}, sessInst1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("update section status = %d: %s", w.Code, w.Body.String())
	}
	var updatedSec courses.Section
	_ = json.Unmarshal(w.Body.Bytes(), &updatedSec)
	if updatedSec.Title != newSecTitle || updatedSec.SortOrder != 0 {
		t.Fatalf("unexpected updated section: %+v", updatedSec)
	}

	// Add lesson to sec1
	w = doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+c.ID+"/lessons", courses.AddLessonRequest{
		SectionID: sec1.ID,
		Title:     "1.1 入门",
		Type:      "article",
		IsTrial:   false,
		Content:   "初始正文",
		SortOrder: 1,
	}, sessInst1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("add lesson status = %d: %s", w.Code, w.Body.String())
	}
	var lsn courses.Lesson
	_ = json.Unmarshal(w.Body.Bytes(), &lsn)

	// Update lesson: move to sec2, toggle trial, update content
	newLsnTitle := "2.1 进阶入门"
	isTrial := true
	newContent := "修订后的正文"
	w = doRequest(t, env.router, http.MethodPatch, fmt.Sprintf("/api/learning/courses/%s/lessons/%s", c.ID, lsn.ID), courses.UpdateLessonRequest{
		SectionID: &sec2.ID,
		Title:     &newLsnTitle,
		IsTrial:   &isTrial,
		Content:   &newContent,
	}, sessInst1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("update lesson status = %d: %s", w.Code, w.Body.String())
	}
	var updatedLsn courses.Lesson
	_ = json.Unmarshal(w.Body.Bytes(), &updatedLsn)
	if updatedLsn.SectionID != sec2.ID || !updatedLsn.IsTrial || updatedLsn.Content != newContent {
		t.Fatalf("unexpected updated lesson: %+v", updatedLsn)
	}

	// Delete lesson
	w = doRequest(t, env.router, http.MethodDelete, fmt.Sprintf("/api/learning/courses/%s/lessons/%s", c.ID, lsn.ID), nil, sessInst1, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete lesson status = %d", w.Code)
	}

	// Delete section 2
	w = doRequest(t, env.router, http.MethodDelete, fmt.Sprintf("/api/learning/courses/%s/sections/%s", c.ID, sec2.ID), nil, sessInst1, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete section status = %d", w.Code)
	}

	// Verify final course structure
	finalCourse, _ := env.coursesStore.GetCourse(t.Context(), c.ID)
	if len(finalCourse.Sections) != 1 || finalCourse.LessonCount != 0 {
		t.Fatalf("unexpected final course sections/lessons: %+v", finalCourse)
	}
}

func TestFreeEnrollmentMyCoursesAndProgressSummary(t *testing.T) {
	env := newTestEnv()

	freeC, _ := env.coursesStore.CreateCourse(t.Context(), "instructor_1", "林讲师", "免费公开课", "", "", "general", true)
	sec, _ := env.coursesStore.AddSection(t.Context(), freeC.ID, "第一章", 1)
	l1, _ := env.coursesStore.AddLesson(t.Context(), freeC.ID, sec.ID, "图文节", "article", true, "阅读内容", "", 1)
	_, _ = env.coursesStore.AddLesson(t.Context(), freeC.ID, sec.ID, "视频节", "video", false, "", "", 2)
	curFree, _ := env.coursesStore.GetCourse(t.Context(), freeC.ID)
	subFree, _ := env.coursesStore.SubmitCourse(t.Context(), freeC.ID, "instructor_1", false, curFree.Version)
	_, _ = env.coursesStore.ReviewCourse(t.Context(), freeC.ID, "approved", "ok", subFree.Version)

	paidC, _ := env.coursesStore.CreateCourse(t.Context(), "instructor_1", "林讲师", "付费实战课", "", "", "ai", false)
	subPaid, _ := env.coursesStore.SubmitCourse(t.Context(), paidC.ID, "instructor_1", false, paidC.Version)
	_, _ = env.coursesStore.ReviewCourse(t.Context(), paidC.ID, "approved", "ok", subPaid.Version)

	// 1. Enroll free succeeds; duplicate returns 409; paid returns 400
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+freeC.ID+"/enrollments", nil, sessStudent1, nil); w.Code != http.StatusOK {
		t.Fatalf("free enroll status = %d: %s", w.Code, w.Body.String())
	}
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+freeC.ID+"/enrollments", nil, sessStudent1, nil); w.Code != http.StatusConflict {
		t.Fatalf("duplicate enroll status = %d, want 409", w.Code)
	}
	if w := doRequest(t, env.router, http.MethodPost, "/api/learning/courses/"+paidC.ID+"/enrollments", nil, sessStudent1, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("paid direct enroll status = %d, want 400", w.Code)
	}

	// 2. Grant paid entitlement & save progress on free course lesson
	env.entitlements.Grant("student_1", paidC.ID)
	w := doRequest(t, env.router, http.MethodPut, "/api/learning/lessons/"+l1.ID+"/progress", progress.SaveProgressRequest{
		CourseID:        freeC.ID,
		ContentVersion:  1,
		PositionSeconds: 42,
		ExpectedVersion: 0,
	}, sessStudent1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("save progress status = %d: %s", w.Code, w.Body.String())
	}

	// 3. GET /api/learning/me/courses merges both free & purchased with progress summary
	myW := doRequest(t, env.router, http.MethodGet, "/api/learning/me/courses", nil, sessStudent1, nil)
	if myW.Code != http.StatusOK {
		t.Fatalf("me/courses status = %d: %s", myW.Code, myW.Body.String())
	}
	var myCourses []courses.MyCourseResponse
	_ = json.Unmarshal(myW.Body.Bytes(), &myCourses)
	if len(myCourses) != 2 {
		t.Fatalf("expected 2 merged courses, got %d: %+v", len(myCourses), myCourses)
	}
	// Recently studied course comes first
	if myCourses[0].ID != freeC.ID || myCourses[0].EnrollmentType != "free" {
		t.Fatalf("expected recently studied free course first: %+v", myCourses[0])
	}
	if myCourses[0].ProgressSummary.LastLessonID != l1.ID ||
		myCourses[0].ProgressSummary.WatchedLessonCount != 1 ||
		myCourses[0].ProgressSummary.CompletedLessonCount != 1 ||
		myCourses[0].ProgressSummary.TotalLessonCount != 2 ||
		myCourses[0].ProgressSummary.LastStudiedAt == nil {
		t.Fatalf("unexpected progress summary: %+v", myCourses[0].ProgressSummary)
	}
	if myCourses[1].ID != paidC.ID || myCourses[1].EnrollmentType != "purchased" {
		t.Fatalf("expected purchased course second: %+v", myCourses[1])
	}
}

func TestInternalEndpoints(t *testing.T) {
	env := newTestEnv()

	c, _ := env.coursesStore.CreateCourse(t.Context(), "instructor_1", "林讲师", "内部契约测试课", "", "", "engineering", false)
	sub, _ := env.coursesStore.SubmitCourse(t.Context(), c.ID, "instructor_1", false, c.Version)
	app, _ := env.coursesStore.ReviewCourse(t.Context(), c.ID, "approved", "ok", sub.Version)

	svcHeaders := map[string]string{"X-Service-Token": testServiceToken}

	// 1. Missing token -> 401
	if w := doRequest(t, env.router, http.MethodGet, "/internal/learning/courses/"+c.ID+"/sale-snapshot", nil, "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing service token status = %d, want 401", w.Code)
	}

	// 2. Sale snapshot returns isFree and deliverable without querying commerce
	w := doRequest(t, env.router, http.MethodGet, "/internal/learning/courses/"+c.ID+"/sale-snapshot", nil, "", svcHeaders)
	if w.Code != http.StatusOK {
		t.Fatalf("sale snapshot status = %d", w.Code)
	}
	var snap courses.SaleSnapshotResponse
	_ = json.Unmarshal(w.Body.Bytes(), &snap)
	if snap.CourseID != c.ID || snap.IsFree || !snap.Deliverable {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}

	// 3. CheckCourseAccess denies unpurchased student, grants purchased student, denies after withdraw
	check := func(userID string, roles []string) courses.CheckCourseAccessResponse {
		t.Helper()
		respW := doRequest(t, env.router, http.MethodPost, "/internal/learning/course-access/check", courses.CheckCourseAccessRequest{
			UserID:   userID,
			CourseID: c.ID,
			Purpose:  "discussion",
			Roles:    roles,
		}, "", svcHeaders)
		if respW.Code != http.StatusOK {
			t.Fatalf("course-access/check status = %d: %s", respW.Code, respW.Body.String())
		}
		var out courses.CheckCourseAccessResponse
		_ = json.Unmarshal(respW.Body.Bytes(), &out)
		return out
	}

	if res := check("student_1", nil); res.Allowed {
		t.Fatalf("unpurchased student should be denied, got %+v", res)
	}
	env.entitlements.Grant("student_1", c.ID)
	if res := check("student_1", nil); !res.Allowed {
		t.Fatalf("purchased student should be allowed, got %+v", res)
	}
	if res := check("instructor_1", nil); !res.Allowed {
		t.Fatalf("owner should be allowed, got %+v", res)
	}
	if res := check("someone_admin", []string{"admin"}); !res.Allowed {
		t.Fatalf("admin should be allowed, got %+v", res)
	}

	// Withdraw denies everyone including purchased student
	_, _ = env.coursesStore.WithdrawCourse(t.Context(), c.ID, "admin_1", "下线", app.Version)
	if res := check("student_1", nil); res.Allowed || res.Reason != "course_withdrawn" {
		t.Fatalf("withdrawn course must deny access, got %+v", res)
	}

	// 4. Delivery evidence keeps stable timestamp
	evW1 := doRequest(t, env.router, http.MethodGet, "/internal/learning/courses/"+c.ID+"/delivery-evidence", nil, "", svcHeaders)
	evW2 := doRequest(t, env.router, http.MethodGet, "/internal/learning/courses/"+c.ID+"/delivery-evidence", nil, "", svcHeaders)
	var ev1, ev2 courses.DeliveryEvidenceResponse
	_ = json.Unmarshal(evW1.Body.Bytes(), &ev1)
	_ = json.Unmarshal(evW2.Body.Bytes(), &ev2)
	if ev1.FirstDeliverableAt == nil || !ev1.FirstDeliverableAt.Equal(*ev2.FirstDeliverableAt) {
		t.Fatalf("delivery evidence mismatch: %+v vs %+v", ev1, ev2)
	}
}

// Package courses defines request DTOs.
package courses

// CreateCourseRequest defines parameters for initiating a draft course.
type CreateCourseRequest struct {
	Title       string `json:"title" binding:"required" example:"深入浅出 Next.js"`
	Description string `json:"description" example:"现代化全栈工程实战"`
	CoverURL    string `json:"coverUrl" example:"https://example.com/cover.png"`
	Category    string `json:"category" example:"engineering"`
	IsFree      bool   `json:"isFree"`
}

// UpdateCourseRequest defines parameters for updating an existing course.
type UpdateCourseRequest struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	CoverURL        string `json:"coverUrl"`
	Category        string `json:"category" example:"ai"`
	IsFree          *bool  `json:"isFree"`
	ExpectedVersion int    `json:"expectedVersion" binding:"required" example:"1"`
}

// SubmitCourseRequest defines parameters for submitting a course for review.
type SubmitCourseRequest struct {
	ExpectedVersion int `json:"expectedVersion" binding:"required" example:"1"`
}

// ReviewCourseRequest defines parameters for reviewing a submitted course.
type ReviewCourseRequest struct {
	Decision        string `json:"decision" binding:"required" example:"approved"`
	Reason          string `json:"reason" example:"审核通过符合交付标准"`
	ExpectedVersion int    `json:"expectedVersion" binding:"required" example:"1"`
}

// CloseSalesRequest defines parameters for stopping new sales of a course.
type CloseSalesRequest struct {
	Reason          string `json:"reason" example:"运营调整停止新售"`
	ExpectedVersion int    `json:"expectedVersion" binding:"required" example:"1"`
}

// WithdrawCourseRequest defines parameters for withdrawing a course from delivery.
type WithdrawCourseRequest struct {
	Reason          string `json:"reason" binding:"required" example:"违规内容撤回"`
	ExpectedVersion int    `json:"expectedVersion" binding:"required" example:"1"`
}

// AddSectionRequest defines parameters for adding a curriculum section.
type AddSectionRequest struct {
	Title     string `json:"title" binding:"required" example:"第一章：工程基线"`
	SortOrder int    `json:"sortOrder" example:"1"`
}

// UpdateSectionRequest defines parameters for updating a curriculum section.
type UpdateSectionRequest struct {
	Title     *string `json:"title" example:"第一章：工程基线（更新）"`
	SortOrder *int    `json:"sortOrder" example:"2"`
}

// AddLessonRequest defines parameters for adding a curriculum lesson.
type AddLessonRequest struct {
	SectionID      string `json:"sectionId" binding:"required" example:"sec_12345"`
	Title          string `json:"title" binding:"required" example:"1.1 项目初始化"`
	Type           string `json:"type" binding:"required" example:"video"`
	IsTrial        bool   `json:"isTrial"`
	Content        string `json:"content"`
	MediaVersionID string `json:"mediaVersionId"`
	SortOrder      int    `json:"sortOrder" example:"1"`
}

// UpdateLessonRequest defines parameters for updating a curriculum lesson.
type UpdateLessonRequest struct {
	SectionID      *string `json:"sectionId" example:"sec_12345"`
	Title          *string `json:"title" example:"1.1 项目初始化（修订）"`
	Type           *string `json:"type" example:"article"`
	IsTrial        *bool   `json:"isTrial"`
	Content        *string `json:"content"`
	MediaVersionID *string `json:"mediaVersionId"`
	SortOrder      *int    `json:"sortOrder" example:"2"`
}

// CheckCourseAccessRequest defines payload for internal access check.
type CheckCourseAccessRequest struct {
	UserID   string   `json:"userId" binding:"required" example:"usr_12345"`
	CourseID string   `json:"courseId" binding:"required" example:"crs_12345"`
	Purpose  string   `json:"purpose" binding:"required" example:"playback"`
	Roles    []string `json:"roles,omitempty"`
}

// CheckCourseManagementRequest defines payload for internal management check.
type CheckCourseManagementRequest struct {
	ActorID    string   `json:"actorId" binding:"required" example:"usr_12345"`
	CourseID   string   `json:"courseId" binding:"required" example:"crs_12345"`
	Purpose    string   `json:"purpose" binding:"required" example:"management"`
	ActorRoles []string `json:"actorRoles,omitempty"`
}

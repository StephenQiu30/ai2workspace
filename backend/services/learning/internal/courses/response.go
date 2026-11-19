// Package courses defines response DTOs.
package courses

import "time"

// CourseResponse represents a single course detail.
type CourseResponse struct {
	ID                 string     `json:"id" example:"crs_12345"`
	OwnerID            string     `json:"ownerId" example:"usr_owner"`
	Title              string     `json:"title" example:"深入浅出 Next.js"`
	Description        string     `json:"description" example:"现代化全栈工程实战"`
	CoverURL           string     `json:"coverUrl" example:"https://example.com/cover.png"`
	IsFree             bool       `json:"isFree"`
	ProductionState    string     `json:"productionState" example:"approved"`
	SaleState          string     `json:"saleState" example:"for_sale"`
	DeliveryState      string     `json:"deliveryState" example:"available"`
	PublishedVersion   int        `json:"publishedVersion" example:"1"`
	Version            int        `json:"version" example:"2"`
	FirstDeliverableAt *time.Time `json:"firstDeliverableAt,omitempty"`
	Sections           []Section  `json:"sections"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// CourseListResponse represents paginated course results.
type CourseListResponse struct {
	Courses []CourseResponse `json:"courses"`
	Total   int              `json:"total" example:"42"`
}

// CourseMutationResponse represents the result of a state or content change.
type CourseMutationResponse struct {
	CourseID string `json:"courseId" example:"crs_12345"`
	State    string `json:"state" example:"draft"`
	Version  int    `json:"version" example:"2"`
}

// SaleSnapshotResponse delivers immutable course snapshot data for commerce checkout.
type SaleSnapshotResponse struct {
	CourseID         string `json:"courseId" example:"crs_12345"`
	PublishedVersion int    `json:"publishedVersion" example:"1"`
	Title            string `json:"title" example:"深入浅出 Next.js"`
	SaleState        string `json:"saleState" example:"for_sale"`
	Deliverable      bool   `json:"deliverable" example:"true"`
}

// CheckCourseAccessResponse returns authorization details for consumers.
type CheckCourseAccessResponse struct {
	Allowed        bool   `json:"allowed" example:"true"`
	Reason         string `json:"reason" example:"access_granted"`
	AccessVersion  int    `json:"accessVersion" example:"1"`
	ContentVersion int    `json:"contentVersion" example:"1"`
}

// CheckCourseManagementResponse returns management permission result.
type CheckCourseManagementResponse struct {
	Allowed         bool `json:"allowed" example:"true"`
	ResourceVersion int  `json:"resourceVersion" example:"1"`
}

// DeliveryEvidenceResponse returns reliable availability timestamp for refunds.
type DeliveryEvidenceResponse struct {
	CourseID           string     `json:"courseId" example:"crs_12345"`
	FirstDeliverableAt *time.Time `json:"firstDeliverableAt,omitempty"`
	EvidenceVersion    int        `json:"evidenceVersion" example:"1"`
	ContentVersion     int        `json:"contentVersion" example:"1"`
}

// ErrorResponse conveys failure details without leaking internal traces.
type ErrorResponse struct {
	Error     string `json:"error" example:"bad_request"`
	Message   string `json:"message" example:"Operation could not be completed"`
	RequestID string `json:"requestId,omitempty"`
}

package usage

import (
	"errors"
	"time"
)

var (
	ErrAIPaused                    = errors.New("new AI requests are paused by administrator")
	ErrDailyLimitExceeded          = errors.New("daily question limit exceeded for account")
	ErrRateLimitExceeded           = errors.New("rate limit exceeded: max 3 requests per minute")
	ErrUserConcurrencyLimit        = errors.New("concurrency limit exceeded: account already has an active request")
	ErrPlatformConcurrency         = errors.New("platform concurrency limit reached")
	ErrInputTokensExceeded         = errors.New("input tokens exceed maximum allowed limit of 8000")
	ErrOutputTokensExceeded        = errors.New("output tokens exceed maximum allowed limit of 2000")
	ErrSingleRequestBudgetExceeded = errors.New("single request reservation exceeds maximum allowed 50 cents")
	ErrUserBudgetExceeded          = errors.New("daily budget limit exceeded for account")
	ErrPlatformBudget              = errors.New("platform daily budget limit exceeded")
	ErrPlatformMonthlyBudget       = errors.New("platform monthly budget limit exceeded")
	ErrBackgroundBudgetExceeded    = errors.New("background indexing daily sub-budget exceeded")
	ErrReservationNotFound         = errors.New("budget reservation not found")
	ErrInvalidReservation          = errors.New("reservation already finalized")
	ErrReconciliationNotFound      = errors.New("reconciliation item not found")
	ErrReconciliationResolved      = errors.New("reconciliation item already resolved")
)

// ReservationPurpose defines what the budget hold is for.
type ReservationPurpose string

const (
	PurposeAgentChat ReservationPurpose = "agent_chat"
	PurposeIndexJob  ReservationPurpose = "index_job"
)

// ReservationState defines the lifecycle state of a budget reservation.
type ReservationState string

const (
	StateReserved              ReservationState = "reserved"
	StateSettled               ReservationState = "settled"
	StateReleased              ReservationState = "released"
	StateOverrun               ReservationState = "overrun"
	StatePendingReconciliation ReservationState = "pending_reconciliation"
)

// ReconciliationState defines the review state of an unknown-cost item.
type ReconciliationState string

const (
	ReconciliationPending      ReconciliationState = "pending"
	ReconciliationManualReview ReconciliationState = "manual_review"
	ReconciliationResolved     ReconciliationState = "resolved"
)

// BudgetConfig defines all MVP AI cost governance limits.
type BudgetConfig struct {
	Version               int       `json:"version"`
	UserDailyQuestions    int       `json:"userDailyQuestions"`
	UserPerMinute         int       `json:"userPerMinute"`
	UserConcurrency       int       `json:"userConcurrency"`
	PlatformConcurrency   int       `json:"platformConcurrency"`
	MaxInputTokens        int       `json:"maxInputTokens"`
	MaxOutputTokens       int       `json:"maxOutputTokens"`
	MaxSingleReserveCents int       `json:"maxSingleReserveCents"`
	UserDailyCents        int       `json:"userDailyCents"`
	PlatformDailyCents    int       `json:"platformDailyCents"`
	PlatformMonthlyCents  int       `json:"platformMonthlyCents"`
	BackgroundDailyCents  int       `json:"backgroundDailyCents"`
	AlertThresholdPct     int       `json:"alertThresholdPct"`
	Paused                bool      `json:"paused"`
	PauseReason           string    `json:"pauseReason,omitempty"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

// BudgetConfigUpdate contains optional fields an administrator can adjust.
type BudgetConfigUpdate struct {
	UserDailyQuestions    *int `json:"userDailyQuestions,omitempty"`
	UserPerMinute         *int `json:"userPerMinute,omitempty"`
	UserConcurrency       *int `json:"userConcurrency,omitempty"`
	PlatformConcurrency   *int `json:"platformConcurrency,omitempty"`
	MaxSingleReserveCents *int `json:"maxSingleReserveCents,omitempty"`
	UserDailyCents        *int `json:"userDailyCents,omitempty"`
	PlatformDailyCents    *int `json:"platformDailyCents,omitempty"`
	PlatformMonthlyCents  *int `json:"platformMonthlyCents,omitempty"`
	BackgroundDailyCents  *int `json:"backgroundDailyCents,omitempty"`
	AlertThresholdPct     *int `json:"alertThresholdPct,omitempty"`
}

// DefaultBudgetConfig returns the confirmed MVP cost governance baseline.
func DefaultBudgetConfig(now time.Time) BudgetConfig {
	return BudgetConfig{
		Version:               1,
		UserDailyQuestions:    20,
		UserPerMinute:         3,
		UserConcurrency:       1,
		PlatformConcurrency:   10,
		MaxInputTokens:        8000,
		MaxOutputTokens:       2000,
		MaxSingleReserveCents: 50,
		UserDailyCents:        200,
		PlatformDailyCents:    10000,
		PlatformMonthlyCents:  200000,
		BackgroundDailyCents:  2000,
		AlertThresholdPct:     80,
		Paused:                false,
		UpdatedAt:             now.UTC(),
	}
}

// Reservation represents an active or finalized budget hold bound to an admission period.
type Reservation struct {
	ID             string             `json:"id"`
	UserID         string             `json:"userId"`
	Purpose        ReservationPurpose `json:"purpose"`
	PeriodDay      string             `json:"periodDay"`
	PeriodMonth    string             `json:"periodMonth"`
	ReservedCents  int                `json:"reservedCents"`
	ActualCents    int                `json:"actualCents"`
	UnknownCents   int                `json:"unknownCents,omitempty"`
	State          ReservationState   `json:"state"`
	ReconcileDueAt *time.Time         `json:"reconcileDueAt,omitempty"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

// SettlementStep records one billable execution step.
type SettlementStep struct {
	Step             string `json:"step"` // "search", "llm", "embedding", "indexing"
	Provider         string `json:"provider"`
	ModelID          string `json:"modelId"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	CostCents        int    `json:"costCents"`
	PriceVersion     string `json:"priceVersion,omitempty"`
}

// SettleRequest describes settlement details including partial/failed/unknown-cost handling.
type SettleRequest struct {
	ReservationID string
	Steps         []SettlementStep
	CountSuccess  bool
	UnknownCents  int
	UnknownReason string
}

// ReconciliationItem tracks unresolved unknown costs across period boundaries.
type ReconciliationItem struct {
	ID             string              `json:"id"`
	ReservationID  string              `json:"reservationId"`
	UserID         string              `json:"userId"`
	Purpose        ReservationPurpose  `json:"purpose"`
	PeriodDay      string              `json:"periodDay"`
	PeriodMonth    string              `json:"periodMonth"`
	UnknownCents   int                 `json:"unknownCents"`
	ResolvedCents  int                 `json:"resolvedCents"`
	State          ReconciliationState `json:"state"`
	Reason         string              `json:"reason,omitempty"`
	ResolutionNote string              `json:"resolutionNote,omitempty"`
	DueAt          time.Time           `json:"dueAt"`
	ResolvedAt     *time.Time          `json:"resolvedAt,omitempty"`
	CreatedAt      time.Time           `json:"createdAt"`
	UpdatedAt      time.Time           `json:"updatedAt"`
}

// UsageSummary aggregates user stats and (for admins) full platform governance metrics.
type UsageSummary struct {
	UserID                     string    `json:"userId"`
	Date                       string    `json:"date"`
	Month                      string    `json:"month"`
	QuestionsCount             int       `json:"questionsCount"`
	PendingQuestionsCount      int       `json:"pendingQuestionsCount"`
	MaxDailyQuestions          int       `json:"maxDailyQuestions"`
	TotalCostCents             int       `json:"totalCostCents"`
	ReservedCostCents          int       `json:"reservedCostCents"`
	UnknownCostCents           int       `json:"unknownCostCents"`
	MaxDailyCostCents          int       `json:"maxDailyCostCents"`
	ActiveRequests             int       `json:"activeRequests"`
	Paused                     bool      `json:"paused"`
	PauseReason                string    `json:"pauseReason,omitempty"`
	PlatformCostCents          int       `json:"platformCostCents,omitempty"`
	PlatformDailyReservedCents int       `json:"platformDailyReservedCents,omitempty"`
	PlatformDailyLimitCents    int       `json:"platformDailyLimitCents,omitempty"`
	PlatformMonthlyCostCents   int       `json:"platformMonthlyCostCents,omitempty"`
	PlatformMonthlyReserved    int       `json:"platformMonthlyReservedCents,omitempty"`
	PlatformMonthlyLimitCents  int       `json:"platformMonthlyLimitCents,omitempty"`
	PlatformUnknownCents       int       `json:"platformUnknownCents,omitempty"`
	PlatformActiveRequests     int       `json:"platformActiveRequests,omitempty"`
	BackgroundDailyCostCents   int       `json:"backgroundDailyCostCents,omitempty"`
	BackgroundDailyReserved    int       `json:"backgroundDailyReservedCents,omitempty"`
	BackgroundDailyLimitCents  int       `json:"backgroundDailyLimitCents,omitempty"`
	DailyAlertTriggered        bool      `json:"dailyAlertTriggered,omitempty"`
	MonthlyAlertTriggered      bool      `json:"monthlyAlertTriggered,omitempty"`
	PendingReconciliations     int       `json:"pendingReconciliations,omitempty"`
	UpdatedAt                  time.Time `json:"updatedAt"`
}

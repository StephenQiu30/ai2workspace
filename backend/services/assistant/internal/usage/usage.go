// Package usage provides AI budget admission, concurrency control, cost settlement, and unknown-cost reconciliation.
package usage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

// Service manages atomic AI budget admission, settlement, and unknown-cost tracking.
type Service struct {
	mu      sync.Mutex
	seq     atomic.Int64
	nowFunc func() time.Time
	loc     *time.Location
	config  BudgetConfig

	// Confirmed costs & counts keyed by Asia/Shanghai calendar day/month
	userDailyCount      map[string]int // user_id:day -> confirmed success count
	userDailyPendingCnt map[string]int // user_id:day -> in-flight agent_chat count
	userDailyCost       map[string]int // user_id:day -> confirmed cents
	userDailyReserved   map[string]int // user_id:day -> in-flight reserved cents

	platformDailyCost       map[string]int // day -> confirmed cents
	platformDailyReserved   map[string]int // day -> in-flight reserved cents
	platformMonthlyCost     map[string]int // month -> confirmed cents
	platformMonthlyReserved map[string]int // month -> in-flight reserved cents

	bgDailyCost     map[string]int // day -> confirmed cents
	bgDailyReserved map[string]int // day -> in-flight reserved cents

	// Unresolved unknown costs persist across natural day/month boundaries until reconciled
	userUnknownCents     map[string]int // user_id -> unresolved unknown cents
	platformUnknownCents int
	bgUnknownCents       int

	userActiveReqs     map[string]int
	platformActiveReqs int
	userRequestTimers  map[string][]time.Time

	reservations    map[string]*Reservation
	steps           map[string][]SettlementStep
	reconciliations map[string]*ReconciliationItem

	store *usageStore
}

// NewService creates an in-memory usage governance service.
func NewService() *Service {
	return NewServiceWithPool(nil)
}

// NewServiceWithPool creates a usage governance service backed by optional PostgreSQL persistence.
func NewServiceWithPool(pool *pgxpool.Pool) *Service {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil || loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Now()
	s := &Service{
		nowFunc:                 time.Now,
		loc:                     loc,
		config:                  DefaultBudgetConfig(now),
		userDailyCount:          make(map[string]int),
		userDailyPendingCnt:     make(map[string]int),
		userDailyCost:           make(map[string]int),
		userDailyReserved:       make(map[string]int),
		platformDailyCost:       make(map[string]int),
		platformDailyReserved:   make(map[string]int),
		platformMonthlyCost:     make(map[string]int),
		platformMonthlyReserved: make(map[string]int),
		bgDailyCost:             make(map[string]int),
		bgDailyReserved:         make(map[string]int),
		userUnknownCents:        make(map[string]int),
		userActiveReqs:          make(map[string]int),
		userRequestTimers:       make(map[string][]time.Time),
		reservations:            make(map[string]*Reservation),
		steps:                   make(map[string][]SettlementStep),
		reconciliations:         make(map[string]*ReconciliationItem),
	}
	if pool != nil {
		s.store = newUsageStore(pool)
		s.bootstrapFromStore(context.Background())
	}
	return s
}

// SetNowFunc overrides the internal clock for deterministic day/month rollover and 24h tests.
func (s *Service) SetNowFunc(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fn == nil {
		s.nowFunc = time.Now
	} else {
		s.nowFunc = fn
	}
}

func (s *Service) periodKeys(now time.Time) (day string, month string) {
	t := now.In(s.loc)
	return t.Format("2006-01-02"), t.Format("2006-01")
}

// GetConfig returns a copy of the active budget governance configuration.
func (s *Service) GetConfig() BudgetConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// UpdateConfig adjusts governance budget limits (admin only).
func (s *Service) UpdateConfig(ctx context.Context, update BudgetConfigUpdate) (BudgetConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.config
	if update.UserDailyQuestions != nil {
		if *update.UserDailyQuestions <= 0 {
			return BudgetConfig{}, errors.New("userDailyQuestions must be positive")
		}
		cfg.UserDailyQuestions = *update.UserDailyQuestions
	}
	if update.UserPerMinute != nil {
		if *update.UserPerMinute <= 0 {
			return BudgetConfig{}, errors.New("userPerMinute must be positive")
		}
		cfg.UserPerMinute = *update.UserPerMinute
	}
	if update.UserConcurrency != nil {
		if *update.UserConcurrency <= 0 {
			return BudgetConfig{}, errors.New("userConcurrency must be positive")
		}
		cfg.UserConcurrency = *update.UserConcurrency
	}
	if update.PlatformConcurrency != nil {
		if *update.PlatformConcurrency <= 0 {
			return BudgetConfig{}, errors.New("platformConcurrency must be positive")
		}
		cfg.PlatformConcurrency = *update.PlatformConcurrency
	}
	if update.MaxSingleReserveCents != nil {
		if *update.MaxSingleReserveCents <= 0 {
			return BudgetConfig{}, errors.New("maxSingleReserveCents must be positive")
		}
		cfg.MaxSingleReserveCents = *update.MaxSingleReserveCents
	}
	if update.UserDailyCents != nil {
		if *update.UserDailyCents <= 0 {
			return BudgetConfig{}, errors.New("userDailyCents must be positive")
		}
		cfg.UserDailyCents = *update.UserDailyCents
	}
	if update.PlatformDailyCents != nil {
		if *update.PlatformDailyCents <= 0 {
			return BudgetConfig{}, errors.New("platformDailyCents must be positive")
		}
		cfg.PlatformDailyCents = *update.PlatformDailyCents
	}
	if update.PlatformMonthlyCents != nil {
		if *update.PlatformMonthlyCents <= 0 {
			return BudgetConfig{}, errors.New("platformMonthlyCents must be positive")
		}
		cfg.PlatformMonthlyCents = *update.PlatformMonthlyCents
	}
	if update.BackgroundDailyCents != nil {
		if *update.BackgroundDailyCents <= 0 {
			return BudgetConfig{}, errors.New("backgroundDailyCents must be positive")
		}
		cfg.BackgroundDailyCents = *update.BackgroundDailyCents
	}
	if update.AlertThresholdPct != nil {
		if *update.AlertThresholdPct <= 0 || *update.AlertThresholdPct > 100 {
			return BudgetConfig{}, errors.New("alertThresholdPct must be between 1 and 100")
		}
		cfg.AlertThresholdPct = *update.AlertThresholdPct
	}
	if cfg.BackgroundDailyCents > cfg.PlatformDailyCents {
		return BudgetConfig{}, errors.New("backgroundDailyCents cannot exceed platformDailyCents")
	}

	cfg.Version++
	cfg.UpdatedAt = s.nowFunc().UTC()

	if s.store != nil {
		if err := s.store.saveConfig(ctx, cfg); err != nil {
			return BudgetConfig{}, err
		}
	}
	s.config = cfg
	return cfg, nil
}

// SetPaused pauses or resumes admitting new AI requests (admin only).
func (s *Service) SetPaused(ctx context.Context, paused bool, reason string) (BudgetConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.config
	cfg.Paused = paused
	if paused {
		cfg.PauseReason = reason
	} else {
		cfg.PauseReason = ""
	}
	cfg.Version++
	cfg.UpdatedAt = s.nowFunc().UTC()

	if s.store != nil {
		if err := s.store.saveConfig(ctx, cfg); err != nil {
			return BudgetConfig{}, err
		}
	}
	s.config = cfg
	return cfg, nil
}

// EstimateAnswerReserveCents dynamically calculates the conservative reservation hold:
// (inputTokens + maxOutputTokens + maxSearchAttempts*searchCost) * 1.20 (ceiling),
// strictly validating <=8000 input tokens, <=2000 output tokens, and <=50 cents final reserve.
func (s *Service) EstimateAnswerReserveCents(
	inputTokens int,
	maxOutputTokens int,
	promptPricePerK int,
	completionPricePerK int,
	maxSearchAttempts int,
	searchCostPerAttemptCents int,
) (int, error) {
	s.mu.Lock()
	cfg := s.config
	s.mu.Unlock()

	if inputTokens < 0 {
		inputTokens = 0
	}
	if inputTokens > cfg.MaxInputTokens {
		return 0, fmt.Errorf("%w: got %d", ErrInputTokensExceeded, inputTokens)
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = cfg.MaxOutputTokens
	}
	if maxOutputTokens > cfg.MaxOutputTokens {
		return 0, fmt.Errorf("%w: got %d", ErrOutputTokensExceeded, maxOutputTokens)
	}
	if maxSearchAttempts <= 0 {
		maxSearchAttempts = 2
	}
	if searchCostPerAttemptCents < 0 {
		searchCostPerAttemptCents = 0
	}

	inputCost := (inputTokens*promptPricePerK + 999) / 1000
	outputCost := (maxOutputTokens*completionPricePerK + 999) / 1000
	searchCost := maxSearchAttempts * searchCostPerAttemptCents
	rawCost := inputCost + outputCost + searchCost
	if rawCost < 1 {
		rawCost = 1
	}

	// Add 20% conservative buffer using integer ceiling math.
	finalReserve := (rawCost*120 + 99) / 100
	if finalReserve > cfg.MaxSingleReserveCents {
		return 0, fmt.Errorf("%w: estimated %d cents > limit %d cents", ErrSingleRequestBudgetExceeded, finalReserve, cfg.MaxSingleReserveCents)
	}
	return finalReserve, nil
}

// Reserve atomically checks pause, concurrency, daily limits, confirmed+reserved+unknown budgets, and rate limits.
// Rate limit quota is ONLY consumed if all preceding admission checks succeed.
func (s *Service) Reserve(ctx context.Context, userID string, purpose ReservationPurpose, estimatedCents int) (*Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.config
	if cfg.Paused {
		return nil, ErrAIPaused
	}
	if estimatedCents <= 0 {
		estimatedCents = 1
	}
	if estimatedCents > cfg.MaxSingleReserveCents {
		return nil, ErrSingleRequestBudgetExceeded
	}

	now := s.nowFunc()
	day, month := s.periodKeys(now)
	userKey := fmt.Sprintf("%s:%s", userID, day)

	if purpose == PurposeAgentChat {
		// 1. Concurrency check
		if s.platformActiveReqs >= cfg.PlatformConcurrency {
			return nil, ErrPlatformConcurrency
		}
		if s.userActiveReqs[userID] >= cfg.UserConcurrency {
			return nil, ErrUserConcurrencyLimit
		}

		// 2. Daily question limit check (confirmed + pending)
		if s.userDailyCount[userKey]+s.userDailyPendingCnt[userKey] >= cfg.UserDailyQuestions {
			return nil, ErrDailyLimitExceeded
		}

		// 3. User daily budget check (confirmed + reserved + unresolved unknown)
		userProjected := s.userDailyCost[userKey] + s.userDailyReserved[userKey] + s.userUnknownCents[userID] + estimatedCents
		if userProjected > cfg.UserDailyCents {
			return nil, ErrUserBudgetExceeded
		}

		// 4. Platform daily & monthly budget check (confirmed + reserved + unresolved unknown)
		platformDailyProjected := s.platformDailyCost[day] + s.platformDailyReserved[day] + s.platformUnknownCents + estimatedCents
		if platformDailyProjected > cfg.PlatformDailyCents {
			return nil, ErrPlatformBudget
		}
		platformMonthlyProjected := s.platformMonthlyCost[month] + s.platformMonthlyReserved[month] + s.platformUnknownCents + estimatedCents
		if platformMonthlyProjected > cfg.PlatformMonthlyCents {
			return nil, ErrPlatformMonthlyBudget
		}

		// 5. Rolling 1-minute rate limit check LAST so rejected budget/daily calls do not burn rate quota!
		oneMinAgo := now.Add(-1 * time.Minute)
		timers := s.userRequestTimers[userID]
		validTimers := make([]time.Time, 0, len(timers)+1)
		for _, t := range timers {
			if t.After(oneMinAgo) {
				validTimers = append(validTimers, t)
			}
		}
		if len(validTimers) >= cfg.UserPerMinute {
			s.userRequestTimers[userID] = validTimers
			return nil, ErrRateLimitExceeded
		}
		validTimers = append(validTimers, now)
		s.userRequestTimers[userID] = validTimers
	} else if purpose == PurposeIndexJob {
		// Background indexing: subject to ¥20/day sub-quota AND shares platform ¥100/day & ¥2000/month.
		bgProjected := s.bgDailyCost[day] + s.bgDailyReserved[day] + s.bgUnknownCents + estimatedCents
		if bgProjected > cfg.BackgroundDailyCents {
			return nil, ErrBackgroundBudgetExceeded
		}
		platformDailyProjected := s.platformDailyCost[day] + s.platformDailyReserved[day] + s.platformUnknownCents + estimatedCents
		if platformDailyProjected > cfg.PlatformDailyCents {
			return nil, ErrPlatformBudget
		}
		platformMonthlyProjected := s.platformMonthlyCost[month] + s.platformMonthlyReserved[month] + s.platformUnknownCents + estimatedCents
		if platformMonthlyProjected > cfg.PlatformMonthlyCents {
			return nil, ErrPlatformMonthlyBudget
		}
	}

	seq := s.seq.Add(1)
	resID := fmt.Sprintf("res_%d_%d", now.UnixNano(), seq)
	reservation := &Reservation{
		ID:            resID,
		UserID:        userID,
		Purpose:       purpose,
		PeriodDay:     day,
		PeriodMonth:   month,
		ReservedCents: estimatedCents,
		State:         StateReserved,
		ActualCents:   0,
		CreatedAt:     now.UTC(),
		UpdatedAt:     now.UTC(),
	}

	if s.store != nil {
		if err := s.store.insertReservation(ctx, reservation); err != nil {
			return nil, err
		}
	}

	s.reservations[resID] = reservation
	s.platformDailyReserved[day] += estimatedCents
	s.platformMonthlyReserved[month] += estimatedCents
	if purpose == PurposeAgentChat {
		s.userDailyReserved[userKey] += estimatedCents
		s.userDailyPendingCnt[userKey]++
		s.userActiveReqs[userID]++
		s.platformActiveReqs++
	} else if purpose == PurposeIndexJob {
		s.bgDailyReserved[day] += estimatedCents
	}

	resCopy := *reservation
	return &resCopy, nil
}

// Settle finalizes a completed successful reservation with actual step costs.
func (s *Service) Settle(ctx context.Context, reservationID string, steps []SettlementStep) (*Reservation, error) {
	return s.SettleDetailed(ctx, SettleRequest{
		ReservationID: reservationID,
		Steps:         steps,
		CountSuccess:  true,
	})
}

// SettleFailed finalizes a failed/cancelled reservation WITHOUT counting a successful answer,
// while truthfully charging all incurred costs (e.g. completed search calls) and retaining any unknown-cost hold.
func (s *Service) SettleFailed(
	ctx context.Context,
	reservationID string,
	steps []SettlementStep,
	unknownCents int,
	unknownReason string,
) (*Reservation, error) {
	return s.SettleDetailed(ctx, SettleRequest{
		ReservationID: reservationID,
		Steps:         steps,
		CountSuccess:  false,
		UnknownCents:  unknownCents,
		UnknownReason: unknownReason,
	})
}

// SettleDetailed settles a reservation with explicit control over success count and unknown cost holds.
func (s *Service) SettleDetailed(ctx context.Context, req SettleRequest) (*Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, exists := s.reservations[req.ReservationID]
	if !exists {
		return nil, ErrReservationNotFound
	}
	if res.State != StateReserved {
		return nil, ErrInvalidReservation
	}

	totalActualCents := 0
	for _, step := range req.Steps {
		if step.CostCents > 0 {
			totalActualCents += step.CostCents
		}
	}
	unknownCents := req.UnknownCents
	if unknownCents < 0 {
		unknownCents = 0
	}

	now := s.nowFunc().UTC()
	day := res.PeriodDay
	month := res.PeriodMonth
	userKey := fmt.Sprintf("%s:%s", res.UserID, day)

	// Release the original in-flight reserved hold from the original admission period.
	s.platformDailyReserved[day] = maxZero(s.platformDailyReserved[day] - res.ReservedCents)
	s.platformMonthlyReserved[month] = maxZero(s.platformMonthlyReserved[month] - res.ReservedCents)
	if res.Purpose == PurposeAgentChat {
		s.userDailyReserved[userKey] = maxZero(s.userDailyReserved[userKey] - res.ReservedCents)
		s.userDailyPendingCnt[userKey] = maxZero(s.userDailyPendingCnt[userKey] - 1)
		s.userActiveReqs[res.UserID] = maxZero(s.userActiveReqs[res.UserID] - 1)
		s.platformActiveReqs = maxZero(s.platformActiveReqs - 1)
		if req.CountSuccess {
			s.userDailyCount[userKey]++
		}
	} else if res.Purpose == PurposeIndexJob {
		s.bgDailyReserved[day] = maxZero(s.bgDailyReserved[day] - res.ReservedCents)
	}

	// Record confirmed incurred costs against the original admission period (never truncate overruns).
	if totalActualCents > 0 {
		s.platformDailyCost[day] += totalActualCents
		s.platformMonthlyCost[month] += totalActualCents
		if res.Purpose == PurposeAgentChat {
			s.userDailyCost[userKey] += totalActualCents
		} else if res.Purpose == PurposeIndexJob {
			s.bgDailyCost[day] += totalActualCents
		}
	}

	res.ActualCents = totalActualCents
	res.UnknownCents = unknownCents
	res.UpdatedAt = now

	var recItem *ReconciliationItem
	if unknownCents > 0 {
		dueAt := now.Add(24 * time.Hour)
		res.State = StatePendingReconciliation
		res.ReconcileDueAt = &dueAt

		s.platformUnknownCents += unknownCents
		if res.Purpose == PurposeAgentChat {
			s.userUnknownCents[res.UserID] += unknownCents
		} else if res.Purpose == PurposeIndexJob {
			s.bgUnknownCents += unknownCents
		}

		recID := fmt.Sprintf("rec_%d_%d", now.UnixNano(), s.seq.Add(1))
		recItem = &ReconciliationItem{
			ID:            recID,
			ReservationID: res.ID,
			UserID:        res.UserID,
			Purpose:       res.Purpose,
			PeriodDay:     day,
			PeriodMonth:   month,
			UnknownCents:  unknownCents,
			State:         ReconciliationPending,
			Reason:        req.UnknownReason,
			DueAt:         dueAt,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		s.reconciliations[recID] = recItem
	} else if totalActualCents == 0 && !req.CountSuccess {
		res.State = StateReleased
	} else if totalActualCents > res.ReservedCents {
		res.State = StateOverrun
	} else {
		res.State = StateSettled
	}

	s.steps[res.ID] = append([]SettlementStep(nil), req.Steps...)

	if s.store != nil {
		if err := s.store.persistSettlement(ctx, res, req.Steps, recItem); err != nil {
			return nil, err
		}
	}

	resCopy := *res
	return &resCopy, nil
}

// Release cancels an active reservation when zero external billable calls occurred.
func (s *Service) Release(ctx context.Context, reservationID string) error {
	_, err := s.SettleDetailed(ctx, SettleRequest{
		ReservationID: reservationID,
		Steps:         nil,
		CountSuccess:  false,
		UnknownCents:  0,
	})
	return err
}

// ListReconciliations returns unknown-cost reconciliation items, automatically flagging items overdue past 24h for manual review.
func (s *Service) ListReconciliations(ctx context.Context) []*ReconciliationItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc().UTC()
	out := make([]*ReconciliationItem, 0, len(s.reconciliations))
	for _, item := range s.reconciliations {
		if item.State == ReconciliationPending && !now.Before(item.DueAt) {
			item.State = ReconciliationManualReview
			item.UpdatedAt = now
			if s.store != nil {
				_ = s.store.updateReconciliation(ctx, item)
			}
		}
		cp := *item
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// ResolveReconciliation finalizes an unknown-cost reconciliation item:
// releases the cross-period unknown risk hold and credits the confirmed amount to the original admission period.
func (s *Service) ResolveReconciliation(ctx context.Context, reconciliationID string, resolvedCents int, note string) (*ReconciliationItem, error) {
	if resolvedCents < 0 {
		return nil, errors.New("resolvedCents must be non-negative")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.reconciliations[reconciliationID]
	if !exists {
		return nil, ErrReconciliationNotFound
	}
	if item.State == ReconciliationResolved {
		return nil, ErrReconciliationResolved
	}

	now := s.nowFunc().UTC()

	// Release unknown risk hold
	s.platformUnknownCents = maxZero(s.platformUnknownCents - item.UnknownCents)
	if item.Purpose == PurposeAgentChat {
		s.userUnknownCents[item.UserID] = maxZero(s.userUnknownCents[item.UserID] - item.UnknownCents)
	} else if item.Purpose == PurposeIndexJob {
		s.bgUnknownCents = maxZero(s.bgUnknownCents - item.UnknownCents)
	}

	// Credit resolved confirmed amount back to the original admission period (day & month)
	if resolvedCents > 0 {
		s.platformDailyCost[item.PeriodDay] += resolvedCents
		s.platformMonthlyCost[item.PeriodMonth] += resolvedCents
		if item.Purpose == PurposeAgentChat {
			origUserKey := fmt.Sprintf("%s:%s", item.UserID, item.PeriodDay)
			s.userDailyCost[origUserKey] += resolvedCents
		} else if item.Purpose == PurposeIndexJob {
			s.bgDailyCost[item.PeriodDay] += resolvedCents
		}
	}

	if res, ok := s.reservations[item.ReservationID]; ok {
		res.ActualCents += resolvedCents
		res.UnknownCents = 0
		res.UpdatedAt = now
		if res.ActualCents > res.ReservedCents {
			res.State = StateOverrun
		} else {
			res.State = StateSettled
		}
		if s.store != nil {
			_ = s.store.updateReservation(ctx, res)
		}
	}

	item.ResolvedCents = resolvedCents
	item.ResolutionNote = note
	item.State = ReconciliationResolved
	item.ResolvedAt = &now
	item.UpdatedAt = now

	if s.store != nil {
		if err := s.store.updateReconciliation(ctx, item); err != nil {
			return nil, err
		}
	}

	cp := *item
	return &cp, nil
}

// GetSummary returns usage statistics for a user; when includePlatform is true (admin), full platform metrics & 80% alerts are included.
func (s *Service) GetSummary(ctx context.Context, userID string, includePlatform bool) *UsageSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc()
	day, month := s.periodKeys(now)
	userKey := fmt.Sprintf("%s:%s", userID, day)
	cfg := s.config

	summary := &UsageSummary{
		UserID:                userID,
		Date:                  day,
		Month:                 month,
		QuestionsCount:        s.userDailyCount[userKey],
		PendingQuestionsCount: s.userDailyPendingCnt[userKey],
		MaxDailyQuestions:     cfg.UserDailyQuestions,
		TotalCostCents:        s.userDailyCost[userKey],
		ReservedCostCents:     s.userDailyReserved[userKey],
		UnknownCostCents:      s.userUnknownCents[userID],
		MaxDailyCostCents:     cfg.UserDailyCents,
		ActiveRequests:        s.userActiveReqs[userID],
		Paused:                cfg.Paused,
		PauseReason:           cfg.PauseReason,
		UpdatedAt:             now.UTC(),
	}

	if includePlatform {
		dailyTotal := s.platformDailyCost[day] + s.platformDailyReserved[day] + s.platformUnknownCents
		monthlyTotal := s.platformMonthlyCost[month] + s.platformMonthlyReserved[month] + s.platformUnknownCents

		pendingRecs := 0
		for _, item := range s.reconciliations {
			if item.State != ReconciliationResolved {
				pendingRecs++
			}
		}

		summary.PlatformCostCents = s.platformDailyCost[day]
		summary.PlatformDailyReservedCents = s.platformDailyReserved[day]
		summary.PlatformDailyLimitCents = cfg.PlatformDailyCents
		summary.PlatformMonthlyCostCents = s.platformMonthlyCost[month]
		summary.PlatformMonthlyReserved = s.platformMonthlyReserved[month]
		summary.PlatformMonthlyLimitCents = cfg.PlatformMonthlyCents
		summary.PlatformUnknownCents = s.platformUnknownCents
		summary.PlatformActiveRequests = s.platformActiveReqs
		summary.BackgroundDailyCostCents = s.bgDailyCost[day]
		summary.BackgroundDailyReserved = s.bgDailyReserved[day]
		summary.BackgroundDailyLimitCents = cfg.BackgroundDailyCents
		summary.DailyAlertTriggered = cfg.PlatformDailyCents > 0 && dailyTotal*100 >= cfg.PlatformDailyCents*cfg.AlertThresholdPct
		summary.MonthlyAlertTriggered = cfg.PlatformMonthlyCents > 0 && monthlyTotal*100 >= cfg.PlatformMonthlyCents*cfg.AlertThresholdPct
		summary.PendingReconciliations = pendingRecs
	}

	return summary
}

// GetReservation returns a copy of a reservation by ID.
func (s *Service) GetReservation(reservationID string) (*Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, ok := s.reservations[reservationID]
	if !ok {
		return nil, ErrReservationNotFound
	}
	cp := *res
	return &cp, nil
}

func maxZero(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

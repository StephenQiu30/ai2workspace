// Package usage provides AI budget admission, concurrency control, and cost settlement.
package usage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrDailyLimitExceeded   = errors.New("daily question limit of 20 exceeded for account")
	ErrRateLimitExceeded    = errors.New("rate limit exceeded: max 3 requests per minute")
	ErrUserConcurrencyLimit = errors.New("concurrency limit exceeded: account already has 1 active request")
	ErrPlatformConcurrency  = errors.New("platform concurrency limit of 10 reached")
	ErrUserBudgetExceeded   = errors.New("daily budget limit exceeded for account (max 200 cents)")
	ErrPlatformBudget       = errors.New("platform daily budget limit exceeded (max 10000 cents)")
	ErrReservationNotFound  = errors.New("budget reservation not found")
	ErrInvalidReservation   = errors.New("reservation already settled or released")
)

// ReservationPurpose defines what the reservation is for.
type ReservationPurpose string

const (
	PurposeAgentChat ReservationPurpose = "agent_chat"
	PurposeIndexJob  ReservationPurpose = "index_job"
)

// ReservationState defines current state of reservation.
type ReservationState string

const (
	StateReserved ReservationState = "reserved"
	StateSettled  ReservationState = "settled"
	StateReleased ReservationState = "released"
	StateOverrun  ReservationState = "overrun"
)

// Reservation represents an active budget hold.
type Reservation struct {
	ID            string             `json:"id"`
	UserID        string             `json:"userId"`
	Purpose       ReservationPurpose `json:"purpose"`
	ReservedCents int                `json:"reservedCents"`
	State         ReservationState   `json:"state"`
	ActualCents   int                `json:"actualCents"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
}

// SettlementStep records one cost step.
type SettlementStep struct {
	Step             string `json:"step"`     // "search", "llm", "embedding"
	Provider         string `json:"provider"` // "openai", "deepseek", etc.
	ModelID          string `json:"modelId"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	CostCents        int    `json:"costCents"`
}

// UsageSummary aggregates daily stats for a user.
type UsageSummary struct {
	UserID            string    `json:"userId"`
	Date              string    `json:"date"`
	QuestionsCount    int       `json:"questionsCount"`
	MaxDailyQuestions int       `json:"maxDailyQuestions"`
	TotalCostCents    int       `json:"totalCostCents"`
	MaxDailyCostCents int       `json:"maxDailyCostCents"`
	PlatformCostCents int       `json:"platformCostCents"`
	ActiveRequests    int       `json:"activeRequests"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// Service manages budget and usage state.
type Service struct {
	mu sync.Mutex

	// In-memory runtime tracking (concurrent safe, reset per day)
	userDailyCount     map[string]int         // user_id:date -> count
	userDailyCost      map[string]int         // user_id:date -> cents
	platformDailyCost  map[string]int         // date -> cents
	userActiveReqs     map[string]int         // user_id -> count
	platformActiveReqs int                    // total
	userRequestTimers  map[string][]time.Time // user_id -> slice of timestamps in last min

	reservations map[string]*Reservation
	steps        map[string][]SettlementStep
}

// NewService creates a new usage governance service.
func NewService() *Service {
	return &Service{
		userDailyCount:    make(map[string]int),
		userDailyCost:     make(map[string]int),
		platformDailyCost: make(map[string]int),
		userActiveReqs:    make(map[string]int),
		userRequestTimers: make(map[string][]time.Time),
		reservations:      make(map[string]*Reservation),
		steps:             make(map[string][]SettlementStep),
	}
}

func (s *Service) todayKey() string {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.UTC
	}
	return time.Now().In(loc).Format("2006-01-02")
}

// Reserve checks limits and places a budget hold.
func (s *Service) Reserve(ctx context.Context, userID string, purpose ReservationPurpose, estimatedCents int) (*Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := s.todayKey()
	userKey := fmt.Sprintf("%s:%s", userID, today)

	now := time.Now()

	// 1. Concurrency limit check
	if s.platformActiveReqs >= 10 {
		return nil, ErrPlatformConcurrency
	}
	if s.userActiveReqs[userID] >= 1 {
		return nil, ErrUserConcurrencyLimit
	}

	// 2. Rate limit (max 3 per minute)
	timers := s.userRequestTimers[userID]
	oneMinAgo := now.Add(-1 * time.Minute)
	validTimers := make([]time.Time, 0, len(timers))
	for _, t := range timers {
		if t.After(oneMinAgo) {
			validTimers = append(validTimers, t)
		}
	}
	if len(validTimers) >= 3 {
		s.userRequestTimers[userID] = validTimers
		return nil, ErrRateLimitExceeded
	}
	validTimers = append(validTimers, now)
	s.userRequestTimers[userID] = validTimers

	// 3. Daily question count limit (20 questions per day for agent_chat)
	if purpose == PurposeAgentChat {
		if s.userDailyCount[userKey] >= 20 {
			return nil, ErrDailyLimitExceeded
		}
	}

	// 4. Daily budget check (User: max 200 cents = ¥2; Platform: max 10000 cents = ¥100)
	if s.userDailyCost[userKey]+estimatedCents > 200 {
		return nil, ErrUserBudgetExceeded
	}
	if s.platformDailyCost[today]+estimatedCents > 10000 {
		return nil, ErrPlatformBudget
	}

	// Record hold
	resID := fmt.Sprintf("res_%d_%s", now.UnixNano(), userID)
	reservation := &Reservation{
		ID:            resID,
		UserID:        userID,
		Purpose:       purpose,
		ReservedCents: estimatedCents,
		State:         StateReserved,
		ActualCents:   0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.reservations[resID] = reservation
	s.userActiveReqs[userID]++
	s.platformActiveReqs++

	return reservation, nil
}

// Settle finalizes the reservation with actual token costs.
func (s *Service) Settle(ctx context.Context, reservationID string, steps []SettlementStep) (*Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, exists := s.reservations[reservationID]
	if !exists {
		return nil, ErrReservationNotFound
	}
	if res.State != StateReserved {
		return nil, ErrInvalidReservation
	}

	totalActualCents := 0
	for _, step := range steps {
		totalActualCents += step.CostCents
	}

	now := time.Now()
	res.ActualCents = totalActualCents
	res.UpdatedAt = now

	if totalActualCents > res.ReservedCents {
		res.State = StateOverrun
	} else {
		res.State = StateSettled
	}

	today := s.todayKey()
	userKey := fmt.Sprintf("%s:%s", res.UserID, today)

	if res.Purpose == PurposeAgentChat {
		s.userDailyCount[userKey]++
	}
	s.userDailyCost[userKey] += totalActualCents
	s.platformDailyCost[today] += totalActualCents

	// Decrement active concurrency
	if s.userActiveReqs[res.UserID] > 0 {
		s.userActiveReqs[res.UserID]--
	}
	if s.platformActiveReqs > 0 {
		s.platformActiveReqs--
	}

	s.steps[reservationID] = steps

	return res, nil
}

// Release cancels an active reservation without incrementing question counts or charging costs.
func (s *Service) Release(ctx context.Context, reservationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, exists := s.reservations[reservationID]
	if !exists {
		return ErrReservationNotFound
	}
	if res.State != StateReserved {
		return ErrInvalidReservation
	}

	res.State = StateReleased
	res.UpdatedAt = time.Now()

	// Decrement active concurrency
	if s.userActiveReqs[res.UserID] > 0 {
		s.userActiveReqs[res.UserID]--
	}
	if s.platformActiveReqs > 0 {
		s.platformActiveReqs--
	}

	return nil
}

// GetSummary returns current user and platform usage statistics.
func (s *Service) GetSummary(ctx context.Context, userID string) *UsageSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := s.todayKey()
	userKey := fmt.Sprintf("%s:%s", userID, today)

	return &UsageSummary{
		UserID:            userID,
		Date:              today,
		QuestionsCount:    s.userDailyCount[userKey],
		MaxDailyQuestions: 20,
		TotalCostCents:    s.userDailyCost[userKey],
		MaxDailyCostCents: 200,
		PlatformCostCents: s.platformDailyCost[today],
		ActiveRequests:    s.userActiveReqs[userID],
		UpdatedAt:         time.Now(),
	}
}

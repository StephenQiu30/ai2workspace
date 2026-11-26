package usage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	usagedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/usage"
)

// Service manages atomic AI budget admission, settlement, and unknown-cost tracking.
type Service struct {
	mu      sync.Mutex
	seq     atomic.Int64
	nowFunc func() time.Time
	loc     *time.Location
	config  usagedomain.BudgetConfig

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

	reservations    map[string]*usagedomain.Reservation
	steps           map[string][]usagedomain.SettlementStep
	reconciliations map[string]*usagedomain.ReconciliationItem

	store Store
}

// NewService creates an in-memory usage governance service.
func NewService() *Service {
	return NewServiceWithStore(nil)
}

// NewServiceWithStore creates a usage governance service backed by optional PostgreSQL persistence.
func NewServiceWithStore(store Store) *Service {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil || loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Now()
	s := &Service{
		nowFunc:                 time.Now,
		loc:                     loc,
		config:                  usagedomain.DefaultBudgetConfig(now),
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
		reservations:            make(map[string]*usagedomain.Reservation),
		steps:                   make(map[string][]usagedomain.SettlementStep),
		reconciliations:         make(map[string]*usagedomain.ReconciliationItem),
	}
	if store != nil {
		s.store = store
		s.restore(store.Load(context.Background(), s.config))
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
func (s *Service) GetConfig() usagedomain.BudgetConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// UpdateConfig adjusts governance budget limits (admin only).
func (s *Service) UpdateConfig(ctx context.Context, update usagedomain.BudgetConfigUpdate) (usagedomain.BudgetConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.config
	if update.UserDailyQuestions != nil {
		if *update.UserDailyQuestions <= 0 {
			return usagedomain.BudgetConfig{}, errors.New("userDailyQuestions must be positive")
		}
		cfg.UserDailyQuestions = *update.UserDailyQuestions
	}
	if update.UserPerMinute != nil {
		if *update.UserPerMinute <= 0 {
			return usagedomain.BudgetConfig{}, errors.New("userPerMinute must be positive")
		}
		cfg.UserPerMinute = *update.UserPerMinute
	}
	if update.UserConcurrency != nil {
		if *update.UserConcurrency <= 0 {
			return usagedomain.BudgetConfig{}, errors.New("userConcurrency must be positive")
		}
		cfg.UserConcurrency = *update.UserConcurrency
	}
	if update.PlatformConcurrency != nil {
		if *update.PlatformConcurrency <= 0 {
			return usagedomain.BudgetConfig{}, errors.New("platformConcurrency must be positive")
		}
		cfg.PlatformConcurrency = *update.PlatformConcurrency
	}
	if update.MaxSingleReserveCents != nil {
		if *update.MaxSingleReserveCents <= 0 {
			return usagedomain.BudgetConfig{}, errors.New("maxSingleReserveCents must be positive")
		}
		cfg.MaxSingleReserveCents = *update.MaxSingleReserveCents
	}
	if update.UserDailyCents != nil {
		if *update.UserDailyCents <= 0 {
			return usagedomain.BudgetConfig{}, errors.New("userDailyCents must be positive")
		}
		cfg.UserDailyCents = *update.UserDailyCents
	}
	if update.PlatformDailyCents != nil {
		if *update.PlatformDailyCents <= 0 {
			return usagedomain.BudgetConfig{}, errors.New("platformDailyCents must be positive")
		}
		cfg.PlatformDailyCents = *update.PlatformDailyCents
	}
	if update.PlatformMonthlyCents != nil {
		if *update.PlatformMonthlyCents <= 0 {
			return usagedomain.BudgetConfig{}, errors.New("platformMonthlyCents must be positive")
		}
		cfg.PlatformMonthlyCents = *update.PlatformMonthlyCents
	}
	if update.BackgroundDailyCents != nil {
		if *update.BackgroundDailyCents <= 0 {
			return usagedomain.BudgetConfig{}, errors.New("backgroundDailyCents must be positive")
		}
		cfg.BackgroundDailyCents = *update.BackgroundDailyCents
	}
	if update.AlertThresholdPct != nil {
		if *update.AlertThresholdPct <= 0 || *update.AlertThresholdPct > 100 {
			return usagedomain.BudgetConfig{}, errors.New("alertThresholdPct must be between 1 and 100")
		}
		cfg.AlertThresholdPct = *update.AlertThresholdPct
	}
	if cfg.BackgroundDailyCents > cfg.PlatformDailyCents {
		return usagedomain.BudgetConfig{}, errors.New("backgroundDailyCents cannot exceed platformDailyCents")
	}

	cfg.Version++
	cfg.UpdatedAt = s.nowFunc().UTC()

	if s.store != nil {
		if err := s.store.SaveConfig(ctx, cfg); err != nil {
			return usagedomain.BudgetConfig{}, err
		}
	}
	s.config = cfg
	return cfg, nil
}

// SetPaused pauses or resumes admitting new AI requests (admin only).
func (s *Service) SetPaused(ctx context.Context, paused bool, reason string) (usagedomain.BudgetConfig, error) {
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
		if err := s.store.SaveConfig(ctx, cfg); err != nil {
			return usagedomain.BudgetConfig{}, err
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
		return 0, fmt.Errorf("%w: got %d", usagedomain.ErrInputTokensExceeded, inputTokens)
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = cfg.MaxOutputTokens
	}
	if maxOutputTokens > cfg.MaxOutputTokens {
		return 0, fmt.Errorf("%w: got %d", usagedomain.ErrOutputTokensExceeded, maxOutputTokens)
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
		return 0, fmt.Errorf("%w: estimated %d cents > limit %d cents", usagedomain.ErrSingleRequestBudgetExceeded, finalReserve, cfg.MaxSingleReserveCents)
	}
	return finalReserve, nil
}

// Reserve atomically checks pause, concurrency, daily limits, confirmed+reserved+unknown budgets, and rate limits.
// Rate limit quota is ONLY consumed if all preceding admission checks succeed.
func (s *Service) Reserve(ctx context.Context, userID string, purpose usagedomain.ReservationPurpose, estimatedCents int) (*usagedomain.Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.config
	if cfg.Paused {
		return nil, usagedomain.ErrAIPaused
	}
	if estimatedCents <= 0 {
		estimatedCents = 1
	}
	if estimatedCents > cfg.MaxSingleReserveCents {
		return nil, usagedomain.ErrSingleRequestBudgetExceeded
	}

	now := s.nowFunc()
	day, month := s.periodKeys(now)
	userKey := fmt.Sprintf("%s:%s", userID, day)

	if purpose == usagedomain.PurposeAgentChat {
		// 1. Concurrency check
		if s.platformActiveReqs >= cfg.PlatformConcurrency {
			return nil, usagedomain.ErrPlatformConcurrency
		}
		if s.userActiveReqs[userID] >= cfg.UserConcurrency {
			return nil, usagedomain.ErrUserConcurrencyLimit
		}

		// 2. Daily question limit check (confirmed + pending)
		if s.userDailyCount[userKey]+s.userDailyPendingCnt[userKey] >= cfg.UserDailyQuestions {
			return nil, usagedomain.ErrDailyLimitExceeded
		}

		// 3. User daily budget check (confirmed + reserved + unresolved unknown)
		userProjected := s.userDailyCost[userKey] + s.userDailyReserved[userKey] + s.userUnknownCents[userID] + estimatedCents
		if userProjected > cfg.UserDailyCents {
			return nil, usagedomain.ErrUserBudgetExceeded
		}

		// 4. Platform daily & monthly budget check (confirmed + reserved + unresolved unknown)
		platformDailyProjected := s.platformDailyCost[day] + s.platformDailyReserved[day] + s.platformUnknownCents + estimatedCents
		if platformDailyProjected > cfg.PlatformDailyCents {
			return nil, usagedomain.ErrPlatformBudget
		}
		platformMonthlyProjected := s.platformMonthlyCost[month] + s.platformMonthlyReserved[month] + s.platformUnknownCents + estimatedCents
		if platformMonthlyProjected > cfg.PlatformMonthlyCents {
			return nil, usagedomain.ErrPlatformMonthlyBudget
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
			return nil, usagedomain.ErrRateLimitExceeded
		}
		validTimers = append(validTimers, now)
		s.userRequestTimers[userID] = validTimers
	} else if purpose == usagedomain.PurposeIndexJob {
		// Background indexing: subject to ¥20/day sub-quota AND shares platform ¥100/day & ¥2000/month.
		bgProjected := s.bgDailyCost[day] + s.bgDailyReserved[day] + s.bgUnknownCents + estimatedCents
		if bgProjected > cfg.BackgroundDailyCents {
			return nil, usagedomain.ErrBackgroundBudgetExceeded
		}
		platformDailyProjected := s.platformDailyCost[day] + s.platformDailyReserved[day] + s.platformUnknownCents + estimatedCents
		if platformDailyProjected > cfg.PlatformDailyCents {
			return nil, usagedomain.ErrPlatformBudget
		}
		platformMonthlyProjected := s.platformMonthlyCost[month] + s.platformMonthlyReserved[month] + s.platformUnknownCents + estimatedCents
		if platformMonthlyProjected > cfg.PlatformMonthlyCents {
			return nil, usagedomain.ErrPlatformMonthlyBudget
		}
	}

	seq := s.seq.Add(1)
	resID := fmt.Sprintf("res_%d_%d", now.UnixNano(), seq)
	reservation := &usagedomain.Reservation{
		ID:            resID,
		UserID:        userID,
		Purpose:       purpose,
		PeriodDay:     day,
		PeriodMonth:   month,
		ReservedCents: estimatedCents,
		State:         usagedomain.StateReserved,
		ActualCents:   0,
		CreatedAt:     now.UTC(),
		UpdatedAt:     now.UTC(),
	}

	if s.store != nil {
		if err := s.store.InsertReservation(ctx, reservation); err != nil {
			return nil, err
		}
	}

	s.reservations[resID] = reservation
	s.platformDailyReserved[day] += estimatedCents
	s.platformMonthlyReserved[month] += estimatedCents
	if purpose == usagedomain.PurposeAgentChat {
		s.userDailyReserved[userKey] += estimatedCents
		s.userDailyPendingCnt[userKey]++
		s.userActiveReqs[userID]++
		s.platformActiveReqs++
	} else if purpose == usagedomain.PurposeIndexJob {
		s.bgDailyReserved[day] += estimatedCents
	}

	resCopy := *reservation
	return &resCopy, nil
}

// Settle finalizes a completed successful reservation with actual step costs.
func (s *Service) Settle(ctx context.Context, reservationID string, steps []usagedomain.SettlementStep) (*usagedomain.Reservation, error) {
	return s.SettleDetailed(ctx, usagedomain.SettleRequest{
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
	steps []usagedomain.SettlementStep,
	unknownCents int,
	unknownReason string,
) (*usagedomain.Reservation, error) {
	return s.SettleDetailed(ctx, usagedomain.SettleRequest{
		ReservationID: reservationID,
		Steps:         steps,
		CountSuccess:  false,
		UnknownCents:  unknownCents,
		UnknownReason: unknownReason,
	})
}

// SettleDetailed settles a reservation with explicit control over success count and unknown cost holds.
func (s *Service) SettleDetailed(ctx context.Context, req usagedomain.SettleRequest) (*usagedomain.Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, exists := s.reservations[req.ReservationID]
	if !exists {
		return nil, usagedomain.ErrReservationNotFound
	}
	if res.State != usagedomain.StateReserved {
		return nil, usagedomain.ErrInvalidReservation
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
	if res.Purpose == usagedomain.PurposeAgentChat {
		s.userDailyReserved[userKey] = maxZero(s.userDailyReserved[userKey] - res.ReservedCents)
		s.userDailyPendingCnt[userKey] = maxZero(s.userDailyPendingCnt[userKey] - 1)
		s.userActiveReqs[res.UserID] = maxZero(s.userActiveReqs[res.UserID] - 1)
		s.platformActiveReqs = maxZero(s.platformActiveReqs - 1)
		if req.CountSuccess {
			s.userDailyCount[userKey]++
		}
	} else if res.Purpose == usagedomain.PurposeIndexJob {
		s.bgDailyReserved[day] = maxZero(s.bgDailyReserved[day] - res.ReservedCents)
	}

	// Record confirmed incurred costs against the original admission period (never truncate overruns).
	if totalActualCents > 0 {
		s.platformDailyCost[day] += totalActualCents
		s.platformMonthlyCost[month] += totalActualCents
		if res.Purpose == usagedomain.PurposeAgentChat {
			s.userDailyCost[userKey] += totalActualCents
		} else if res.Purpose == usagedomain.PurposeIndexJob {
			s.bgDailyCost[day] += totalActualCents
		}
	}

	res.ActualCents = totalActualCents
	res.UnknownCents = unknownCents
	res.UpdatedAt = now

	var recItem *usagedomain.ReconciliationItem
	if unknownCents > 0 {
		dueAt := now.Add(24 * time.Hour)
		res.State = usagedomain.StatePendingReconciliation
		res.ReconcileDueAt = &dueAt

		s.platformUnknownCents += unknownCents
		if res.Purpose == usagedomain.PurposeAgentChat {
			s.userUnknownCents[res.UserID] += unknownCents
		} else if res.Purpose == usagedomain.PurposeIndexJob {
			s.bgUnknownCents += unknownCents
		}

		recID := fmt.Sprintf("rec_%d_%d", now.UnixNano(), s.seq.Add(1))
		recItem = &usagedomain.ReconciliationItem{
			ID:            recID,
			ReservationID: res.ID,
			UserID:        res.UserID,
			Purpose:       res.Purpose,
			PeriodDay:     day,
			PeriodMonth:   month,
			UnknownCents:  unknownCents,
			State:         usagedomain.ReconciliationPending,
			Reason:        req.UnknownReason,
			DueAt:         dueAt,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		s.reconciliations[recID] = recItem
	} else if totalActualCents == 0 && !req.CountSuccess {
		res.State = usagedomain.StateReleased
	} else if totalActualCents > res.ReservedCents {
		res.State = usagedomain.StateOverrun
	} else {
		res.State = usagedomain.StateSettled
	}

	s.steps[res.ID] = append([]usagedomain.SettlementStep(nil), req.Steps...)

	if s.store != nil {
		if err := s.store.PersistSettlement(ctx, res, req.Steps, recItem); err != nil {
			return nil, err
		}
	}

	resCopy := *res
	return &resCopy, nil
}

// Release cancels an active reservation when zero external billable calls occurred.
func (s *Service) Release(ctx context.Context, reservationID string) error {
	_, err := s.SettleDetailed(ctx, usagedomain.SettleRequest{
		ReservationID: reservationID,
		Steps:         nil,
		CountSuccess:  false,
		UnknownCents:  0,
	})
	return err
}

// ListReconciliations returns unknown-cost reconciliation items, automatically flagging items overdue past 24h for manual review.
func (s *Service) ListReconciliations(ctx context.Context) []*usagedomain.ReconciliationItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc().UTC()
	out := make([]*usagedomain.ReconciliationItem, 0, len(s.reconciliations))
	for _, item := range s.reconciliations {
		if item.State == usagedomain.ReconciliationPending && !now.Before(item.DueAt) {
			item.State = usagedomain.ReconciliationManualReview
			item.UpdatedAt = now
			if s.store != nil {
				_ = s.store.UpdateReconciliation(ctx, item)
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
func (s *Service) ResolveReconciliation(ctx context.Context, reconciliationID string, resolvedCents int, note string) (*usagedomain.ReconciliationItem, error) {
	if resolvedCents < 0 {
		return nil, errors.New("resolvedCents must be non-negative")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.reconciliations[reconciliationID]
	if !exists {
		return nil, usagedomain.ErrReconciliationNotFound
	}
	if item.State == usagedomain.ReconciliationResolved {
		return nil, usagedomain.ErrReconciliationResolved
	}

	now := s.nowFunc().UTC()

	// Release unknown risk hold
	s.platformUnknownCents = maxZero(s.platformUnknownCents - item.UnknownCents)
	if item.Purpose == usagedomain.PurposeAgentChat {
		s.userUnknownCents[item.UserID] = maxZero(s.userUnknownCents[item.UserID] - item.UnknownCents)
	} else if item.Purpose == usagedomain.PurposeIndexJob {
		s.bgUnknownCents = maxZero(s.bgUnknownCents - item.UnknownCents)
	}

	// Credit resolved confirmed amount back to the original admission period (day & month)
	if resolvedCents > 0 {
		s.platformDailyCost[item.PeriodDay] += resolvedCents
		s.platformMonthlyCost[item.PeriodMonth] += resolvedCents
		if item.Purpose == usagedomain.PurposeAgentChat {
			origUserKey := fmt.Sprintf("%s:%s", item.UserID, item.PeriodDay)
			s.userDailyCost[origUserKey] += resolvedCents
		} else if item.Purpose == usagedomain.PurposeIndexJob {
			s.bgDailyCost[item.PeriodDay] += resolvedCents
		}
	}

	if res, ok := s.reservations[item.ReservationID]; ok {
		res.ActualCents += resolvedCents
		res.UnknownCents = 0
		res.UpdatedAt = now
		if res.ActualCents > res.ReservedCents {
			res.State = usagedomain.StateOverrun
		} else {
			res.State = usagedomain.StateSettled
		}
		if s.store != nil {
			_ = s.store.UpdateReservation(ctx, res)
		}
	}

	item.ResolvedCents = resolvedCents
	item.ResolutionNote = note
	item.State = usagedomain.ReconciliationResolved
	item.ResolvedAt = &now
	item.UpdatedAt = now

	if s.store != nil {
		if err := s.store.UpdateReconciliation(ctx, item); err != nil {
			return nil, err
		}
	}

	cp := *item
	return &cp, nil
}

// GetSummary returns usage statistics for a user; when includePlatform is true (admin), full platform metrics & 80% alerts are included.
func (s *Service) GetSummary(ctx context.Context, userID string, includePlatform bool) *usagedomain.UsageSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc()
	day, month := s.periodKeys(now)
	userKey := fmt.Sprintf("%s:%s", userID, day)
	cfg := s.config

	summary := &usagedomain.UsageSummary{
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
			if item.State != usagedomain.ReconciliationResolved {
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
func (s *Service) GetReservation(reservationID string) (*usagedomain.Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, ok := s.reservations[reservationID]
	if !ok {
		return nil, usagedomain.ErrReservationNotFound
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

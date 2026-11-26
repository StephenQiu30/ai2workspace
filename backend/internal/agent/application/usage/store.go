package usage

import (
	"context"
	"fmt"

	usagedomain "github.com/StephenQiu30/ai2workspace/backend/internal/agent/domain/usage"
)

// StoredState contains the records restored when the use case starts.
type StoredState struct {
	Config          usagedomain.BudgetConfig
	Reservations    []*usagedomain.Reservation
	Reconciliations []*usagedomain.ReconciliationItem
}

// Store persists usage limits and settlement state.
type Store interface {
	Load(ctx context.Context, defaults usagedomain.BudgetConfig) StoredState
	SaveConfig(ctx context.Context, cfg usagedomain.BudgetConfig) error
	InsertReservation(ctx context.Context, r *usagedomain.Reservation) error
	UpdateReservation(ctx context.Context, r *usagedomain.Reservation) error
	PersistSettlement(ctx context.Context, r *usagedomain.Reservation, steps []usagedomain.SettlementStep, rec *usagedomain.ReconciliationItem) error
	UpdateReconciliation(ctx context.Context, item *usagedomain.ReconciliationItem) error
}

func (s *Service) restore(state StoredState) {
	s.config = state.Config
	for _, r := range state.Reservations {
		s.reservations[r.ID] = r
		userKey := fmt.Sprintf("%s:%s", r.UserID, r.PeriodDay)
		if r.ActualCents > 0 {
			s.platformDailyCost[r.PeriodDay] += r.ActualCents
			s.platformMonthlyCost[r.PeriodMonth] += r.ActualCents
			if r.Purpose == usagedomain.PurposeAgentChat {
				s.userDailyCost[userKey] += r.ActualCents
			} else if r.Purpose == usagedomain.PurposeIndexJob {
				s.bgDailyCost[r.PeriodDay] += r.ActualCents
			}
		}
		if r.Purpose == usagedomain.PurposeAgentChat && (r.State == usagedomain.StateSettled || r.State == usagedomain.StateOverrun) && r.ActualCents > 0 {
			s.userDailyCount[userKey]++
		}
	}
	for _, item := range state.Reconciliations {
		s.reconciliations[item.ID] = item
		if item.State != usagedomain.ReconciliationResolved {
			s.platformUnknownCents += item.UnknownCents
			if item.Purpose == usagedomain.PurposeAgentChat {
				s.userUnknownCents[item.UserID] += item.UnknownCents
			} else if item.Purpose == usagedomain.PurposeIndexJob {
				s.bgUnknownCents += item.UnknownCents
			}
		}
	}
}

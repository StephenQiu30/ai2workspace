package commerce

import (
	"context"

	entitlementspg "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/adapter/postgres/entitlements"
	entitlementsdomain "github.com/StephenQiu30/ai2workspace/backend/internal/commerce/domain/entitlements"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Grants provides read access to purchase entitlements for other modules.
type Grants struct {
	store *entitlementspg.PostgresStore
}

// NewGrants opens the commerce entitlement store on the application pool.
func NewGrants(pool *pgxpool.Pool) *Grants {
	return &Grants{store: entitlementspg.NewPostgresStore(pool)}
}

// CheckEntitlement checks the user's current access to a paid course.
func (g *Grants) CheckEntitlement(ctx context.Context, userID, courseID, purpose string) (bool, string, error) {
	allowed, _, reason, err := g.store.CheckEntitlement(ctx, userID, courseID, purpose)
	return allowed, reason, err
}

// ListActiveCourseIDs lists the user's currently purchased courses in grant order.
func (g *Grants) ListActiveCourseIDs(ctx context.Context, userID string) ([]string, error) {
	grants, err := g.store.ListUserEntitlements(ctx, userID)
	if err != nil {
		return nil, err
	}
	courses := make([]string, 0, len(grants))
	for _, grant := range grants {
		if grant.Status == entitlementsdomain.GrantActive {
			courses = append(courses, grant.CourseID)
		}
	}
	return courses, nil
}

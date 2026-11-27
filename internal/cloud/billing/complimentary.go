// SPDX-License-Identifier: FSL-1.1-MIT

package billing

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
)

var ErrComplimentaryEnd = errors.New("complimentary Pro must end in the future")

// GrantComplimentary is called only by the operator console.
func (s *Service) GrantComplimentary(ctx context.Context, accountID, actor string, end time.Time) (int64, error) {
	now := s.cfg.Now()
	if !strings.HasPrefix(actor, "admin:") || !end.After(now) {
		return 0, ErrComplimentaryEnd
	}
	id := "csub_" + rand.Text()
	var auditID int64
	err := s.db.Transaction(ctx, func(q *store.Queries) error {
		account, err := q.LockAuthAccount(ctx, accountID)
		if err != nil {
			return err
		}
		if account.State == "deleting" {
			return store.ErrAuthNotFound
		}
		if err := q.InsertComplimentaryGrant(ctx, id, accountID, actor, now, end); err != nil {
			return err
		}
		if err := q.ApplyAccountPlan(ctx, accountID, plan.Pro, now, true); err != nil {
			return err
		}
		auditID, err = q.AppendSecurityEventID(ctx, store.SecurityEvent{
			AccountID: accountID, Actor: actor, At: now, Type: "complimentary_pro_granted",
			Details: map[string]any{"subscription_id": id, "ends_at": end},
		})
		return err
	})
	return auditID, err
}

// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (q *Queries) GatePolicy(ctx context.Context) (remoteproto.GatePolicy, error) {
	row, err := q.generated.GetGatePolicy(ctx)
	if err != nil {
		return remoteproto.GatePolicy{}, fmt.Errorf("read cloud gate policy: %w", err)
	}
	policy := remoteproto.GatePolicy{Mode: remoteproto.GateMode(row.Mode), Message: row.Message}
	if row.Until.Valid {
		policy.Until = new(row.Until.Time.UTC())
	}
	return policy, nil
}

func (s *Store) SetGatePolicy(ctx context.Context, actor string, policy remoteproto.GatePolicy, now time.Time) error {
	_, err := s.ChangeGatePolicy(ctx, actor, policy, now)
	return err
}

func (s *Store) ChangeGatePolicy(
	ctx context.Context, actor string, policy remoteproto.GatePolicy, now time.Time,
) (int64, error) {
	email, admin := strings.CutPrefix(actor, "admin:")
	if !admin || strings.TrimSpace(email) == "" {
		return 0, errors.New("gate policy changes require an admin actor")
	}
	enforce := policy.Mode == remoteproto.GateEnforce && policy.Until == nil
	if len(policy.Message) > 2_048 || (!enforce && !policy.GraceAt(now)) {
		return 0, remoteproto.ErrGatePolicyInvalid
	}
	var eventID int64
	err := s.Transaction(ctx, func(q *Queries) error {
		params := sqlcgen.UpdateGatePolicyParams{
			Mode: string(policy.Mode), Message: policy.Message,
			UpdatedBy: actor, UpdatedAt: timestamp(now),
		}
		if policy.Until != nil {
			params.Until = pgtype.Timestamptz{Time: *policy.Until, Valid: true}
		}
		changed, err := q.generated.UpdateGatePolicy(ctx, params)
		if err != nil {
			return fmt.Errorf("update cloud gate policy: %w", err)
		}
		if changed == 0 {
			return nil
		}
		eventID, err = q.AppendSecurityEventID(ctx, SecurityEvent{
			Type: "gate_policy_changed", Actor: actor, At: now,
			Details: map[string]any{"mode": policy.Mode, "until": policy.Until},
		})
		return err
	})
	return eventID, err
}

// SPDX-License-Identifier: FSL-1.1-MIT

package store

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/cloud/store/sqlcgen"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (s *Store) RetiredSigningKeys(ctx context.Context) ([]string, error) {
	return s.generated.RetiredSigningKeys(ctx)
}

func (s *Store) PublishSigningKeys(ctx context.Context, retired string, now time.Time) error {
	return s.Transaction(ctx, func(q *Queries) error {
		if retired != "" {
			changed, err := q.generated.RetireSigningKey(ctx, sqlcgen.RetireSigningKeyParams{
				Kid: retired, RetiredAt: timestamp(now),
			})
			if err != nil {
				return err
			}
			if changed != 0 {
				if err := q.AppendSecurityEvent(ctx, SecurityEvent{
					Type: "cloud_signing_key_retired", Actor: "system", At: now,
					Details: map[string]any{"kid": retired},
				}); err != nil {
					return err
				}
			}
		}
		return q.NotifyControlEvent(ctx, ControlEvent{Type: remoteproto.ControlJWKS})
	})
}

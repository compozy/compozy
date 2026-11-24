// SPDX-License-Identifier: FSL-1.1-MIT

package cloud

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/cloud/backup"
	"github.com/compozy/compozy/internal/cloud/billing"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
)

func FinalizeRestore(ctx context.Context, cfg Config, bucket *backup.R2, point, through time.Time) error {
	now := cmp.Or(cfg.Now, time.Now)
	if point.IsZero() || !through.After(point) || through.After(now()) {
		return errors.New("restore archive cutoff must cover the restore point and be in the past")
	}
	if now().Sub(point) >= 30*24*time.Hour {
		return errors.New("restore point is outside the Stripe event history window")
	}
	db, err := store.Open(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.WithBackupRecoveryLock(ctx, func() error {
		return finalizeRestore(ctx, db, cfg, bucket, now, point, through)
	})
}

func finalizeRestore(
	ctx context.Context, db *store.Store, cfg Config, bucket *backup.R2, now func() time.Time, point, through time.Time,
) error {
	state, err := db.BeginRestore(ctx, point, through)
	if err != nil || state.Completed {
		return err
	}
	if !state.Applied {
		id, events, err := bucket.RestoreEvents(ctx, point, through)
		if err != nil {
			return fmt.Errorf("recover security archive: %w", err)
		}
		if err := db.ApplyRestore(ctx, point, now(), id, events); err != nil {
			return fmt.Errorf("apply restore security changes: %w", err)
		}
	}
	payments := billing.New(db, billing.Config{
		Stripe: cfg.Stripe, MonthlyPrice: cfg.MonthlyPrice, AnnualPrice: cfg.AnnualPrice, Now: now,
	})
	if err := payments.ReplayAfterRestore(ctx, point); err != nil {
		return fmt.Errorf("replay Stripe events: %w", err)
	}
	tunnels := control.NewService(
		db,
		control.ServiceConfig{HostedTunnels: cfg.HostedTunnels, Now: now, Logger: cfg.Logger},
	)
	if err := tunnels.SweepAfterRestore(ctx); err != nil {
		return fmt.Errorf("reconcile restored hosted tunnels: %w", err)
	}
	return db.FinishRestore(ctx, point, now())
}

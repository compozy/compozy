// SPDX-License-Identifier: FSL-1.1-MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/backup"
	"github.com/compozy/compozy/internal/cloud/store"
)

const envRestoreThrough = "COMPOZY_CLOUD_RESTORE_ARCHIVE_THROUGH"

func archiveBucket(getenv func(string) string, environment string) (*backup.R2, error) {
	endpoint := getenv("COMPOZY_CLOUD_R2_ENDPOINT")
	if !strings.HasPrefix(endpoint, "https://") && environment != development {
		return nil, errors.New("COMPOZY_CLOUD_R2_ENDPOINT requires HTTPS")
	}
	return backup.NewR2(backup.R2Config{
		Endpoint: endpoint, Bucket: getenv("COMPOZY_CLOUD_R2_BACKUP_BUCKET"),
		AccessKey: getenv("COMPOZY_CLOUD_R2_ACCESS_KEY_ID"), SecretKey: getenv("COMPOZY_CLOUD_R2_SECRET_ACCESS_KEY"),
	})
}

func shipEvents(ctx context.Context, getenv func(string) string, out io.Writer) error {
	environment, err := cloudEnvironment(getenv)
	if err != nil {
		return err
	}
	direct, err := databaseURL(getenv, envNeonDirect, environment)
	if err != nil {
		return err
	}
	bucket, err := archiveBucket(getenv, environment)
	if err != nil {
		return err
	}
	db, err := store.Open(ctx, store.Config{PooledDSN: direct, DirectDSN: direct})
	if err != nil {
		return err
	}
	defer db.Close()
	through, err := backup.ShipSecurityEvents(ctx, bucket, db)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, through.Format(time.RFC3339Nano))
	return err
}

func restoreFinalize(
	ctx context.Context, value string, getenv func(string) string, out io.Writer, logger *slog.Logger,
) error {
	point, err := parseRestoreTime(value)
	if err != nil {
		return fmt.Errorf("--restore-point: %w", err)
	}
	through, err := parseRestoreTime(getenv(envRestoreThrough))
	if err != nil {
		return fmt.Errorf("%s: %w", envRestoreThrough, err)
	}
	environment, err := cloudEnvironment(getenv)
	if err != nil {
		return err
	}
	direct, err := databaseURL(getenv, envNeonDirect, environment)
	if err != nil {
		return err
	}
	bucket, err := archiveBucket(getenv, environment)
	if err != nil {
		return err
	}
	var stripeOrigin string
	if environment == development {
		stripeOrigin = getenv("COMPOZY_CLOUD_STRIPE_API_ORIGIN")
	}
	stripe, err := loadStripeAt(getenv, stripeOrigin)
	if err != nil {
		return err
	}
	tunnels, err := loadHostedTunnels(getenv, environment)
	if err != nil {
		return err
	}
	err = cloud.FinalizeRestore(ctx, cloud.Config{
		Postgres:     store.Config{PooledDSN: direct, DirectDSN: direct},
		MonthlyPrice: getenv("STRIPE_PRICE_PRO_MONTHLY"), AnnualPrice: getenv("STRIPE_PRICE_PRO_ANNUAL"),
		Stripe: stripe, HostedTunnels: tunnels, Logger: logger,
	}, bucket, point, through)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Cloud restore finalized for %s\n", point.Format(time.RFC3339Nano))
	return err
}

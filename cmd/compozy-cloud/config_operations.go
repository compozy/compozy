// SPDX-License-Identifier: FSL-1.1-MIT

package main

import (
	"cmp"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/observe"
)

const (
	envGrafanaURL      = "COMPOZY_CLOUD_GRAFANA_PROM_URL"
	envGrafanaUser     = "COMPOZY_CLOUD_GRAFANA_PROM_USER"
	envGrafanaToken    = "COMPOZY_CLOUD_GRAFANA_PROM_TOKEN"
	envSentryDSN       = "COMPOZY_CLOUD_SENTRY_DSN"
	envInstance        = "COMPOZY_CLOUD_INSTANCE"
	envExpectedRestore = "COMPOZY_CLOUD_EXPECTED_RESTORE_POINT"
)

var cloudRevision = "dev"

func configureOperations(cfg *cloud.Config, env string, getenv func(string) string) error {
	var err error
	cfg.Revision = cloudRevision
	if dsn := getenv(envSentryDSN); dsn != "" {
		cfg.Reporter, err = observe.NewSentry(dsn, env)
		if err != nil {
			return err
		}
	}
	endpoint, user, token := getenv(envGrafanaURL), getenv(envGrafanaUser), getenv(envGrafanaToken)
	if cmp.Or(endpoint, user, token) != "" {
		cfg.Publisher, err = observe.NewPublisher(endpoint, user, token, env, getenv(envInstance))
		if err != nil {
			return err
		}
	}
	if point := getenv(envExpectedRestore); point != "" {
		cfg.ExpectedRestore, err = parseRestoreTime(point)
		if err != nil {
			return fmt.Errorf("%s: %w", envExpectedRestore, err)
		}
	}
	return nil
}

func parseRestoreTime(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("use an RFC 3339 timestamp")
	}
	return t.UTC().Truncate(time.Microsecond), nil
}

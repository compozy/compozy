//go:build integration

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const gatewayUpgrade = "gateway-upgrade"

func prepareGatewayUpgrade(f *fixture, db *remotetest.Database) scenarioManifest {
	f.Helper()
	cloudURL := startCloud(f, db, "", nil).Server.URL
	controller := prepareDaemon(f, false)
	paths, err := config.ResolveHomePathsFrom(controller.Home)
	if err != nil {
		f.Fatal(err)
	}
	seedGatewayUpgrade(f, paths.DatabaseFile)
	startDaemon(f, controller, cloudURL, signInRequiredEnv+"=1")
	assertGatewayCommandRetired(f, controller, cloudURL)
	controller.DeviceID = signIn(f, controller)
	cliJSON[contract.OnboardingStatusResponse](f, controller, cloudURL, "onboarding", "complete")
	return scenarioManifest{
		Scenario: gatewayUpgrade, CloudURL: cloudURL, Controller: controller,
		Email: fixtureEmail, Computers: []computerTarget{},
	}
}

func seedGatewayUpgrade(f *fixture, filename string) {
	f.Helper()
	ctx := f.Context()
	db, err := remotetest.OpenLegacyGlobalDB(ctx, filename)
	if err != nil {
		f.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			f.Error(err)
		}
	}()
	const at = "2026-10-10T00:00:00Z"
	if _, err := db.ExecContext(ctx, `INSERT INTO gateway_device_sessions
		(id, name, token_hash, actor_kind, pairing_origin, created_at)
		VALUES ('old-device', 'Old device', ?, 'operator_device', 'local', ?)`, strings.Repeat("a", 64), at); err != nil {
		f.Fatal(err)
	}
	// The old public address changed after both webhooks were confirmed at generation 3.
	if _, err := db.ExecContext(ctx, `INSERT INTO gateway_endpoint_state
		(tier, endpoint_url, endpoint_generation, reachable, updated_at)
		VALUES ('public', 'https://old-hooks.example.test', 4, 1, ?)`, at); err != nil {
		f.Fatal(err)
	}
	for _, slug := range []string{"deploy", "release"} {
		id := "legacy-" + slug
		spec := fmt.Sprintf(`{"id":%q,"event":"webhook","scope":"global","endpoint_slug":%q,"webhook_id":%q}`,
			id, slug, "wbh_"+slug)
		if _, err := db.ExecContext(ctx, `INSERT INTO resource_records
			(kind, id, version, scope_kind, owner_kind, owner_id, source_kind, source_id, spec_json, created_at, updated_at)
			VALUES ('automation.trigger', ?, 1, 'user', 'operator', 'fixture', 'dynamic', 'fixture', ?, ?, ?)`,
			id, spec, at, at); err != nil {
			f.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO gateway_ingress_bindings
			(subject_kind, subject_id, scope_kind, endpoint_generation, confirmed_at)
			VALUES ('webhook_trigger', ?, 'global', 3, ?)`, id, at); err != nil {
			f.Fatal(err)
		}
	}
}

func assertGatewayCommandRetired(f *fixture, controller daemonInstance, cloudURL string) {
	f.Helper()
	ctx, cancel := context.WithTimeout(f.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Getenv(daemonBinaryEnv), "gateway", "status")
	cmd.Env, cmd.Dir = controller.env(cloudURL), controller.OperatorHome
	output, err := cmd.CombinedOutput()
	exit, ok := errors.AsType[*exec.ExitError](err)
	const want = "compozy gateway was removed in 0.8.0; use compozy computer — see https://compozy.com/docs/computers"
	if !ok || exit.ExitCode() != 2 || strings.TrimSpace(string(output)) != want {
		f.Fatalf("compozy gateway status: %v, output %q", err, output)
	}
}

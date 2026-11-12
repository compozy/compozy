package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/testutil"
	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/spf13/cobra"
)

func TestDaemonBootstrapPolicy(t *testing.T) {
	t.Run("Should prefer attach then start then provision", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			probe bootstrapProbe
			want  bootstrapResolution
		}{
			{
				name:  "Should attach to a healthy runtime",
				probe: bootstrapProbe{Healthy: true, Installed: true},
				want:  bootstrapResolutionAttach,
			},
			{
				name:  "Should start an installed runtime",
				probe: bootstrapProbe{Installed: true},
				want:  bootstrapResolutionStart,
			},
			{
				name:  "Should provision an empty runtime home",
				probe: bootstrapProbe{},
				want:  bootstrapResolutionProvision,
			},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if got := resolveBootstrapAction(test.probe); got != test.want {
					t.Fatalf("resolveBootstrapAction() = %q, want %q", got, test.want)
				}
			})
		}
	})

	t.Run("Should classify every readiness observation", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			probe bootstrapProbe
			want  bootstrapProbeClass
		}{
			{
				name:  "Should classify a healthy runtime",
				probe: bootstrapProbe{Healthy: true},
				want:  bootstrapProbeHealthy,
			},
			{
				name:  "Should classify an unhealthy listener",
				probe: bootstrapProbe{Listening: true},
				want:  bootstrapProbeListeningUnhealthy,
			},
			{
				name:  "Should classify a stale record",
				probe: bootstrapProbe{RecordPresent: true},
				want:  bootstrapProbeStaleRecord,
			},
			{
				name:  "Should classify a port conflict",
				probe: bootstrapProbe{PortOccupied: true},
				want:  bootstrapProbePortConflict,
			},
			{name: "Should classify an unavailable runtime", probe: bootstrapProbe{}, want: bootstrapProbeUnavailable},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				if got := classifyBootstrapProbe(test.probe); got != test.want {
					t.Fatalf("classifyBootstrapProbe(%#v) = %q, want %q", test.probe, got, test.want)
				}
			})
		}
	})

	t.Run("Should back off from 500 milliseconds and give up after five unreadied starts", func(t *testing.T) {
		t.Parallel()

		want := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
		for attempt, expected := range want {
			t.Run(fmt.Sprintf("Should use backoff step %d", attempt+1), func(t *testing.T) {
				t.Parallel()
				if got := bootstrapBackoff(attempt + 1); got != expected {
					t.Fatalf("bootstrapBackoff(%d) = %s, want %s", attempt+1, got, expected)
				}
			})
		}
		if bootstrapShouldGiveUp(4) || !bootstrapShouldGiveUp(5) {
			t.Fatal("bootstrapShouldGiveUp() did not enforce the five-attempt boundary")
		}
	})

	t.Run("Should refuse a parallel retry attempt", func(t *testing.T) {
		t.Parallel()

		gate := &bootstrapAttemptGate{}
		finish, err := gate.Begin()
		if err != nil {
			t.Fatalf("Begin(first) error = %v", err)
		}
		if _, err := gate.Begin(); !errors.Is(err, errBootstrapAttemptInFlight) {
			t.Fatalf("Begin(parallel) error = %v, want in-flight refusal", err)
		}
		finish()
		finishAgain, err := gate.Begin()
		if err != nil {
			t.Fatalf("Begin(after finish) error = %v", err)
		}
		finishAgain()
	})
}

func TestDaemonBootstrapCompatibility(t *testing.T) {
	t.Run("Should refuse runtimes below the shell minimum", func(t *testing.T) {
		t.Parallel()

		err := validateBootstrapCompatibility(DaemonStatus{
			Version: "v1.0.0", MinAppVersion: "v1.0.0",
		}, ">=1.1.0", "v1.1.0")
		assertErrorContains(t, err, "repair or update the runtime")
		failure, failureOK := errors.AsType[*bootstrapCompatibilityFailure](err)
		if !failureOK || failure.Reason != "runtime_below_minimum" ||
			failure.Runtime != "v1.0.0" || failure.Needed != ">=1.1.0" {
			t.Fatalf("compatibility failure = %#v, want typed runtime skew", failure)
		}
	})

	t.Run("Should refuse runtimes requiring a newer app", func(t *testing.T) {
		t.Parallel()

		err := validateBootstrapCompatibility(DaemonStatus{
			Version: "v1.2.0", MinAppVersion: "v1.2.0",
		}, ">=1.0.0", "v1.1.0")
		assertErrorContains(t, err, "repair the desktop app")
		failure, failureOK := errors.AsType[*bootstrapCompatibilityFailure](err)
		if !failureOK || failure.Reason != "app_below_minimum" ||
			failure.Runtime != "v1.2.0" || failure.Needed != "v1.2.0" {
			t.Fatalf("compatibility failure = %#v, want typed app skew", failure)
		}
	})
}

func TestDesktopRuntimeBootstrapReplacement(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name           string
		status         DaemonStatus
		replaced       bool
		bundleRequired bool
		missingBundle  bool
		restartError   bool
		customSocket   bool
		migration      string
	}{
		{
			name:     "Should restart a stale desktop-owned runtime through the recovery owner",
			status:   DaemonStatus{PID: 4242, Version: "0.3.0-beta.20"},
			replaced: true,
		},
		{
			name:     "Should preserve a newer desktop-owned runtime that accepts the app",
			status:   DaemonStatus{PID: 4242, Version: "0.3.0", MinAppVersion: "0.3.0-beta.21"},
			replaced: false,
		},
		{
			name:           "Should replace a newer desktop-owned runtime that rejects the app with the bundle",
			status:         DaemonStatus{PID: 4242, Version: "0.3.0", MinAppVersion: "0.3.0"},
			replaced:       true,
			bundleRequired: true,
		},
		{
			name:   "Should preserve live work when the replacement payload cannot be read",
			status: DaemonStatus{PID: 4242, Version: "0.3.0-beta.20"}, missingBundle: true,
		},
		{
			name:   "Should retain recovery evidence without a raw stop when coordinated restart fails",
			status: DaemonStatus{PID: 4242, Version: "0.3.0-beta.20"}, restartError: true,
		},
		{
			name:      "Should keep waiting while a slow replacement migration advances",
			status:    DaemonStatus{PID: 4242, Version: "0.3.0-beta.20"},
			replaced:  true,
			migration: "advancing",
		},
		{
			name:      "Should fail and retain recovery evidence when the replacement migration stalls",
			status:    DaemonStatus{PID: 4242, Version: "0.3.0-beta.20"},
			migration: "stalled",
		},
		{
			name:   "Should restart the configured local socket even with a remote connection selected",
			status: DaemonStatus{PID: 4242, Version: "0.3.0-beta.20"}, replaced: true, customSocket: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client := &stubClient{}
			deps := newTestDeps(t, client).withDefaults()
			homePaths, err := deps.resolveHome()
			if err != nil {
				t.Fatalf("resolveHome() error = %v", err)
			}
			if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
				t.Fatalf("EnsureHomeLayout() error = %v", err)
			}
			socketPath := homePaths.DaemonSocket
			if test.customSocket {
				socketPath = filepath.Join(filepath.Dir(homePaths.DaemonSocket), "custom.sock")
				deps.loadConfig = func() (compozyconfig.Config, error) {
					cfg := compozyconfig.DefaultWithHome(homePaths)
					cfg.Daemon.Socket = socketPath
					cfg.Gateway.ActiveConnection = "remote"
					return cfg, nil
				}
			}
			installedPath := bootstrapRuntimePath(homePaths)
			bundlePath := filepath.Join(t.TempDir(), "bundled-compozy")
			writeFile(t, installedPath, "current-runtime")
			if !test.missingBundle {
				writeFile(t, bundlePath, "replacement-runtime")
			}
			cmd := &cobra.Command{}
			var events bytes.Buffer
			cmd.SetOut(&events)
			cmd.Flags().StringP("output", "o", "jsonl", "")
			cmd.SetContext(testutil.Context(t))
			execution := bootstrapExecution{
				cmd:            cmd,
				deps:           deps,
				options:        bootstrapOptions{appVersion: "0.3.0-beta.21", channel: "beta"},
				homePaths:      homePaths,
				bundlePath:     bundlePath,
				installedPath:  installedPath,
				runtimeVersion: "0.3.0-beta.21",
			}
			if err := compozyupdate.WriteDesktopProvenance(
				homePaths,
				installedPath,
				execution.provenance(),
			); err != nil {
				t.Fatalf("WriteDesktopProvenance() error = %v", err)
			}
			writeFile(t, homePaths.DaemonInfo, `{"pid":4242,"started_at":"2026-04-03T12:00:00Z"}`)
			restarts := 0
			execution.deps.processAlive = func(int) bool { return true }
			execution.deps.processMatchesStartTime = func(int, time.Time) bool { return true }
			execution.deps.signalProcess = func(_ int, signal syscall.Signal) error {
				t.Errorf("bootstrap must not cancel active work with raw signal %v", signal)
				return errors.New("unexpected raw stop")
			}
			execution.deps.newClient = func(target ClientTarget) (DaemonClient, error) {
				if target != LocalClientTarget(socketPath) {
					t.Fatalf("restart target = %+v, want local bootstrap daemon", target)
				}
				return client, nil
			}
			client.triggerSettingsRestartFn = func(context.Context) (SettingsRestartActionRecord, error) {
				restarts++
				contents, err := os.ReadFile(installedPath)
				if err != nil || string(contents) != "replacement-runtime" {
					t.Fatalf("runtime at restart = %q, %v; want provisioned replacement", contents, err)
				}
				if test.restartError {
					return SettingsRestartActionRecord{}, errors.New("restart rejected")
				}
				return SettingsRestartActionRecord{OperationID: "bootstrap-restart"}, nil
			}
			statuses := bootstrapRestartStatuses(test.migration)
			polls := 0
			client.getSettingsRestartStatusFn = func(context.Context, string) (SettingsRestartStatusRecord, error) {
				status := statuses[min(polls, len(statuses)-1)]
				polls++
				return status, nil
			}
			execution.deps.pollInterval = time.Millisecond
			if test.migration == "stalled" {
				// Idleness is measured on the observer clock, so the stall needs a moving one.
				execution.deps.now = time.Now
				execution.deps.restartMigrationIdleTimeout = 20 * time.Millisecond
			}

			// A started runtime that rejects the app asks for one re-resolution, never a second.
			if restart := execution.restartRejectingDesktopRuntime(test.status, true); restart != test.bundleRequired {
				t.Fatalf(
					"restartRejectingDesktopRuntime() before replacement = %t, want %t",
					restart, test.bundleRequired,
				)
			}
			replaced, err := execution.replaceOutdatedDesktopRuntime(test.status)
			if test.missingBundle || test.restartError || test.migration == "stalled" {
				if err == nil || replaced {
					t.Fatalf("failed replacement = %t, %v", replaced, err)
				}
				if test.missingBundle {
					contents, readErr := os.ReadFile(installedPath)
					if restarts != 0 || readErr != nil || string(contents) != "current-runtime" {
						t.Fatalf(
							"failed provisioning mutated runtime: restarts=%d, binary=%q, err=%v",
							restarts,
							contents,
							readErr,
						)
					}
				} else {
					if test.restartError {
						assertErrorContains(t, err, "restart rejected")
					} else {
						assertErrorContains(t, err, "stopped progressing at 1024 of 2048 rows")
					}
					copies, globErr := filepath.Glob(filepath.Join(homePaths.BinDir, ".compozy-bootstrap-backup-*"))
					if restarts != 1 || globErr != nil || len(copies) != 1 {
						t.Fatalf(
							"restart failure lost recovery copy: restarts=%d, copies=%v, err=%v",
							restarts,
							copies,
							globErr,
						)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("replaceOutdatedDesktopRuntime() error = %v", err)
			}
			if replaced != test.replaced || execution.bundleRequired != test.bundleRequired {
				t.Fatalf(
					"replaced = %t, bundleRequired = %t; want %t, %t",
					replaced, execution.bundleRequired, test.replaced, test.bundleRequired,
				)
			}
			wantRestarts := 0
			if test.replaced {
				wantRestarts = 1
			}
			if restarts != wantRestarts {
				t.Fatalf("restart requests = %d, want %d", restarts, wantRestarts)
			}
			copies, err := filepath.Glob(filepath.Join(homePaths.BinDir, ".compozy-bootstrap-backup-*"))
			if err != nil || len(copies) != 0 {
				t.Fatalf("successful replacement left recovery copies %v, %v", copies, err)
			}
			if test.migration == "advancing" {
				assertBootstrapMigrationEvents(t, events.String(), []int{0, 0, 50, 100})
				assertBootstrapProvisionVersions(t, events.String(), test.status.Version)
			}
			if execution.restartRejectingDesktopRuntime(test.status, true) {
				t.Fatal("restartRejectingDesktopRuntime() after replacement = true, want no second restart")
			}
		})
	}
}

func TestProvisionBundledRuntime(t *testing.T) {
	if installedPath := os.Getenv("COMPOZY_BOOTSTRAP_PATH_PROBE"); installedPath != "" {
		probeRunningBootstrapReplacement(t, installedPath)
		return
	}
	t.Run("Should preserve the executable path seen by an older running daemon", func(t *testing.T) {
		t.Parallel()
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		payload, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		homePaths, err := compozyconfig.ResolveHomePathsFrom(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatal(err)
		}
		installedPath := bootstrapRuntimePath(homePaths)
		if err := os.WriteFile(installedPath, payload, 0o700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), installedPath, "-test.run=^TestProvisionBundledRuntime$")
		cmd.Env = append(os.Environ(), "COMPOZY_BOOTSTRAP_PATH_PROBE="+installedPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("running runtime replacement: %v\n%s", err, output)
		}
	})

	t.Run("Should record the desktop release identity for a provisioned runtime", func(t *testing.T) {
		t.Parallel()

		homePaths := mustTestHomePaths(t)
		source := filepath.Join(t.TempDir(), "bundled-compozy")
		installedPath := filepath.Join(homePaths.BinDir, bootstrapBinaryName)
		writeFile(t, source, "bundled-runtime")
		cmd := &cobra.Command{}
		cmd.Flags().StringP("output", "o", "jsonl", "")
		cmd.SetOut(&bytes.Buffer{})
		err := provisionBootstrapRuntime(cmd, &bootstrapProvisionRequest{
			resolution:    bootstrapResolutionProvision,
			homePaths:     homePaths,
			bundlePath:    source,
			installedPath: installedPath,
			provenance: compozyupdate.DesktopProvenanceMetadata{
				AppVersion: "1.0.0-beta.1", Channel: "beta", RuntimeVersion: "1.0.0-beta.1",
			},
		})
		if err != nil {
			t.Fatalf("provisionBootstrapRuntime() error = %v", err)
		}
		markerBytes, err := os.ReadFile(homePaths.DesktopProvenanceFile)
		if err != nil {
			t.Fatalf("ReadFile(provenance) error = %v", err)
		}
		var marker map[string]string
		if err := json.Unmarshal(markerBytes, &marker); err != nil {
			t.Fatalf("json.Unmarshal(provenance) error = %v", err)
		}
		for key, want := range map[string]string{
			"installed_by": "desktop-app", "app_version": "1.0.0-beta.1",
			"channel": "beta", "runtime_version": "1.0.0-beta.1",
		} {
			if marker[key] != want {
				t.Fatalf("provenance[%q] = %q, want %q", key, marker[key], want)
			}
		}
	})

	t.Run("Should remove a provisioned runtime when provenance publication fails", func(t *testing.T) {
		t.Parallel()

		homePaths := mustTestHomePaths(t)
		source := filepath.Join(t.TempDir(), "bundled-compozy")
		installedPath := filepath.Join(homePaths.BinDir, bootstrapBinaryName)
		writeFile(t, source, "bundled-runtime")
		cmd := &cobra.Command{}
		cmd.Flags().StringP("output", "o", "jsonl", "")
		cmd.SetOut(&bytes.Buffer{})
		err := provisionBootstrapRuntime(cmd, &bootstrapProvisionRequest{
			resolution:    bootstrapResolutionProvision,
			homePaths:     homePaths,
			bundlePath:    source,
			installedPath: installedPath,
			provenance:    compozyupdate.DesktopProvenanceMetadata{Channel: "beta"},
		})
		assertErrorContains(t, err, "app version is required")
		if _, statErr := os.Stat(installedPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(provisioned runtime) error = %v, want removed runtime", statErr)
		}
	})

	t.Run("Should replace a desktop-owned runtime and publish the new release identity", func(t *testing.T) {
		t.Parallel()

		homePaths := mustTestHomePaths(t)
		installedPath := filepath.Join(homePaths.BinDir, bootstrapBinaryName)
		bundlePath := filepath.Join(t.TempDir(), "bundled-compozy")
		writeFile(t, installedPath, "old-runtime")
		writeFile(t, bundlePath, "new-runtime")
		if err := compozyupdate.WriteDesktopProvenance(
			homePaths,
			installedPath,
			compozyupdate.DesktopProvenanceMetadata{
				AppVersion: "0.3.0-beta.20", Channel: "beta", RuntimeVersion: "0.3.0-beta.20",
			},
		); err != nil {
			t.Fatalf("WriteDesktopProvenance(old runtime) error = %v", err)
		}
		metadata := compozyupdate.DesktopProvenanceMetadata{
			AppVersion: "0.3.0-beta.21", Channel: "beta", RuntimeVersion: "0.3.0-beta.21",
		}
		cmd := &cobra.Command{}
		cmd.Flags().StringP("output", "o", "jsonl", "")
		var events bytes.Buffer
		cmd.SetOut(&events)
		if err := provisionBootstrapRuntime(cmd, &bootstrapProvisionRequest{
			resolution:    bootstrapResolutionProvision,
			homePaths:     homePaths,
			bundlePath:    bundlePath,
			installedPath: installedPath,
			provenance:    metadata,
		}); err != nil {
			t.Fatalf("provisionBootstrapRuntime(replacement) error = %v", err)
		}
		// A stopped runtime has no status to ask; its recorded release names the transition.
		assertBootstrapProvisionVersions(t, events.String(), "0.3.0-beta.20")
		installed, err := os.ReadFile(installedPath)
		if err != nil {
			t.Fatalf("ReadFile(replaced runtime) error = %v", err)
		}
		if got, want := string(installed), "new-runtime"; got != want {
			t.Fatalf("replaced runtime = %q, want %q", got, want)
		}
		matches, err := compozyupdate.DesktopRuntimeMatchesBundle(
			homePaths,
			installedPath,
			bundlePath,
			metadata,
		)
		if err != nil {
			t.Fatalf("DesktopRuntimeMatchesBundle() error = %v", err)
		}
		if !matches {
			t.Fatal("DesktopRuntimeMatchesBundle() = false, want current bundled runtime")
		}
		assertNoBootstrapBackups(t, installedPath)
	})

	t.Run("Should restore the previous runtime when replacement provenance fails", func(t *testing.T) {
		t.Parallel()

		homePaths := mustTestHomePaths(t)
		installedPath := filepath.Join(homePaths.BinDir, bootstrapBinaryName)
		bundlePath := filepath.Join(t.TempDir(), "bundled-compozy")
		writeFile(t, installedPath, "old-runtime")
		writeFile(t, bundlePath, "new-runtime")
		if err := compozyupdate.WriteDesktopProvenance(
			homePaths,
			installedPath,
			compozyupdate.DesktopProvenanceMetadata{
				AppVersion: "0.3.0-beta.20", Channel: "beta", RuntimeVersion: "0.3.0-beta.20",
			},
		); err != nil {
			t.Fatalf("WriteDesktopProvenance(old runtime) error = %v", err)
		}
		cmd := &cobra.Command{}
		cmd.Flags().StringP("output", "o", "jsonl", "")
		cmd.SetOut(&bytes.Buffer{})
		err := provisionBootstrapRuntime(cmd, &bootstrapProvisionRequest{
			resolution:    bootstrapResolutionProvision,
			homePaths:     homePaths,
			bundlePath:    bundlePath,
			installedPath: installedPath,
			provenance:    compozyupdate.DesktopProvenanceMetadata{Channel: "beta"},
		})
		assertErrorContains(t, err, "app version is required")
		installed, readErr := os.ReadFile(installedPath)
		if readErr != nil {
			t.Fatalf("ReadFile(restored runtime) error = %v", readErr)
		}
		if got, want := string(installed), "old-runtime"; got != want {
			t.Fatalf("restored runtime = %q, want %q", got, want)
		}
		if !compozyupdate.RuntimeOwnedByDesktopApp(homePaths, installedPath) {
			t.Fatal("RuntimeOwnedByDesktopApp(restored runtime) = false, want previous ownership")
		}
		assertNoBootstrapBackups(t, installedPath)
	})

	t.Run("Should publish exactly one complete executable under concurrent first runs", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		source := filepath.Join(dir, "bundled-compozy")
		target := filepath.Join(dir, "home", "bin", "compozy")
		payload := []byte("complete-bundled-runtime")
		if err := os.WriteFile(source, payload, 0o700); err != nil {
			t.Fatalf("WriteFile(source) error = %v", err)
		}
		var workers sync.WaitGroup
		errs := make(chan error, 2)
		for range 2 {
			workers.Go(func() { errs <- provisionBundledRuntime(source, target) })
		}
		workers.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("provisionBundledRuntime() error = %v", err)
			}
		}
		got, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("ReadFile(target) error = %v", err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("provisioned payload = %q, want complete payload", got)
		}
		matches, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".compozy-bootstrap-*"))
		if err != nil {
			t.Fatalf("Glob(temp files) error = %v", err)
		}
		if len(matches) != 0 {
			t.Fatalf("provision temp files = %v, want clean directory", matches)
		}
	})

	t.Run("Should leave no executable when the bundle is missing", func(t *testing.T) {
		t.Parallel()

		target := filepath.Join(t.TempDir(), "home", "bin", "compozy")
		assertErrorContains(
			t,
			provisionBundledRuntime(filepath.Join(t.TempDir(), "missing"), target),
			"bundled runtime payload",
		)
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(target) error = %v, want no partial executable", err)
		}
	})

	t.Run("Should reject a symlinked installed runtime", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		source := filepath.Join(dir, "bundled-compozy")
		external := filepath.Join(dir, "external-compozy")
		target := filepath.Join(dir, "home", "bin", "compozy")
		writeFile(t, source, "bundled")
		writeFile(t, external, "external")
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatalf("MkdirAll(target parent) error = %v", err)
		}
		if err := os.Symlink(external, target); err != nil {
			t.Fatalf("Symlink(target) error = %v", err)
		}

		assertErrorContains(t, provisionBundledRuntime(source, target), "publish provisioned runtime")
		if regularFileExists(target) {
			t.Fatal("regularFileExists(symlink) = true, want symlink rejection")
		}
	})
}

func probeRunningBootstrapReplacement(t *testing.T, installedPath string) {
	t.Helper()
	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Dir(filepath.Dir(installedPath)))
	if err != nil {
		t.Fatal(err)
	}
	metadata := compozyupdate.DesktopProvenanceMetadata{
		AppVersion: "0.3.0", Channel: "stable", RuntimeVersion: "0.3.0",
	}
	if err := compozyupdate.WriteDesktopProvenance(homePaths, installedPath, metadata); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Stat(installedPath)
	if err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(t.TempDir(), "bundled-compozy")
	writeFile(t, bundlePath, "new-runtime")
	install, err := installBootstrapRuntime(&bootstrapProvisionRequest{
		homePaths: homePaths, installedPath: installedPath, bundlePath: bundlePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Released daemons resolve their executable at restart time, after this replacement.
	got, err := os.Executable()
	if err != nil || got != installedPath {
		t.Fatalf("executable after replacement = %q, %v; want %q", got, err, installedPath)
	}
	payload, err := os.ReadFile(installedPath)
	if err != nil || string(payload) != "new-runtime" {
		t.Fatalf("installed runtime = %q, %v; want new runtime", payload, err)
	}
	backup, err := os.Stat(install.backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(previous, backup) {
		t.Fatal("recovery copy no longer holds the previous runtime")
	}
}

func assertNoBootstrapBackups(t *testing.T, installedPath string) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(filepath.Dir(installedPath), ".compozy-bootstrap-backup-*"))
	if err != nil {
		t.Fatalf("Glob(bootstrap backups) error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("bootstrap backups = %v, want none", matches)
	}
}

func TestResolveBootstrapRuntime(t *testing.T) {
	t.Run("Should prefer an operator runtime before provisioning", func(t *testing.T) {
		t.Parallel()

		homePaths := mustTestHomePaths(t)
		operatorPath := filepath.Join(t.TempDir(), bootstrapBinaryName)
		writeFile(t, operatorPath, "operator")
		deps := newTestDeps(t, &stubClient{})
		deps.lookPath = func(name string) (string, error) {
			if name != bootstrapBinaryName {
				t.Fatalf("lookPath(%q), want %q", name, bootstrapBinaryName)
			}
			return operatorPath, nil
		}

		path, owned := resolveBootstrapRuntime(deps, homePaths, bootstrapRuntimePath(homePaths))
		if path != operatorPath || owned {
			t.Fatalf("resolveBootstrapRuntime() = (%q, %t), want operator path with owned=false", path, owned)
		}
	})
}

func TestProbeBootstrapDaemonUsesLocalIdentity(t *testing.T) {
	for _, test := range []struct {
		name         string
		pid          int
		customSocket bool
		healthy      bool
	}{
		{name: "Should reject a status response that does not match daemon.json", pid: 99},
		{name: "Should verify the configured local socket despite a selected remote connection", pid: 42, customSocket: true, healthy: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			startedAt := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
			deps := newTestDeps(t, &stubClient{})
			homePaths, err := deps.resolveHome()
			if err != nil {
				t.Fatal(err)
			}
			socketPath := homePaths.DaemonSocket
			if test.customSocket {
				socketPath = filepath.Join(homePaths.HomeDir, "custom.sock")
				deps.loadConfig = func() (compozyconfig.Config, error) {
					cfg := compozyconfig.DefaultWithHome(homePaths)
					cfg.Daemon.Socket = socketPath
					cfg.Gateway.ActiveConnection = "remote"
					return cfg, nil
				}
			}
			deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
				return compozydaemon.Info{PID: 42, Port: 2123, StartedAt: startedAt}, nil
			}
			deps.processAlive = func(int) bool { return true }
			deps.processMatchesStartTime = func(int, time.Time) bool { return true }
			deps.newClient = func(target ClientTarget) (DaemonClient, error) {
				if target != LocalClientTarget(socketPath) {
					t.Fatalf("newClient() target = %+v, want local socket %q", target, socketPath)
				}
				return &stubClient{daemonStatusFn: func(context.Context) (DaemonStatus, error) {
					return DaemonStatus{
						Status: daemonRunningStatus, PID: test.pid, StartedAt: startedAt,
						Socket: socketPath, HTTPHost: "127.0.0.1", HTTPPort: 2123,
					}, nil
				}}, nil
			}
			_, healthy, err := probeBootstrapDaemon(t.Context(), deps, homePaths)
			if test.healthy {
				if !healthy || err != nil {
					t.Fatalf("probeBootstrapDaemon() = healthy=%t error=%v", healthy, err)
				}
			} else if healthy || err == nil || !strings.Contains(err.Error(), "does not match the local discovery record") {
				t.Fatalf("probeBootstrapDaemon() = healthy=%t error=%v, want identity mismatch", healthy, err)
			}
		})
	}
}

func TestDaemonBootstrapSetupFailures(t *testing.T) {
	t.Run("Should emit a typed resolve failure before home setup", func(t *testing.T) {
		deps := newTestDeps(t, &stubClient{})
		deps.resolveHome = func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{}, errors.New("injected home resolution failure")
		}

		stdout, _, err := executeRootCommand(t, deps, "daemon", "bootstrap", "-o", "jsonl")
		assertErrorContains(t, err, "injected home resolution failure")
		var event bootstrapEvent
		if decodeErr := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &event); decodeErr != nil {
			t.Fatalf("json.Unmarshal(failure event) error = %v; output=%s", decodeErr, stdout)
		}
		if event.Phase != bootstrapPhaseResolve || event.Status != bootstrapStatusFailed ||
			event.Classification != bootstrapProbeUnavailable {
			t.Fatalf("failure event = %#v, want typed resolve failure", event)
		}
	})
}

func TestDaemonBootstrapEmitsAttachJSONL(t *testing.T) {
	t.Run("Should emit the attached daemon origin in the ready event", func(t *testing.T) {
		t.Parallel()

		startedAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
		var daemonSocket string
		client := &stubClient{daemonStatusFn: func(context.Context) (DaemonStatus, error) {
			return DaemonStatus{
				Status: "running", PID: 4242, Version: "v1.2.0", MinAppVersion: "v1.0.0",
				HTTPHost: "127.0.0.1", HTTPPort: 2123, StartedAt: startedAt, Socket: daemonSocket,
			}, nil
		}}
		deps := newTestDeps(t, client)
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
		daemonSocket = homePaths.DaemonSocket
		writeFile(t, homePaths.DaemonInfo, `{"pid":4242,"port":2123,"started_at":"2026-04-03T12:00:00Z"}`)
		deps.processAlive = func(int) bool { return true }
		deps.processMatchesStartTime = func(int, time.Time) bool { return true }
		deps.executable = os.Executable

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"daemon",
			"bootstrap",
			"--minimum-runtime",
			">=1.1.0",
			"--app-version",
			"v1.0.0",

			"-o",
			"jsonl",
		)
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 2 {
			t.Fatalf("bootstrap JSONL lines = %d, want resolve and ready; output=%s", len(lines), stdout)
		}
		var ready bootstrapEvent
		if err := json.Unmarshal([]byte(lines[1]), &ready); err != nil {
			t.Fatalf("json.Unmarshal(ready) error = %v", err)
		}
		if ready.Phase != bootstrapPhaseReady || ready.Resolution != bootstrapResolutionAttach || ready.Daemon == nil {
			t.Fatalf("ready event = %#v, want attach completion", ready)
		}
		if ready.Daemon.Origin != "http://127.0.0.1:2123" {
			t.Fatalf("ready daemon origin = %q, want listener origin", ready.Daemon.Origin)
		}
		if ready.Daemon.Owned {
			t.Fatal("ready daemon owned = true, want external attach ownership")
		}
	})
}

func TestBootstrapRemovesOnlyStaleDaemonRecords(t *testing.T) {
	t.Run("Should preserve a record published after an absent lookup", func(t *testing.T) {
		t.Parallel()
		deps := newTestDeps(t, &stubClient{}).withDefaults()
		paths, err := deps.resolveHome()
		if err != nil {
			t.Fatal(err)
		}
		info := compozydaemon.Info{PID: 42, StartedAt: fixedTestNow}
		deps.readDaemonInfo = func(path string) (compozydaemon.Info, error) {
			if err := compozydaemon.WriteInfo(path, info); err != nil {
				t.Fatal(err)
			}
			return compozydaemon.Info{}, os.ErrNotExist
		}
		if err := cleanupStaleBootstrapDaemonRecord(paths, deps); err != nil {
			t.Fatal(err)
		}
		got, err := compozydaemon.ReadInfo(paths.DaemonInfo)
		if err != nil || got != info {
			t.Fatalf("concurrently published info = %#v, %v; want %#v", got, err, info)
		}
	})
	t.Run("Should remove a daemon record whose process identity is stale", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		deps = deps.withDefaults()
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
		writeFile(t, homePaths.DaemonInfo, `{"pid":4242,"port":2123,"started_at":"2026-04-03T12:00:00Z"}`)
		deps.processAlive = func(int) bool { return true }
		deps.processMatchesStartTime = func(int, time.Time) bool { return false }

		if err := cleanupStaleBootstrapDaemonRecord(homePaths, deps); err != nil {
			t.Fatalf("cleanupStaleBootstrapDaemonRecord() error = %v", err)
		}
		if _, err := os.Stat(homePaths.DaemonInfo); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(stale daemon record) error = %v, want removed record", err)
		}
	})
}

// TestBootstrapRuntimeVersionPolicy verifies that desktop bootstrap preserves compatible newer runtimes.
func TestBootstrapRuntimeVersionPolicy(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, running, bundled string
		newer                  bool
	}{
		{"Should preserve a newer prerelease daemon", "v0.3.0-beta.23", "0.3.0-beta.22", true},
		{"Should preserve a newer stable daemon", "1.2.0", "1.1.0", true},
		{"Should allow upgrading an older daemon", "0.3.0-beta.22", "0.3.0-beta.23", false},
		{"Should compare equal versions without the tag prefix", "v1.0.0", "1.0.0", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := bootstrapRuntimeIsNewer(test.running, test.bundled); got != test.newer {
				t.Fatalf("newer = %t, want %t", got, test.newer)
			}
		})
	}
}

// bootstrapLockOwnerFixture drives bootstrap while another daemon process may own the singleton lock.
type bootstrapLockOwnerFixture struct {
	deps       commandDeps
	homePaths  compozyconfig.HomePaths
	ownerPID   int
	ownerAlive bool
	lockHeld   bool
	// staleRecordedPID is reported by the classification inspection; later inspections up to
	// staleInspections report a held lock with no recorded PID before the holder records ownerPID.
	// 0 reports ownerPID from the start.
	staleRecordedPID int
	staleInspections int
	inspections      int
	// foreignReadyPID makes a daemon other than the lock holder answer healthy; 0 disables it.
	foreignReadyPID int
	// bootAfterProbes publishes the owner's daemon record after this many post-spawn probes; 0 never boots.
	bootAfterProbes int
	// recoverOnSpawn makes this spawn attempt (1-based) a healthy child; 0 never recovers.
	recoverOnSpawn int
	spawns         int
	probes         int
}

func newBootstrapLockOwnerFixture(t *testing.T) *bootstrapLockOwnerFixture {
	t.Helper()
	fixture := &bootstrapLockOwnerFixture{ownerPID: 777, ownerAlive: true, lockHeld: true}
	startedAt := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	var socketPath string
	statusFor := func(pid int) DaemonStatus {
		return DaemonStatus{
			Status: daemonRunningStatus, PID: pid, Version: "v1.2.0", StartedAt: startedAt,
			Socket: socketPath, HTTPHost: "127.0.0.1", HTTPPort: 2123,
		}
	}
	readyPID := func() int {
		switch {
		case fixture.recoverOnSpawn > 0 && fixture.spawns >= fixture.recoverOnSpawn:
			return 42
		case fixture.bootAfterProbes > 0 && fixture.probes >= fixture.bootAfterProbes:
			return fixture.ownerPID
		case fixture.foreignReadyPID > 0 && fixture.spawns > 0:
			return fixture.foreignReadyPID
		default:
			return 0
		}
	}
	client := &stubClient{daemonStatusFn: func(context.Context) (DaemonStatus, error) {
		if pid := readyPID(); pid > 0 {
			return statusFor(pid), nil
		}
		return DaemonStatus{}, errors.New("injected: daemon socket is not listening yet")
	}}
	deps := newTestDeps(t, client)
	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatal(err)
	}
	socketPath = homePaths.DaemonSocket
	writeFile(t, bootstrapRuntimePath(homePaths), "operator runtime")
	deps.executable = os.Executable
	deps.pollInterval = time.Millisecond
	deps.startTimeout = time.Second
	deps.inspectDaemonLock = func(path string) (compozydaemon.LockOwner, error) {
		if path != homePaths.DaemonLock {
			t.Fatalf("inspectDaemonLock(%q), want %q", path, homePaths.DaemonLock)
		}
		fixture.inspections++
		if fixture.staleRecordedPID > 0 && fixture.inspections == 1 {
			return compozydaemon.LockOwner{PID: fixture.staleRecordedPID, Held: fixture.lockHeld}, nil
		}
		if fixture.staleRecordedPID > 0 && fixture.inspections <= fixture.staleInspections {
			return compozydaemon.LockOwner{Held: fixture.lockHeld}, nil
		}
		return compozydaemon.LockOwner{PID: fixture.ownerPID, Held: fixture.lockHeld}, nil
	}
	deps.processAlive = func(pid int) bool {
		if pid == fixture.ownerPID {
			return fixture.ownerAlive
		}
		if fixture.foreignReadyPID > 0 && pid == fixture.foreignReadyPID {
			return true
		}
		if pid == fixture.staleRecordedPID {
			// The stale PID is alive at classification, then exits while the holder keeps the lock.
			return fixture.inspections <= 1
		}
		return pid == 42 && fixture.recoverOnSpawn > 0 && fixture.spawns >= fixture.recoverOnSpawn
	}
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		if fixture.spawns > 0 {
			fixture.probes++
		}
		if pid := readyPID(); pid > 0 {
			return compozydaemon.Info{PID: pid, Port: 2123, StartedAt: startedAt}, nil
		}
		return compozydaemon.Info{}, os.ErrNotExist
	}
	deps.spawnBootstrapRuntime = func(context.Context, compozyconfig.HomePaths, string) (daemonProcess, error) {
		fixture.spawns++
		child := &timeoutDaemonProcess{done: make(chan struct{})}
		if fixture.recoverOnSpawn == 0 || fixture.spawns < fixture.recoverOnSpawn {
			child.complete(fmt.Errorf(
				"exit status 1: stderr=error: daemon: already running with pid %d", fixture.ownerPID,
			))
		}
		return child, nil
	}
	fixture.deps = deps
	fixture.homePaths = homePaths
	return fixture
}

func (f *bootstrapLockOwnerFixture) run(t *testing.T, flags ...string) ([]bootstrapEvent, error) {
	t.Helper()
	args := append([]string{"daemon", "bootstrap", "-o", "jsonl"}, flags...)
	stdout, _, err := executeRootCommand(t, f.deps, args...)
	return decodeBootstrapEvents(t, stdout), err
}

// observeStatus wraps every client the fixture hands out so a test can script the status answers.
func (f *bootstrapLockOwnerFixture) observeStatus(
	answer func(ctx context.Context, next func(context.Context) (DaemonStatus, error)) (DaemonStatus, error),
) {
	newClient := f.deps.newClient
	f.deps.newClient = func(target ClientTarget) (DaemonClient, error) {
		client, err := newClient(target)
		if err != nil {
			return nil, err
		}
		return &stubClient{daemonStatusFn: func(ctx context.Context) (DaemonStatus, error) {
			return answer(ctx, client.DaemonStatus)
		}}, nil
	}
}

func decodeBootstrapEvents(t *testing.T, stdout string) []bootstrapEvent {
	t.Helper()
	var events []bootstrapEvent
	for line := range strings.SplitSeq(strings.TrimSpace(stdout), "\n") {
		var event bootstrapEvent
		if decodeErr := json.Unmarshal([]byte(line), &event); decodeErr != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", line, decodeErr)
		}
		events = append(events, event)
	}
	return events
}

func countBootstrapEvents(events []bootstrapEvent, phase bootstrapPhase, status string) int {
	count := 0
	for _, event := range events {
		if event.Phase == phase && event.Status == status {
			count++
		}
	}
	return count
}

// TestDaemonBootstrapLockOwner verifies bootstrap attaches to a live booting lock owner instead of
// burning start attempts, while stale locks still recover through start.
func TestDaemonBootstrapLockOwner(t *testing.T) {
	t.Run("Should attach once a live booting lock owner becomes ready", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		fixture.bootAfterProbes = 3
		// A different healthy daemon answers first; bootstrap must keep waiting for the holder.
		fixture.foreignReadyPID = 888
		newClient := fixture.deps.newClient
		attachClients := 0
		fixture.deps.newClient = func(target ClientTarget) (DaemonClient, error) {
			if fixture.inspections > 1 {
				attachClients++
			}
			return newClient(target)
		}

		events, err := fixture.run(t)
		if err != nil {
			t.Fatalf("bootstrap error = %v; events=%#v", err, events)
		}
		if fixture.spawns != 1 {
			t.Fatalf("spawned children = %d, want one lock-contended start before attach", fixture.spawns)
		}
		if attachClients != 1 {
			t.Fatalf("attach clients = %d, want one transport throughout readiness polling", attachClients)
		}
		if got := countBootstrapEvents(events, bootstrapPhaseStart, bootstrapStatusRetrying); got != 0 {
			t.Fatalf("retrying events = %d, want none while a live owner boots; events=%#v", got, events)
		}
		if got := countBootstrapEvents(events, bootstrapPhaseAttach, bootstrapStatusStarted); got != 1 {
			t.Fatalf("attach started events = %d, want 1; events=%#v", got, events)
		}
		ready := events[len(events)-1]
		if ready.Phase != bootstrapPhaseReady || ready.Resolution != bootstrapResolutionAttach ||
			ready.Message != "Attached to the running CompozyOS runtime." ||
			ready.Daemon == nil || ready.Daemon.PID != fixture.ownerPID || ready.Daemon.Owned {
			t.Fatalf("final event = %#v, want attach to lock owner pid %d", ready, fixture.ownerPID)
		}
	})

	t.Run("Should keep waiting for an unrecorded lock holder after a stale recorded pid exits", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		fixture.staleRecordedPID = 666
		fixture.staleInspections = 3
		fixture.bootAfterProbes = 4

		events, err := fixture.run(t)
		if err != nil {
			t.Fatalf("bootstrap error = %v; events=%#v", err, events)
		}
		if fixture.spawns != 1 {
			t.Fatalf("spawned children = %d, want one lock-contended start before attach", fixture.spawns)
		}
		if got := countBootstrapEvents(events, bootstrapPhaseStart, bootstrapStatusRetrying); got != 0 {
			t.Fatalf("retrying events = %d, want none while a live owner boots; events=%#v", got, events)
		}
		ready := events[len(events)-1]
		if ready.Phase != bootstrapPhaseReady || ready.Resolution != bootstrapResolutionAttach ||
			ready.Daemon == nil || ready.Daemon.PID != fixture.ownerPID {
			t.Fatalf("final event = %#v, want attach to the re-inspected holder pid %d", ready, fixture.ownerPID)
		}
	})

	t.Run("Should recover a stale lock whose recorded pid was recycled through the start path", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		fixture.lockHeld = false
		fixture.recoverOnSpawn = 2

		events, err := fixture.run(t)
		if err != nil {
			t.Fatalf("bootstrap error = %v; events=%#v", err, events)
		}
		if got := countBootstrapEvents(events, bootstrapPhaseAttach, bootstrapStatusStarted); got != 0 {
			t.Fatalf("attach started events = %d, want none for a free lock; events=%#v", got, events)
		}
		if got := countBootstrapEvents(events, bootstrapPhaseStart, bootstrapStatusRetrying); got != 1 {
			t.Fatalf("retrying events = %d, want 1 failed launch before recovery; events=%#v", got, events)
		}
		ready := events[len(events)-1]
		if ready.Phase != bootstrapPhaseReady || ready.Resolution != bootstrapResolutionStart ||
			ready.Daemon == nil || ready.Daemon.PID != 42 {
			t.Fatalf("final event = %#v, want started child pid 42", ready)
		}
	})

	t.Run("Should cancel an in-flight status probe and reject readiness after the attach deadline", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		fixture.bootAfterProbes = 3
		fixture.deps.bootstrapAttachTimeout = 20 * time.Millisecond
		newClient := fixture.deps.newClient
		deadlineObserved := false
		fixture.deps.newClient = func(target ClientTarget) (DaemonClient, error) {
			client, err := newClient(target)
			if err != nil {
				return nil, err
			}
			return &stubClient{daemonStatusFn: func(ctx context.Context) (DaemonStatus, error) {
				status, err := client.DaemonStatus(ctx)
				if err != nil || status.PID != fixture.ownerPID {
					return status, err
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Error("attach status request has no deadline")
					return status, nil
				}
				<-ctx.Done()
				deadlineObserved = true
				return status, nil
			}}, nil
		}

		events, err := fixture.run(t)
		assertErrorContains(t, err, "the running CompozyOS daemon pid=777 did not become ready")
		if !deadlineObserved || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf(
				"deadline observed = %t, error = %v; want cancellation during status request",
				deadlineObserved, err,
			)
		}
		failed := events[len(events)-1]
		if failed.Phase != bootstrapPhaseAttach || failed.Status != bootstrapStatusFailed {
			t.Fatalf("final event = %#v, want attach failure instead of late readiness", failed)
		}
	})

	t.Run("Should fail naming the lock owner while another daemon answers healthy", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		fixture.foreignReadyPID = 888
		fixture.deps.bootstrapAttachTimeout = 20 * time.Millisecond

		events, err := fixture.run(t)
		assertErrorContains(t, err, "the running CompozyOS daemon pid=777 did not become ready")
		if fixture.spawns != 1 {
			t.Fatalf("spawned children = %d, want 1", fixture.spawns)
		}
		failed := events[len(events)-1]
		if failed.Phase != bootstrapPhaseAttach || failed.Status != bootstrapStatusFailed ||
			failed.Classification != bootstrapProbeUnavailable ||
			strings.Contains(failed.Message, "detached daemon exited before readiness") {
			t.Fatalf("failure event = %#v, want attach failure naming the running daemon", failed)
		}
	})
}

// TestDaemonBootstrapConfigFailure verifies a config load failure ends bootstrap once, names the
// file and its fault, and is never reported or retried as an unhealthy runtime.
func TestDaemonBootstrapConfigFailure(t *testing.T) {
	for _, test := range []struct {
		name     string
		contents string
		wantKeys []string
		wantLine int
	}{
		{
			name:     "Should name the unknown keys without asking the running daemon",
			contents: "[daemon]\nbogus = true\n",
			wantKeys: []string{"daemon.bogus"},
		},
		{
			name:     "Should locate invalid TOML without asking the running daemon",
			contents: "[daemon]\nsocket = = 1\n",
			wantLine: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			statusRequests := 0
			deps := newTestDeps(t, &stubClient{daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				statusRequests++
				return DaemonStatus{}, errors.New("injected: daemon status must not be requested")
			}})
			homePaths, err := deps.resolveHome()
			if err != nil {
				t.Fatalf("resolveHome() error = %v", err)
			}
			writeFile(t, homePaths.ConfigFile, test.contents)
			writeFile(t, homePaths.DaemonInfo, `{"pid":4242,"port":2123,"started_at":"2026-04-03T12:00:00Z"}`)
			deps.loadConfig = func() (compozyconfig.Config, error) {
				return compozyconfig.LoadGlobalConfig(homePaths)
			}
			deps.processAlive = func(int) bool { return true }
			deps.executable = os.Executable

			stdout, _, err := executeRootCommand(t, deps, "daemon", "bootstrap", "-o", "jsonl")
			assertErrorContains(t, err, "cli: CompozyOS configuration could not be loaded")
			events := decodeBootstrapEvents(t, stdout)
			if strings.Contains(stdout, "listening but unhealthy") || statusRequests != 0 {
				t.Fatalf("status requests = %d, output = %s; want no unhealthy daemon report", statusRequests, stdout)
			}
			if got := countBootstrapEvents(events, bootstrapPhaseStart, bootstrapStatusStarted); got != 0 {
				t.Fatalf("start events = %d, want none for a config failure; events=%#v", got, events)
			}
			failed := events[len(events)-1]
			if failed.Phase != bootstrapPhaseResolve || failed.Status != bootstrapStatusFailed ||
				failed.Classification != bootstrapProbeConfigInvalid || failed.Config == nil {
				t.Fatalf("final event = %#v, want a config-invalid resolve failure", failed)
			}
			if failed.Config.Path != homePaths.ConfigFile ||
				!slices.Equal(failed.Config.UnknownKeys, test.wantKeys) || failed.Config.Line != test.wantLine {
				t.Fatalf(
					"config detail = %#v, want path %q keys %v line %d",
					failed.Config, homePaths.ConfigFile, test.wantKeys, test.wantLine,
				)
			}
		})
	}

	t.Run("Should stop after one failed launch when the config cannot be loaded", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		fixture.lockHeld = false
		writeFile(t, fixture.homePaths.ConfigFile, "[daemon]\nbogus = true\n")
		fixture.deps.loadConfig = func() (compozyconfig.Config, error) {
			return compozyconfig.LoadGlobalConfig(fixture.homePaths)
		}

		events, err := fixture.run(t)
		assertErrorContains(t, err, "cli: CompozyOS configuration could not be loaded")
		if fixture.spawns != 1 {
			t.Fatalf("spawned children = %d, want one launch before the config diagnosis", fixture.spawns)
		}
		if got := countBootstrapEvents(events, bootstrapPhaseStart, bootstrapStatusRetrying); got != 0 {
			t.Fatalf("retrying events = %d, want none for a config failure; events=%#v", got, events)
		}
		failed := events[len(events)-1]
		if failed.Phase != bootstrapPhaseFailed || failed.Status != bootstrapStatusFailed ||
			failed.Classification != bootstrapProbeConfigInvalid || failed.Config == nil ||
			failed.Config.Path != fixture.homePaths.ConfigFile ||
			!slices.Equal(failed.Config.UnknownKeys, []string{"daemon.bogus"}) {
			t.Fatalf("final event = %#v, want a terminal config-invalid failure naming the key", failed)
		}
	})
}

// bootstrapRestartStatuses scripts the restart journal a replacement publishes while it migrates data.
func bootstrapRestartStatuses(migration string) []SettingsRestartStatusRecord {
	migratingOf := func(completed, total int64) SettingsRestartStatusRecord {
		return SettingsRestartStatusRecord{
			OperationID: "bootstrap-restart",
			Status:      contract.RestartOperationStarting,
			Progress: &contract.RestartProgress{
				Phase: contract.RestartProgressMigrating, Stage: "telemetry_relocation",
				Completed: completed, Total: total,
			},
		}
	}
	migrating := func(completed int64) SettingsRestartStatusRecord { return migratingOf(completed, 2048) }
	ready := SettingsRestartStatusRecord{OperationID: "bootstrap-restart", Status: "ready", NewPID: 4343}
	switch migration {
	case "advancing":
		statuses := []SettingsRestartStatusRecord{migratingOf(0, 0)}
		for completed := range int64(20) {
			statuses = append(statuses, migrating(completed/10*1024))
		}
		return append(statuses, migrating(2048), ready)
	case "stalled":
		return []SettingsRestartStatusRecord{migrating(1024)}
	default:
		return []SettingsRestartStatusRecord{ready}
	}
}

// assertBootstrapProvisionVersions checks that a replacement names the runtime it replaces and the
// bundled version it installs, so the desktop can show the transition.
func assertBootstrapProvisionVersions(t *testing.T, output string, replaced string) {
	t.Helper()
	for line := range strings.Lines(output) {
		var event bootstrapEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("bootstrap event %q: %v", line, err)
		}
		if event.Phase != bootstrapPhaseProvision || event.Status != bootstrapStatusStarted || event.Stage != "" {
			continue
		}
		if event.Versions == nil || event.Versions.To == "" ||
			event.Versions.From != normalizedBootstrapVersion(replaced) {
			t.Fatalf("provision versions = %#v, want %q replaced by the bundle", event.Versions, replaced)
		}
		return
	}
	t.Fatal("replacement emitted no provision started event")
}

func assertBootstrapMigrationEvents(t *testing.T, output string, wantPercents []int) {
	t.Helper()
	var percents []int
	for line := range strings.Lines(output) {
		var event bootstrapEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("bootstrap event %q: %v", line, err)
		}
		if event.Stage != bootstrapStageMigrate {
			continue
		}
		if event.Phase != bootstrapPhaseProvision || event.Status != bootstrapStatusStarted || event.Percent == nil {
			t.Fatalf("migration event = %#v, want a started provision event with a percentage", event)
		}
		if event.Completed == nil || event.Total == nil || event.Migration != "telemetry_relocation" {
			t.Fatalf("migration event = %#v, want counts and the runtime's stage name", event)
		}
		percents = append(percents, *event.Percent)
	}
	if !slices.Equal(percents, wantPercents) {
		t.Fatalf("migration progress events = %v, want %v", percents, wantPercents)
	}
}

// TestDaemonBootstrapDesktopHandoff verifies the desktop handoff contract of `daemon bootstrap`:
// a runtime mutation held by another process is waited out instead of spending start attempts, a
// launch that cannot succeed fails once, and the daemon's lifecycle phase is reported and, only
// when asked with --ready-phase serving, accepted as ready before `running`.
func TestDaemonBootstrapDesktopHandoff(t *testing.T) {
	serving := func(status DaemonStatus, step string) DaemonStatus {
		status.Status = "draining"
		status.Readiness = &contract.DaemonReadinessPayload{Phase: contract.DaemonReadinessServing, Step: step}
		return status
	}
	holdMutationLock := func(t *testing.T, fixture *bootstrapLockOwnerFixture) *compozydaemon.UpdateLock {
		t.Helper()
		lock, err := compozydaemon.AcquireUpdateLock(
			daemonStartUpdateLockPath(fixture.homePaths),
			compozydaemon.UpdateLockOwner{PID: os.Getpid(), StartedAt: time.Now().UTC()},
		)
		if err != nil {
			t.Fatalf("AcquireUpdateLock() error = %v", err)
		}
		return lock
	}

	t.Run("Should wait out a runtime mutation without spending start attempts", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		fixture.recoverOnSpawn = 1
		lock := holdMutationLock(t, fixture)
		statusRequests := 0
		fixture.observeStatus(func(
			ctx context.Context, next func(context.Context) (DaemonStatus, error),
		) (DaemonStatus, error) {
			statusRequests++
			// The other mutation finishes while bootstrap is polling.
			if statusRequests == 3 {
				if err := lock.Release(); err != nil {
					t.Errorf("Release() error = %v", err)
				}
			}
			return next(ctx)
		})

		events, err := fixture.run(t, "--ready-phase", "serving")
		if err != nil {
			t.Fatalf("bootstrap error = %v; events=%#v", err, events)
		}
		if fixture.spawns != 1 {
			t.Fatalf("spawned children = %d, want one launch after the mutation finished", fixture.spawns)
		}
		if got := countBootstrapEvents(events, bootstrapPhaseStart, bootstrapStatusRetrying); got != 0 {
			t.Fatalf("retrying events = %d, want none while another mutation runs; events=%#v", got, events)
		}
		waits := 0
		for _, event := range events {
			if event.Wait == nil {
				continue
			}
			waits++
			if event.Wait.Reason != bootstrapWaitRuntimeMutation || event.Wait.OwnerPID != os.Getpid() ||
				event.Attempt != 1 || event.Classification != bootstrapProbeMutationBusy {
				t.Fatalf("wait event = %#v, want a first-attempt mutation wait naming pid %d", event, os.Getpid())
			}
		}
		if waits != 1 {
			t.Fatalf("wait events = %d, want 1; events=%#v", waits, events)
		}
		ready := events[len(events)-1]
		if ready.Phase != bootstrapPhaseReady || ready.Attempt != 1 || ready.Daemon == nil || ready.Daemon.PID != 42 {
			t.Fatalf("final event = %#v, want the first attempt ready as pid 42", ready)
		}
	})

	t.Run("Should fail naming the runtime mutation once the wait bound passes", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		fixture.deps.bootstrapAttachTimeout = 20 * time.Millisecond
		lock := holdMutationLock(t, fixture)
		t.Cleanup(func() {
			if err := lock.Release(); err != nil {
				t.Errorf("Release() error = %v", err)
			}
		})

		events, err := fixture.run(t)
		if !errors.Is(err, compozydaemon.ErrRuntimeMutationInProgress) {
			t.Fatalf("bootstrap error = %v, want the runtime mutation", err)
		}
		if fixture.spawns != 0 {
			t.Fatalf("spawned children = %d, want none while the mutation lock is held", fixture.spawns)
		}
		if got := countBootstrapEvents(events, bootstrapPhaseStart, bootstrapStatusRetrying); got != 0 {
			t.Fatalf("retrying events = %d, want none; events=%#v", got, events)
		}
		failed := events[len(events)-1]
		if failed.Phase != bootstrapPhaseFailed || failed.Status != bootstrapStatusFailed ||
			failed.Classification != bootstrapProbeMutationBusy {
			t.Fatalf("final event = %#v, want a terminal mutation-in-progress failure", failed)
		}
	})

	t.Run("Should fail once when the runtime binary cannot be launched", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		launches := 0
		fixture.deps.spawnBootstrapRuntime = func(
			context.Context, compozyconfig.HomePaths, string,
		) (daemonProcess, error) {
			launches++
			return nil, fmt.Errorf("cli: spawn detached daemon: %w", os.ErrPermission)
		}

		events, err := fixture.run(t)
		assertErrorContains(t, err, "the CompozyOS runtime cannot be launched")
		if launches != 1 {
			t.Fatalf("launches = %d, want one for a failure no retry can change", launches)
		}
		if got := countBootstrapEvents(events, bootstrapPhaseStart, bootstrapStatusRetrying); got != 0 {
			t.Fatalf("retrying events = %d, want none; events=%#v", got, events)
		}
		failed := events[len(events)-1]
		if failed.Phase != bootstrapPhaseFailed || failed.Classification != bootstrapProbeUnlaunchable {
			t.Fatalf("final event = %#v, want a terminal runtime-unlaunchable failure", failed)
		}
	})

	t.Run("Should report lifecycle steps and still wait for running by default", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		fixture.recoverOnSpawn = 1
		steps := []string{"recoverBootDelegatedSessions", "resumeBootInterruptedSessions"}
		answers := 0
		fixture.observeStatus(func(
			ctx context.Context, next func(context.Context) (DaemonStatus, error),
		) (DaemonStatus, error) {
			status, err := next(ctx)
			if err != nil || answers >= len(steps) {
				return status, err
			}
			answers++
			return serving(status, steps[answers-1]), nil
		})

		events, err := fixture.run(t)
		if err != nil {
			t.Fatalf("bootstrap error = %v; events=%#v", err, events)
		}
		var reported []string
		for _, event := range events[:len(events)-1] {
			if event.Readiness == nil {
				continue
			}
			if event.Phase != bootstrapPhaseStart || event.Status != bootstrapStatusStarted || event.Attempt != 1 ||
				event.Readiness.Phase != string(contract.DaemonReadinessServing) {
				t.Fatalf("readiness event = %#v, want a first-attempt start event in the serving phase", event)
			}
			reported = append(reported, event.Readiness.Step)
		}
		if !slices.Equal(reported, steps) {
			t.Fatalf("reported steps = %v, want %v; events=%#v", reported, steps, events)
		}
		ready := events[len(events)-1]
		if ready.Phase != bootstrapPhaseReady || ready.Daemon == nil || ready.Daemon.Status != daemonRunningStatus {
			t.Fatalf("final event = %#v, want readiness only once the daemon is running", ready)
		}
	})

	t.Run("Should report a started daemon ready at the serving phase when asked", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		fixture.recoverOnSpawn = 1
		fixture.observeStatus(func(
			ctx context.Context, next func(context.Context) (DaemonStatus, error),
		) (DaemonStatus, error) {
			status, err := next(ctx)
			if err != nil {
				return status, err
			}
			return serving(status, "resumeBootInterruptedSessions"), nil
		})

		events, err := fixture.run(t, "--ready-phase", "serving")
		if err != nil {
			t.Fatalf("bootstrap error = %v; events=%#v", err, events)
		}
		ready := events[len(events)-1]
		if ready.Phase != bootstrapPhaseReady || ready.Status != bootstrapStatusCompleted ||
			ready.Resolution != bootstrapResolutionStart || ready.Daemon == nil || ready.Daemon.PID != 42 ||
			ready.Readiness == nil || ready.Readiness.Phase != string(contract.DaemonReadinessServing) ||
			ready.Readiness.Step != "resumeBootInterruptedSessions" {
			t.Fatalf("final event = %#v, want pid 42 ready in the serving phase", ready)
		}
	})

	t.Run("Should attach to a booting lock owner at the serving phase when asked", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		// The owner never publishes daemon.json: only its status answer says it serves.
		fixture.observeStatus(func(
			context.Context, func(context.Context) (DaemonStatus, error),
		) (DaemonStatus, error) {
			if fixture.spawns == 0 {
				return DaemonStatus{}, errors.New("injected: daemon socket is not listening yet")
			}
			return serving(DaemonStatus{
				PID: fixture.ownerPID, Version: "v1.2.0", HTTPHost: "127.0.0.1", HTTPPort: 2123,
			}, "resumeBootInterruptedSessions"), nil
		})

		events, err := fixture.run(t, "--ready-phase", "serving")
		if err != nil {
			t.Fatalf("bootstrap error = %v; events=%#v", err, events)
		}
		ready := events[len(events)-1]
		if ready.Phase != bootstrapPhaseReady || ready.Resolution != bootstrapResolutionAttach ||
			ready.Daemon == nil || ready.Daemon.PID != fixture.ownerPID ||
			ready.Readiness == nil || ready.Readiness.Phase != string(contract.DaemonReadinessServing) {
			t.Fatalf("final event = %#v, want attach to serving lock owner pid %d", ready, fixture.ownerPID)
		}
	})

	t.Run("Should reject an unknown ready phase", func(t *testing.T) {
		fixture := newBootstrapLockOwnerFixture(t)
		_, _, err := executeRootCommand(t, fixture.deps, "daemon", "bootstrap", "--ready-phase", "later")
		assertErrorContains(t, err, `--ready-phase "later" is invalid`)
		if fixture.spawns != 0 {
			t.Fatalf("spawned children = %d, want none for an invalid flag", fixture.spawns)
		}
	})
}

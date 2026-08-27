package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

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
		var failure *bootstrapCompatibilityFailure
		if !errors.As(err, &failure) || failure.Reason != "runtime_below_minimum" ||
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
		var failure *bootstrapCompatibilityFailure
		if !errors.As(err, &failure) || failure.Reason != "app_below_minimum" ||
			failure.Runtime != "v1.2.0" || failure.Needed != "v1.2.0" {
			t.Fatalf("compatibility failure = %#v, want typed app skew", failure)
		}
	})
}

func TestDesktopRuntimeBootstrapReplacement(t *testing.T) {
	t.Parallel()

	t.Run("Should stop a stale desktop-owned runtime before attaching", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		deps = deps.withDefaults()
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		installedPath := bootstrapRuntimePath(homePaths)
		bundlePath := filepath.Join(t.TempDir(), "bundled-compozy")
		writeFile(t, installedPath, "current-runtime")
		writeFile(t, bundlePath, "current-runtime")
		cmd := &cobra.Command{}
		cmd.SetContext(testutil.Context(t))
		execution := bootstrapExecution{
			cmd:           cmd,
			deps:          deps,
			options:       bootstrapOptions{appVersion: "0.3.0-beta.21", channel: "beta"},
			homePaths:     homePaths,
			bundlePath:    bundlePath,
			installedPath: installedPath,
		}
		if err := compozyupdate.WriteDesktopProvenance(
			homePaths,
			installedPath,
			execution.provenance(),
		); err != nil {
			t.Fatalf("WriteDesktopProvenance() error = %v", err)
		}
		writeFile(t, homePaths.DaemonInfo, `{"pid":4242,"started_at":"2026-04-03T12:00:00Z"}`)
		running := true
		var signaled syscall.Signal
		execution.deps.processAlive = func(int) bool { return running }
		execution.deps.processMatchesStartTime = func(int, time.Time) bool { return true }
		execution.deps.signalProcess = func(_ int, signal syscall.Signal) error {
			signaled = signal
			running = false
			return nil
		}
		execution.deps.pollInterval = time.Millisecond
		execution.deps.stopTimeout = time.Second

		replaced, err := execution.replaceOutdatedDesktopRuntime(DaemonStatus{
			PID:     4242,
			Version: "0.3.0-beta.20",
		})
		if err != nil {
			t.Fatalf("replaceOutdatedDesktopRuntime() error = %v", err)
		}
		if !replaced {
			t.Fatal("replaceOutdatedDesktopRuntime() = false, want stale runtime restart")
		}
		if signaled != syscall.SIGTERM {
			t.Fatalf("signal = %v, want SIGTERM", signaled)
		}
	})
}

func TestProvisionBundledRuntime(t *testing.T) {
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
		cmd.SetOut(&bytes.Buffer{})
		if err := provisionBootstrapRuntime(cmd, &bootstrapProvisionRequest{
			resolution:    bootstrapResolutionProvision,
			homePaths:     homePaths,
			bundlePath:    bundlePath,
			installedPath: installedPath,
			provenance:    metadata,
		}); err != nil {
			t.Fatalf("provisionBootstrapRuntime(replacement) error = %v", err)
		}
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
	t.Run("Should reject a status response that does not match daemon.json", func(t *testing.T) {
		t.Parallel()

		startedAt := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
		homePaths := mustTestHomePaths(t)
		deps := newTestDeps(t, &stubClient{})
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{PID: 42, Port: 2123, StartedAt: startedAt}, nil
		}
		deps.processAlive = func(int) bool { return true }
		deps.processMatchesStartTime = func(int, time.Time) bool { return true }
		deps.newClient = func(target ClientTarget) (DaemonClient, error) {
			if target.socketPath != homePaths.DaemonSocket {
				t.Fatalf("newClient() socket = %q, want %q", target.socketPath, homePaths.DaemonSocket)
			}
			return &stubClient{daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{
					Status: daemonRunningStatus, PID: 99, StartedAt: startedAt,
					Socket: homePaths.DaemonSocket, HTTPHost: "127.0.0.1", HTTPPort: 2123,
				}, nil
			}}, nil
		}

		_, healthy, err := probeBootstrapDaemon(t.Context(), deps, homePaths)
		if healthy || err == nil || !strings.Contains(err.Error(), "does not match the local discovery record") {
			t.Fatalf("probeBootstrapDaemon() = healthy=%t error=%v, want identity mismatch", healthy, err)
		}
	})
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
			"--channel",
			"beta",
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

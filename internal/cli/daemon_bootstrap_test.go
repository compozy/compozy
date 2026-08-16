package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDaemonBootstrapPolicy(t *testing.T) {
	t.Run("Should prefer attach then start then provision", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			probe bootstrapProbe
			want  bootstrapResolution
		}{
			{name: "healthy", probe: bootstrapProbe{Healthy: true, Installed: true}, want: bootstrapResolutionAttach},
			{name: "installed", probe: bootstrapProbe{Installed: true}, want: bootstrapResolutionStart},
			{name: "empty", probe: bootstrapProbe{}, want: bootstrapResolutionProvision},
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
			probe bootstrapProbe
			want  bootstrapProbeClass
		}{
			{probe: bootstrapProbe{Healthy: true}, want: bootstrapProbeHealthy},
			{probe: bootstrapProbe{Listening: true}, want: bootstrapProbeListeningUnhealthy},
			{probe: bootstrapProbe{RecordPresent: true}, want: bootstrapProbeStaleRecord},
			{probe: bootstrapProbe{PortOccupied: true}, want: bootstrapProbePortConflict},
			{probe: bootstrapProbe{}, want: bootstrapProbeUnavailable},
		}
		for _, test := range cases {
			if got := classifyBootstrapProbe(test.probe); got != test.want {
				t.Fatalf("classifyBootstrapProbe(%#v) = %q, want %q", test.probe, got, test.want)
			}
		}
	})

	t.Run("Should back off from 500 milliseconds and give up after five unreadied starts", func(t *testing.T) {
		t.Parallel()

		want := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
		for attempt, expected := range want {
			if got := bootstrapBackoff(attempt + 1); got != expected {
				t.Fatalf("bootstrapBackoff(%d) = %s, want %s", attempt+1, got, expected)
			}
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
		if err == nil || !strings.Contains(err.Error(), "repair or update the runtime") {
			t.Fatalf("validateBootstrapCompatibility() error = %v, want runtime repair guidance", err)
		}
	})

	t.Run("Should refuse runtimes requiring a newer app", func(t *testing.T) {
		t.Parallel()

		err := validateBootstrapCompatibility(DaemonStatus{
			Version: "v1.2.0", MinAppVersion: "v1.2.0",
		}, ">=1.0.0", "v1.1.0")
		if err == nil || !strings.Contains(err.Error(), "repair the desktop app") {
			t.Fatalf("validateBootstrapCompatibility() error = %v, want app repair guidance", err)
		}
	})
}

func TestProvisionBundledRuntime(t *testing.T) {
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
		if err := provisionBundledRuntime(filepath.Join(t.TempDir(), "missing"), target); err == nil {
			t.Fatal("provisionBundledRuntime() error = nil, want missing bundle failure")
		}
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(target) error = %v, want no partial executable", err)
		}
	})
}

func TestDaemonBootstrapEmitsAttachJSONL(t *testing.T) {
	t.Run("Should emit the attached daemon origin in the ready event", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{daemonStatusFn: func(context.Context) (DaemonStatus, error) {
			return DaemonStatus{
				Status: "running", PID: 4242, Version: "v1.2.0", MinAppVersion: "v1.0.0",
				HTTPHost: "127.0.0.1", HTTPPort: 2123,
			}, nil
		}}
		deps := newTestDeps(t, client)
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
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

//go:build integration

package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil"
)

// TestBootMarksRestartOperationReadyAfterFreshDaemonInfo verifies real boot completes before readiness and reconciles abandoned restarts.
func TestBootMarksRestartOperationReadyAfterFreshDaemonInfo(t *testing.T) {
	t.Parallel()

	t.Run("Should persist the replacement process identity before marking restart ready", func(t *testing.T) {
		t.Parallel()

		homeDir := t.TempDir()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(homeDir)
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		homePaths.DaemonSocket = shortSocketPath(t)
		cfg := testConfig(t, homePaths)

		store := newRestartStore(homePaths, sequentialTime([]time.Time{
			time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
			time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
			time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
			time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
		}))
		operation, err := store.Create(RestartOperation{
			OperationID:        "restart-ready-integration",
			Status:             RestartStatusPending,
			OldPID:             5151,
			OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
			OldSocketPath:      homePaths.DaemonSocket,
			ActiveSessionCount: 1,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		for _, status := range []RestartStatus{
			RestartStatusStopping,
			RestartStatusWaitingRelease,
			RestartStatusStarting,
		} {
			operation, err = store.Transition(operation.OperationID, restartTransition{status: status})
			if err != nil {
				t.Fatalf("Transition(%s) error = %v", status, err)
			}
		}

		abandoned := operation
		abandoned.OperationID = "restart-abandoned-observer"
		if _, err := store.Create(abandoned); err != nil {
			t.Fatalf("create abandoned observation: %v", err)
		}
		// Unreadable historical metadata must not prevent recovery of valid observations or boot.
		if err := os.WriteFile(
			filepath.Join(homePaths.RestartsDir, "corrupt-history.json"),
			[]byte("{"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}

		d, err := New(
			WithHomePaths(homePaths),
			WithConfig(&cfg),
			WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		d.pid = func() int { return 9393 }
		replacementStartedAt := time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC)
		d.processStartedAt = func(int) (time.Time, error) {
			return replacementStartedAt, nil
		}
		d.getenv = func(key string) string {
			if key == RestartOperationEnvKey {
				return operation.OperationID
			}
			if key == "HOME" {
				return homePaths.HomeDir
			}
			return os.Getenv(key)
		}

		helper := newRelaunchHelper(&RelaunchHelperConfig{HomePaths: homePaths, OperationID: operation.OperationID,
			PollInterval: time.Millisecond, ReadyTimeout: time.Millisecond, ExitDrainWait: time.Millisecond})
		childExit := make(chan struct{})
		observed := make(chan error, 1)
		go func() {
			observed <- helper.waitForReady(t.Context(), store, operation.OperationID, restartProcessStub{
				pid: 9393, wait: func() error { <-childExit; return nil },
			})
		}()
		t.Cleanup(func() { close(childExit) })
		timer := time.NewTimer(15 * time.Millisecond)
		defer timer.Stop()
		select {
		case err := <-observed:
			t.Fatalf("restart stopped observing a live boot: %v", err)
		case <-timer.C:
		}
		pending, err := store.Get(operation.OperationID)
		if err != nil || pending.Status != RestartStatusStarting {
			t.Fatalf("pre-boot status = %#v, %v", pending, err)
		}
		if err := d.boot(testutil.Context(t)); err != nil {
			t.Fatalf("boot() error = %v", err)
		}
		t.Cleanup(func() {
			if err := d.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})

		completion := time.NewTimer(10 * time.Second)
		defer completion.Stop()
		select {
		case err := <-observed:
			if err != nil {
				t.Fatalf("observe completed boot: %v", err)
			}
		case <-completion.C:
			t.Fatal("restart observer did not complete")
		}
		reconciled, err := store.Get(abandoned.OperationID)
		if err != nil || reconciled.Status != RestartStatusFailed || reconciled.CompletedAt == nil {
			t.Fatalf("superseded restart = %#v, %v", reconciled, err)
		}

		persisted, err := store.Get(operation.OperationID)
		if err != nil {
			t.Fatalf("store.Get() error = %v", err)
		}
		if got, want := persisted.Status, RestartStatusReady; got != want {
			t.Fatalf("persisted.Status = %q, want %q", got, want)
		}
		if got, want := persisted.NewPID, 9393; got != want {
			t.Fatalf("persisted.NewPID = %d, want %d", got, want)
		}
		info, err := ReadInfo(homePaths.DaemonInfo)
		if err != nil {
			t.Fatalf("ReadInfo(daemon.json) error = %v", err)
		}
		if !info.StartedAt.Equal(replacementStartedAt) {
			t.Fatalf("replacement started_at = %v, want %v", info.StartedAt, replacementStartedAt)
		}
	})
}

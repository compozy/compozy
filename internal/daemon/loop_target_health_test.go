package daemon

import (
	"context"
	"log/slog"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	"github.com/compozy/compozy/internal/store"
)

// Invariant: [loops.breaker] configures a dedicated loop-target service and cannot change the
// shared bridge/MCP/sidecar breaker policy. This daemon wiring suite owns that construction seam.
func TestBootDeadEntityRegistryShouldIsolateLoopTargetPolicy(t *testing.T) {
	t.Parallel()
	t.Run("Should isolate loop-target policy from shared dead-entity policy", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 13, 0, 0, 0, time.UTC)
		registry := &recordingRegistry{}
		state := &bootState{
			registry: registry,
			logger:   slog.Default(),
			cfg: compozyconfig.Config{Loops: compozyconfig.LoopsConfig{
				Breaker: compozyconfig.LoopBreakerConfig{Threshold: 2, ProbeInterval: "30s"},
			}},
		}
		daemon := &Daemon{now: func() time.Time { return now }}
		if err := daemon.bootDeadEntityRegistry(state); err != nil {
			t.Fatalf("bootDeadEntityRegistry() error = %v", err)
		}
		loopService, err := state.loopTargetHealth.current()
		if err != nil {
			t.Fatalf("loopTargetHealth.current() error = %v", err)
		}
		if loopService == state.deadEntities {
			t.Fatal("loop target service shares the generic service instance")
		}

		ctx := context.Background()
		loopKey := store.DeadEntityKey{
			WorkspaceID: "ws-1", Kind: store.DeadEntityKindLoopTarget,
			EntityID: "toolcall:compozy__search",
		}
		sharedKey := store.DeadEntityKey{
			WorkspaceID: "ws-1", Kind: store.DeadEntityKindMCPSidecar, EntityID: "github",
		}
		for range 2 {
			if err := state.loopTargetHealth.RecordFailure(
				ctx, loopKey, deadentity.FailurePermanent, "transport failure",
			); err != nil {
				t.Fatalf("loop RecordFailure() error = %v", err)
			}
			if err := state.deadEntities.RecordFailure(
				ctx, sharedKey, deadentity.FailurePermanent, "sidecar failure",
			); err != nil {
				t.Fatalf("shared RecordFailure() error = %v", err)
			}
		}
		loopStatus, err := state.loopTargetHealth.Status(ctx, loopKey)
		if err != nil {
			t.Fatalf("loop Status() error = %v", err)
		}
		sharedStatus, err := state.deadEntities.Status(ctx, sharedKey)
		if err != nil {
			t.Fatalf("shared Status() error = %v", err)
		}
		if !loopStatus.Dead || sharedStatus.Dead {
			t.Fatalf(
				"loop/shared status = %#v/%#v, want open loop target and live shared runtime",
				loopStatus,
				sharedStatus,
			)
		}
	})
}

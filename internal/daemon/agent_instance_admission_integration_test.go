//go:build integration

package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// Invariant (IT-005, IT-016): pause, counters and held inputs survive a daemon restart;
// a real ACP process receives the retained row once after resume. Owning layer: daemon admission.
func TestInstancePausedRuntimeIntegration(t *testing.T) {
	t.Run("Should retain the stopped main queue and counter across a daemon restart", func(t *testing.T) {
		home, deps, resolved, diagnostics, daemon := newBoundedRebuildFixture(t, true, false)
		db := daemon.registry.(*globaldb.GlobalDB)
		now := time.Now().UTC()
		if err := db.InsertWorkspace(t.Context(), workspacepkg.Workspace{ID: resolved.ID, Name: resolved.Name,
			RootDir: resolved.RootDir, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		newManager := func(d *Daemon, dependencies SessionManagerDeps) *session.Manager {
			registry := d.registry.(*globaldb.GlobalDB)
			manager := newHarnessIntegrationManager(
				t,
				home,
				dependencies,
				resolved,
				session.NewACPDriverAdapter(acp.New(acp.WithProviderPreStarter(d.providerPreStarter))),
				session.WithHostedMCPLauncher(dependencies.HostedMCP),
				session.WithSessionCatalog(registry),
				session.WithSessionInputQueueStore(registry),
				session.WithEventLedger(registry),
			)
			t.Cleanup(func() {
				if err := manager.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			return manager
		}
		manager := newManager(daemon, deps)
		main, err := manager.Create(
			t.Context(),
			&session.CreateOpts{AgentName: "rebuild-agent", Workspace: resolved.ID},
		)
		if err != nil {
			t.Fatal(err)
		}
		promptBoundedRebuild(t, manager, main.ID, "bind before pause")
		instance, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: main.AgentName, WorkspaceID: resolved.ID,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, main.ID, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().
			ExecContext(t.Context(), "UPDATE agent_instance_wake_state SET unattended_turns=50 WHERE instance_id=?", instance.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().Pause(t.Context(), instance.ID, "manual"); err != nil {
			t.Fatal(err)
		}
		if err := manager.Stop(t.Context(), main.ID); err != nil {
			t.Fatal(err)
		}
		held, err := manager.SendPrompt(t.Context(), main.ID, session.SendPromptOpts{
			Message:        "retained peer work",
			MessageID:      "held-peer",
			IdempotencyKey: "held-peer",
			Origin: &acp.PromptOriginMeta{
				Kind:        acp.PromptOriginKindSession,
				SessionID:   "peer",
				WorkspaceID: resolved.ID,
				Hop:         1,
			},
		})
		if err != nil || held.Status != store.SessionPromptResultStatusQueued {
			t.Fatalf("held=%+v %v", held, err)
		}
		if info, err := manager.Status(t.Context(), main.ID); err != nil || info.State != session.StateStopped {
			t.Fatalf("paused wake started runtime: %+v %v", info, err)
		}
		if err := manager.Shutdown(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := daemon.Shutdown(t.Context()); err != nil {
			t.Fatal(err)
		}
		restarted, restartedDeps := bootHarnessPolicyDaemon(t, home, &resolved.Config)
		t.Cleanup(func() {
			if err := restarted.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		db = restarted.registry.(*globaldb.GlobalDB)
		manager = newManager(restarted, restartedDeps)
		wake, err := db.Instances().WakeState(t.Context(), instance.ID)
		if err != nil || wake.UnattendedTurns != 50 {
			t.Fatalf("counter after restart=%+v %v", wake, err)
		}
		if _, found, err := db.PeekNextSessionInput(t.Context(), main.ID); err != nil || found {
			t.Fatalf("paused selection=%v %v", found, err)
		}
		if _, err := db.Instances().Resume(t.Context(), instance.ID, 10, true); err != nil {
			t.Fatal(err)
		}
		state := &bootState{sessions: manager}
		current, err := db.Instances().Get(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := releaseInstanceInputs(t.Context(), state, db.Instances(), current); err != nil {
			t.Fatal(err)
		}
		waitForCondition(t, "retained peer input delivered", func() bool {
			row, err := db.GetSessionInputQueueEntry(t.Context(), main.ID, held.QueueEntryID)
			return err == nil && row.Status == store.SessionInputQueueStatusSent
		})
		if err := releaseInstanceInputs(t.Context(), state, db.Instances(), current); err != nil {
			t.Fatal(err)
		}
		waitForCondition(t, "retained ACP prompt completed", func() bool {
			records, err := acpmock.ReadDiagnostics(diagnostics)
			return err == nil && len(acpmock.PromptDiagnostics(records)) == 2
		})
		wake, err = db.Instances().WakeState(t.Context(), instance.ID)
		if err != nil || wake.UnattendedTurns != 1 {
			t.Fatalf("resumed counter=%+v %v", wake, err)
		}
	})
}

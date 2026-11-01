package globaldb

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

func TestInstanceWorkspaceDeletion(t *testing.T) {
	for _, staged := range []bool{false, true} {
		name := "Should archive instances before deleting their workspace"
		if staged {
			name = "Should archive instances when staging workspace deletion"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "instance-delete", "sess-instance-delete")
			repo := db.Instances()
			input := agentinstance.Instance{
				ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace,
				PermissionCeiling: "{}", CreatedByKind: "human",
			}
			instance, err := repo.Adopt(t.Context(), input, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			report, err := db.InstanceReports().AppendReport(t.Context(), instance.ID, agentinstance.ReportInput{
				SourceKey: "turn:delete", Kind: "failure", Payload: []byte(`{"reason":"failed","detail":"test"}`),
			})
			if err != nil {
				t.Fatal(err)
			}
			remove := db.DeleteWorkspace
			if staged {
				remove = db.StageWorkspaceDeletion
			}
			if err := remove(t.Context(), workspace); err == nil {
				t.Fatal("active main session must block deletion")
			}
			unchanged, err := repo.Get(t.Context(), instance.ID)
			if err != nil || unchanged.ArchivedAt != nil || unchanged.MainSessionID != main {
				t.Fatalf("rejected deletion changed instance: %#v, %v", unchanged, err)
			}
			if _, err := db.DB().
				ExecContext(t.Context(), "UPDATE sessions SET state='stopped' WHERE id=?", main); err != nil {
				t.Fatal(err)
			}
			if err := remove(t.Context(), workspace); err != nil {
				t.Fatal(err)
			}
			archived, err := repo.Get(t.Context(), instance.ID)
			if err != nil || archived.ArchivedAt == nil || archived.MainSessionID != "" {
				t.Fatalf("deleted workspace instance: %#v, %v", archived, err)
			}
			reports, err := db.InstanceReports().ListReports(t.Context(), agentinstance.ReportQuery{
				InstanceID: instance.ID, Limit: 10,
			})
			if err != nil || len(reports) != 1 || reports[0].ID != report.ID {
				t.Fatalf("reports were not preserved: %#v, %v", reports, err)
			}
		})
	}
}

func TestInstanceAdmission(t *testing.T) {
	t.Run("Should admit one concurrent identity and return it even at the cap", func(t *testing.T) {
		t.Parallel()
		db, err := OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), GlobalDatabaseName))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		repo := db.Instances()
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		type outcome struct {
			instance agentinstance.Instance
			created  bool
			err      error
		}
		outcomes := make(chan outcome, 8)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				i, created, err := repo.AdmitCreate(t.Context(), input, 1)
				outcomes <- outcome{i, created, err}
			})
		}
		wg.Wait()
		close(outcomes)
		id := ""
		created := 0
		for o := range outcomes {
			if o.err != nil {
				t.Fatal(o.err)
			}
			if id == "" {
				id = o.instance.ID
			}
			if o.instance.ID != id {
				t.Fatal("concurrent admission created different identities")
			}
			if o.created {
				created++
			}
		}
		if created != 1 {
			t.Fatalf("created %d instances", created)
		}
		reused, wasCreated, err := repo.AdmitCreate(t.Context(), input, 1)
		if err != nil || wasCreated || reused.ID != id {
			t.Fatalf("reuse: %#v %v %v", reused, wasCreated, err)
		}
		input.AgentName = "other"
		if _, _, err := repo.AdmitCreate(t.Context(), input, 1); !errors.Is(err, agentinstance.ErrLimit) {
			t.Fatalf("cap error: %v", err)
		}
	})
}

func TestInstanceSessionRegistration(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		name := "Should atomically register a session with its instance"
		if mismatch {
			name = "Should roll back registration when the instance scope differs"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			instance, _, err := db.Instances().AdmitCreate(t.Context(), agentinstance.Instance{
				ProfileID: store.DefaultProfileID, AgentName: "coder", PermissionCeiling: "{}", CreatedByKind: "human",
			}, 10)
			if err != nil {
				t.Fatal(err)
			}
			info := store.SessionInfo{ID: "sess-instance-register", ProfileID: store.DefaultProfileID,
				AgentName: "coder", State: "starting", RuntimeStatus: store.SessionRuntimeUnbound, SessionType: "user"}
			if mismatch {
				info.AgentName = "other"
			}
			registration, err := db.RegisterInstanceSession(t.Context(), info, instance.ID, nil)
			if mismatch {
				if !errors.Is(err, agentinstance.ErrScopeMismatch) {
					t.Fatalf("registration: %v", err)
				}
				var count int
				if err := db.DB().
					QueryRowContext(t.Context(), "SELECT count(*) FROM sessions WHERE id=?", info.ID).
					Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatal("failed instance registration retained a session")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !registration.Created {
				t.Fatal("new registration must report created")
			}
			caller, err := db.Instances().InstanceCaller(t.Context(), info.ID)
			if err != nil || caller.InstanceID != instance.ID {
				t.Fatalf("unstamped session: %#v, %v", caller, err)
			}
			if repeated, err := db.RegisterInstanceSession(
				t.Context(),
				info,
				instance.ID,
				nil,
			); err != nil ||
				repeated.Created {
				t.Fatalf("repeat registration: %#v, %v", repeated, err)
			}
		})
	}
}

func TestInstanceLifecycle(t *testing.T) {
	t.Run(
		"Should retain a paused instance when resume exceeds the cap and allow recreation after archive",
		func(t *testing.T) {
			t.Parallel()
			db, err := OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), GlobalDatabaseName))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "lifecycle", "sess-instance-lifecycle")
			repo := db.Instances()
			input := agentinstance.Instance{
				ProfileID:         store.DefaultProfileID,
				AgentName:         "coder",
				WorkspaceID:       workspace,
				PermissionCeiling: "{}",
				CreatedByKind:     "human",
			}
			first, _, err := repo.AdmitCreate(t.Context(), input, 1)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repo.BindMain(t.Context(), first.ID, main); err != nil {
				t.Fatal(err)
			}
			var stamped string
			if err := db.DB().
				QueryRowContext(t.Context(), "SELECT instance_id FROM sessions WHERE id = ?", main).
				Scan(&stamped); err != nil ||
				stamped != first.ID {
				t.Fatalf("session instance: %s %v", stamped, err)
			}
			if err := db.DeleteSession(t.Context(), main); !errors.Is(err, store.ErrSessionIsInstanceMain) {
				t.Fatalf("delete main error: %v", err)
			}
			if _, err := db.SetSessionArchived(
				t.Context(),
				workspace,
				main,
				true,
			); !errors.Is(
				err,
				store.ErrSessionIsInstanceMain,
			) {
				t.Fatalf("archive main error: %v", err)
			}
			paused, err := repo.Pause(t.Context(), first.ID, "manual")
			if err != nil || paused.State != "paused" {
				t.Fatalf("pause: %#v %v", paused, err)
			}
			again, err := repo.Pause(t.Context(), first.ID, "fatigue")
			if err != nil || again.PauseReason != "manual" {
				t.Fatalf("repeat pause: %#v %v", again, err)
			}
			input.AgentName = "other"
			other, _, err := repo.AdmitCreate(t.Context(), input, 1)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Resume(t.Context(), first.ID, 1); !errors.Is(err, agentinstance.ErrLimit) {
				t.Fatalf("resume error: %v", err)
			}
			retained, err := repo.Get(t.Context(), first.ID)
			if err != nil || retained.State != "paused" {
				t.Fatalf("retained: %#v %v", retained, err)
			}
			if err := repo.DeleteUnstarted(t.Context(), other.ID); err != nil {
				t.Fatal(err)
			}
			resumed, err := repo.Resume(t.Context(), first.ID, 1)
			if err != nil || resumed.State != "active" || resumed.PauseReason != "" {
				t.Fatalf("resume: %#v %v", resumed, err)
			}
			if _, err := repo.Archive(t.Context(), first.ID); err != nil {
				t.Fatal(err)
			}
			input.AgentName = "coder"
			replacement, created, err := repo.AdmitCreate(t.Context(), input, 1)
			if err != nil || !created || replacement.ID == first.ID {
				t.Fatalf("replacement: %#v %v %v", replacement, created, err)
			}
		},
	)
}

func TestInstanceAdoption(t *testing.T) {
	t.Run("Should roll back failed adoption and atomically adopt a matching root", func(t *testing.T) {
		t.Parallel()
		db, err := OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), GlobalDatabaseName))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "adopt", "sess-adopt")
		repo := db.Instances()
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "wrong",
			WorkspaceID:       workspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		if _, err := repo.Adopt(t.Context(), input, main, 1); err == nil {
			t.Fatal("adopt accepted wrong agent")
		}
		rows, err := repo.List(
			t.Context(),
			agentinstance.ListQuery{ProfileID: store.DefaultProfileID, WorkspaceID: workspace, Limit: 10},
		)
		if err != nil || len(rows) != 0 {
			t.Fatalf("failed adoption leaked rows: %v %v", rows, err)
		}
		input.AgentName = "coder"
		adopted, err := repo.Adopt(t.Context(), input, main, 1)
		if err != nil {
			t.Fatal(err)
		}
		caller, err := repo.InstanceCaller(t.Context(), main)
		if err != nil || caller.InstanceID != adopted.ID || caller.AgentName != "coder" {
			t.Fatalf("caller: %#v %v", caller, err)
		}
		if adopted.State != "active" || adopted.MainSessionID != main {
			t.Fatalf("adopted: %#v", adopted)
		}
		if _, err := repo.Adopt(t.Context(), input, main, 1); !errors.Is(err, agentinstance.ErrExists) {
			t.Fatalf("duplicate: %v", err)
		}
		var instanceID string
		if err := db.DB().
			QueryRowContext(t.Context(), "SELECT instance_id FROM sessions WHERE id=?", main).
			Scan(&instanceID); err != nil ||
			instanceID != adopted.ID {
			t.Fatalf("stamp: %s %v", instanceID, err)
		}
	})
}

func TestInstanceMissingMain(t *testing.T) {
	t.Run(
		"Should publish a missing main without changing pause state or replacing a concurrent winner",
		func(t *testing.T) {
			t.Parallel()
			db, err := OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), GlobalDatabaseName))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "missing-main", "sess-missing-main")
			repo := db.Instances()
			input := agentinstance.Instance{
				ProfileID:         store.DefaultProfileID,
				AgentName:         "coder",
				WorkspaceID:       workspace,
				PermissionCeiling: "{}",
				CreatedByKind:     "human",
			}
			instance, _, err := repo.AdmitCreate(t.Context(), input, 1)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repo.BindMain(t.Context(), instance.ID, main); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pause(t.Context(), instance.ID, "fatigue"); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().
				ExecContext(t.Context(), "UPDATE agent_instances SET main_session_id=NULL WHERE id=?", instance.ID); err != nil {
				t.Fatal(err)
			}
			restored, err := repo.PublishMissingMain(t.Context(), instance.ID, main)
			if err != nil {
				t.Fatal(err)
			}
			if restored.State != "paused" || restored.PauseReason != "fatigue" || restored.MainSessionID != main {
				t.Fatalf("restored: %#v", restored)
			}
			winner, err := repo.PublishMissingMain(t.Context(), instance.ID, "nonexistent-loser")
			if err != nil || winner.MainSessionID != main {
				t.Fatalf("winner: %#v %v", winner, err)
			}
			page, err := repo.ReconciliationPage(t.Context(), "", 1)
			if err != nil || len(page) != 1 {
				t.Fatalf("page: %v %v", page, err)
			}
			next, err := repo.ReconciliationPage(t.Context(), page[0].ID, 1)
			if err != nil || len(next) != 0 {
				t.Fatalf("next: %v %v", next, err)
			}
		},
	)
}

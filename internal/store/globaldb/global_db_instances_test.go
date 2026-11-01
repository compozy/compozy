package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
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
			if _, err := repo.Resume(t.Context(), first.ID, 1, true); !errors.Is(err, agentinstance.ErrLimit) {
				t.Fatalf("resume error: %v", err)
			}
			retained, err := repo.Get(t.Context(), first.ID)
			if err != nil || retained.State != "paused" {
				t.Fatalf("retained: %#v %v", retained, err)
			}
			if err := repo.DeleteUnstarted(t.Context(), other.ID); err != nil {
				t.Fatal(err)
			}
			resumed, err := repo.Resume(t.Context(), first.ID, 1, true)
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

func TestInstanceConcurrentLimit(t *testing.T) {
	t.Run("Should admit exactly ten distinct identities under concurrent creation", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		repo := db.Instances()
		results := make(chan error, 11)
		var wg sync.WaitGroup
		for n := range 11 {
			wg.Go(func() {
				_, created, err := repo.AdmitCreate(t.Context(), agentinstance.Instance{
					ProfileID: store.DefaultProfileID, AgentName: fmt.Sprintf("coder-%d", n),
					PermissionCeiling: "{}", CreatedByKind: "human",
				}, 10)
				if err == nil && !created {
					err = errors.New("distinct identity was not created")
				}
				results <- err
			})
		}
		wg.Wait()
		close(results)
		admitted, limited := 0, 0
		for err := range results {
			switch {
			case err == nil:
				admitted++
			case errors.Is(err, agentinstance.ErrLimit):
				limited++
			default:
				t.Fatalf("admission: %v", err)
			}
		}
		if admitted != 10 || limited != 1 {
			t.Fatalf("admitted=%d limited=%d", admitted, limited)
		}
		rows, err := repo.List(
			t.Context(),
			agentinstance.ListQuery{ProfileID: store.DefaultProfileID, AllWorkspaces: true, Limit: 100},
		)
		if err != nil || len(rows) != 10 {
			t.Fatalf("persisted instances: %d, %v", len(rows), err)
		}
	})
}

func TestInstanceWorkspaceIsolation(t *testing.T) {
	t.Run("Should apply agent and state filters before the instance page cut", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		repo := db.Instances()
		for n := range 4 {
			if _, _, err := repo.AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: fmt.Sprintf("filter-%d", n), PermissionCeiling: "{}", CreatedByKind: "human"}, 10); err != nil {
				t.Fatal(err)
			}
		}
		all, err := repo.List(t.Context(), agentinstance.ListQuery{ProfileID: store.DefaultProfileID, Limit: 10})
		if err != nil || len(all) != 4 {
			t.Fatalf("seed rows=%v err=%v", all, err)
		}
		last := all[len(all)-1]
		if err := db.RegisterSession(t.Context(), store.SessionInfo{ID: "sess-filter-main", ProfileID: store.DefaultProfileID, AgentName: last.AgentName, State: "active", RuntimeStatus: store.SessionRuntimeUnbound, CreatedAt: last.CreatedAt, UpdatedAt: last.UpdatedAt}); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.BindMain(t.Context(), last.ID, "sess-filter-main"); err != nil {
			t.Fatal(err)
		}
		if paused, err := repo.Pause(t.Context(), last.ID, "manual"); err != nil || paused.State != "paused" {
			t.Fatalf("pause=%+v, %v", paused, err)
		}
		for _, query := range []agentinstance.ListQuery{
			{AgentName: last.AgentName}, {State: "paused"}, {AgentName: last.AgentName, State: "paused"},
		} {
			query.ProfileID, query.Limit = store.DefaultProfileID, 1
			rows, err := repo.List(t.Context(), query)
			if err != nil || len(rows) != 1 || rows[0].ID != last.ID {
				t.Fatalf("filtered page %+v: %v, %v", query, rows, err)
			}
			query.AfterID = last.ID
			rows, err = repo.List(t.Context(), query)
			if err != nil || len(rows) != 0 {
				t.Fatalf("continuation=%v, %v", rows, err)
			}
		}
	})

	t.Run("Should pause one workspace without changing the other instance", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		repo := db.Instances()
		firstWorkspace, firstMain := registerHeartbeatWorkspaceAndSession(
			t,
			db,
			"isolated-first",
			"sess-isolated-first",
		)
		secondWorkspace, secondMain := registerHeartbeatWorkspaceAndSession(
			t,
			db,
			"isolated-second",
			"sess-isolated-second",
		)
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			WorkspaceID:       firstWorkspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		first, err := repo.Adopt(t.Context(), input, firstMain, 10)
		if err != nil {
			t.Fatal(err)
		}
		input.WorkspaceID = secondWorkspace
		second, err := repo.Adopt(t.Context(), input, secondMain, 10)
		if err != nil {
			t.Fatal(err)
		}
		if first.ID == second.ID {
			t.Fatal("workspaces shared an instance")
		}
		paused, err := repo.Pause(t.Context(), first.ID, "manual")
		if err != nil || paused.State != "paused" {
			t.Fatalf("pause: %#v, %v", paused, err)
		}
		unchanged, err := repo.Get(t.Context(), second.ID)
		if err != nil || !reflect.DeepEqual(unchanged, second) {
			t.Fatalf("other instance changed: %#v, %v", unchanged, err)
		}
	})
}

// instanceServiceAdmission supplies definition I/O while lifecycle writes use SQLite.
type instanceServiceAdmission struct {
	input          agentinstance.Instance
	resumeErr      error
	validateResume func(context.Context, agentinstance.Instance) error
}

var _ agentinstance.Admission = instanceServiceAdmission{}

func (a instanceServiceAdmission) ResolveCreate(
	context.Context,
	agentinstance.CreateRequest,
) (agentinstance.Instance, error) {
	return a.input, nil
}
func (a instanceServiceAdmission) ValidateResume(ctx context.Context, instance agentinstance.Instance) error {
	if a.validateResume != nil {
		return a.validateResume(ctx, instance)
	}
	return a.resumeErr
}

func (a instanceServiceAdmission) ResolveAdopt(
	context.Context,
	agentinstance.AdoptRequest,
) (agentinstance.Instance, error) {
	return a.input, nil
}

type instanceServiceStarter struct {
	stop    func(context.Context, string) error
	start   func(context.Context, agentinstance.StartMainSession) (string, error)
	outcome agentinstance.StartOutcome
	discard func(context.Context, agentinstance.Instance, string) error
}

var _ agentinstance.MainSessionStarter = (*instanceServiceStarter)(nil)
var _ agentinstance.ReconciliationSessions = (*instanceServiceStarter)(nil)

func (s *instanceServiceStarter) Start(ctx context.Context, request agentinstance.StartMainSession) (string, error) {
	return s.start(ctx, request)
}
func (s *instanceServiceStarter) Stop(ctx context.Context, id string) error {
	if s.stop != nil {
		return s.stop(ctx, id)
	}
	return nil
}

func (s *instanceServiceStarter) StartOutcome(
	context.Context,
	agentinstance.Instance,
) (agentinstance.StartOutcome, error) {
	return s.outcome, nil
}

func (s *instanceServiceStarter) DiscardFailedStart(
	ctx context.Context,
	instance agentinstance.Instance,
	id string,
) error {
	if s.discard != nil {
		return s.discard(ctx, instance, id)
	}
	return nil
}

func TestInstanceServiceRecovery(t *testing.T) {
	t.Run("Should remove a failed start without a session and allow retry", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "retry-start", "sess-retry-start")
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			WorkspaceID:       workspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		failed := errors.New("provider start failed")
		starter := &instanceServiceStarter{
			start: func(context.Context, agentinstance.StartMainSession) (string, error) { return "", failed },
		}
		service, err := agentinstance.NewService(
			agentinstance.WithStore(db.Instances()),
			agentinstance.WithAdmission(instanceServiceAdmission{input: input}),
			agentinstance.WithMainSessions(starter),
			agentinstance.WithActiveLimit(10),
		)
		if err != nil {
			t.Fatal(err)
		}
		first, _, err := service.Create(t.Context(), agentinstance.CreateRequest{})
		if !errors.Is(err, failed) {
			t.Fatalf("create error: %v", err)
		}
		rows, err := db.Instances().
			List(t.Context(), agentinstance.ListQuery{ProfileID: store.DefaultProfileID, AllWorkspaces: true, Limit: 10})
		if err != nil || len(rows) != 0 {
			t.Fatalf("failed start retained rows: %#v, %v", rows, err)
		}
		starts := 0
		starter.start = func(context.Context, agentinstance.StartMainSession) (string, error) { starts++; return main, nil }
		retry, created, err := service.Create(t.Context(), agentinstance.CreateRequest{})
		if err != nil || !created || retry.ID == first.ID || retry.MainSessionID != main {
			t.Fatalf("retry: %#v, %v, %v", retry, created, err)
		}
		again, created, err := service.Create(t.Context(), agentinstance.CreateRequest{})
		if err != nil || created || again.ID != retry.ID || starts != 1 {
			t.Fatalf("repeat: %#v, %v, %v, starts=%d", again, created, err, starts)
		}
	})
	t.Run("Should bind an accepted session after interruption without starting another", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "recover-start", "sess-recover-start")
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			WorkspaceID:       workspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		pending, _, err := db.Instances().AdmitCreate(t.Context(), input, 10)
		if err != nil {
			t.Fatal(err)
		}
		starter := &instanceServiceStarter{
			outcome: agentinstance.StartOutcome{State: "accepted", SessionID: main},
			start: func(context.Context, agentinstance.StartMainSession) (string, error) {
				t.Error("reconciliation started another session")
				return "", errors.New("unexpected start")
			},
		}
		service, err := agentinstance.NewService(
			agentinstance.WithStore(db.Instances()),
			agentinstance.WithAdmission(instanceServiceAdmission{input: input}),
			agentinstance.WithMainSessions(starter),
			agentinstance.WithActiveLimit(10),
		)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := service.Reconcile(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		recovered, err := db.Instances().Get(t.Context(), pending.ID)
		if err != nil || recovered.State != "active" || recovered.MainSessionID != main {
			t.Fatalf("recovered: %#v, %v", recovered, err)
		}
	})
	t.Run("Should preserve a paused instance when its definition is missing", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "missing-definition", "sess-missing-definition")
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			WorkspaceID:       workspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		instance, err := db.Instances().Adopt(t.Context(), input, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		paused, err := db.Instances().Pause(t.Context(), instance.ID, "manual")
		if err != nil {
			t.Fatal(err)
		}
		service, err := agentinstance.NewService(
			agentinstance.WithStore(db.Instances()),
			agentinstance.WithAdmission(
				instanceServiceAdmission{input: input, resumeErr: agentinstance.ErrDefinitionMissing},
			),
			agentinstance.WithMainSessions(&instanceServiceStarter{}),
			agentinstance.WithActiveLimit(10),
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Resume(t.Context(), instance.ID, workspaceaccess.ActorRef{Operator: true}); !errors.Is(err, agentinstance.ErrDefinitionMissing) {
			t.Fatalf("resume: %v", err)
		}
		retained, err := db.Instances().Get(t.Context(), instance.ID)
		if err != nil || !reflect.DeepEqual(retained, paused) {
			t.Fatalf("failed resume changed instance: %#v, %v", retained, err)
		}
	})
}

func TestInstanceServiceConcurrentCreate(t *testing.T) {
	t.Run("Should start one main session for concurrent creates of the same identity", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "concurrent-service", "sess-concurrent-service")
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			WorkspaceID:       workspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		var starts atomic.Int32
		starter := &instanceServiceStarter{
			start: func(context.Context, agentinstance.StartMainSession) (string, error) { starts.Add(1); return main, nil },
		}
		service, err := agentinstance.NewService(
			agentinstance.WithStore(db.Instances()),
			agentinstance.WithAdmission(instanceServiceAdmission{input: input}),
			agentinstance.WithMainSessions(starter),
			agentinstance.WithActiveLimit(1),
		)
		if err != nil {
			t.Fatal(err)
		}
		type result struct {
			instance agentinstance.Instance
			created  bool
			err      error
		}
		results := make(chan result, 8)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				i, created, err := service.Create(t.Context(), agentinstance.CreateRequest{})
				results <- result{i, created, err}
			})
		}
		wg.Wait()
		close(results)
		id, created := "", 0
		for result := range results {
			if result.err != nil {
				t.Fatal(result.err)
			}
			if id == "" {
				id = result.instance.ID
			}
			if result.instance.ID != id {
				t.Fatal("different instance identities")
			}
			if result.created {
				created++
			}
		}
		if created != 1 || starts.Load() != 1 {
			t.Fatalf("created=%d starts=%d", created, starts.Load())
		}
		bound, err := db.Instances().Get(t.Context(), id)
		if err != nil || bound.MainSessionID != main || bound.State != "active" {
			t.Fatalf("bound: %#v, %v", bound, err)
		}
	})
}

func TestInstanceServiceRetainedFailure(t *testing.T) {
	t.Run("Should archive a failed attempt before removing its instance and permit retry", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "retained-start", "sess-retained-start")
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			WorkspaceID:       workspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		failure := errors.New("provider failed after registration")
		starter := &instanceServiceStarter{outcome: agentinstance.StartOutcome{State: "failed", SessionID: main}}
		starter.start = func(ctx context.Context, request agentinstance.StartMainSession) (string, error) {
			_, err := db.DB().
				ExecContext(ctx, "UPDATE sessions SET instance_id=?, state='stopped' WHERE id=?", request.Instance.ID, main)
			if err != nil {
				return "", err
			}
			return main, failure
		}
		discarded := 0
		starter.discard = func(ctx context.Context, _ agentinstance.Instance, id string) error {
			discarded++
			_, err := db.DB().ExecContext(ctx, "UPDATE sessions SET archived_at='2026-01-01T00:00:00Z' WHERE id=?", id)
			return err
		}
		service, err := agentinstance.NewService(
			agentinstance.WithStore(db.Instances()),
			agentinstance.WithAdmission(instanceServiceAdmission{input: input}),
			agentinstance.WithMainSessions(starter),
			agentinstance.WithActiveLimit(1),
		)
		if err != nil {
			t.Fatal(err)
		}
		failed, _, err := service.Create(t.Context(), agentinstance.CreateRequest{})
		if !errors.Is(err, failure) {
			t.Fatalf("start: %v", err)
		}
		retained, err := db.Instances().Get(t.Context(), failed.ID)
		if err != nil || retained.State != "starting" {
			t.Fatalf("retained: %#v, %v", retained, err)
		}
		for range 2 {
			if err := service.Reconcile(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		if discarded != 1 {
			t.Fatalf("discard calls: %d", discarded)
		}
		var detached, archived bool
		if err := db.DB().
			QueryRowContext(t.Context(), "SELECT instance_id IS NULL, archived_at IS NOT NULL FROM sessions WHERE id=?", main).
			Scan(&detached, &archived); err != nil {
			t.Fatal(err)
		}
		if !detached || !archived {
			t.Fatalf("failed attempt: detached=%v archived=%v", detached, archived)
		}
		newMain := "sess-retained-retry"
		if err := db.RegisterSession(
			t.Context(),
			store.SessionInfo{
				ID:            newMain,
				ProfileID:     store.DefaultProfileID,
				AgentName:     "coder",
				WorkspaceID:   workspace,
				State:         "active",
				RuntimeStatus: store.SessionRuntimeUnbound,
				CreatedAt:     failed.CreatedAt,
				UpdatedAt:     failed.CreatedAt,
			},
		); err != nil {
			t.Fatal(err)
		}
		starter.start = func(context.Context, agentinstance.StartMainSession) (string, error) { return newMain, nil }
		retry, created, err := service.Create(t.Context(), agentinstance.CreateRequest{})
		if err != nil || !created || retry.ID == failed.ID || retry.MainSessionID != newMain {
			t.Fatalf("retry: %#v, %v, %v", retry, created, err)
		}
	})
}

func TestInstanceServiceRecreation(t *testing.T) {
	t.Run("Should create a new main session after archiving while retaining the previous session", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, firstMain := registerHeartbeatWorkspaceAndSession(t, db, "recreation", "sess-recreation-first")
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			WorkspaceID:       workspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		nextMain := firstMain
		starter := &instanceServiceStarter{
			start: func(context.Context, agentinstance.StartMainSession) (string, error) { return nextMain, nil },
		}
		service, err := agentinstance.NewService(
			agentinstance.WithStore(db.Instances()),
			agentinstance.WithAdmission(instanceServiceAdmission{input: input}),
			agentinstance.WithMainSessions(starter),
			agentinstance.WithActiveLimit(1),
		)
		if err != nil {
			t.Fatal(err)
		}
		first, created, err := service.Create(t.Context(), agentinstance.CreateRequest{})
		if err != nil || !created {
			t.Fatalf("first create: %#v, %v, %v", first, created, err)
		}
		stopErr := errors.New("stop unavailable")
		starter.stop = func(context.Context, string) error { return stopErr }
		if err := service.Archive(t.Context(), first.ID); !errors.Is(err, stopErr) {
			t.Fatalf("archive stop error = %v", err)
		}
		retryable, err := db.Instances().Get(t.Context(), first.ID)
		if err != nil || retryable.ArchivedAt != nil || retryable.MainSessionID != firstMain {
			t.Fatalf("failed stop lost retryable instance: %+v, %v", retryable, err)
		}
		starter.stop = nil
		if err := service.Archive(t.Context(), first.ID); err != nil {
			t.Fatal(err)
		}
		nextMain = "sess-recreation-second"
		if err := db.RegisterSession(
			t.Context(),
			store.SessionInfo{
				ID:            nextMain,
				ProfileID:     store.DefaultProfileID,
				AgentName:     "coder",
				WorkspaceID:   workspace,
				State:         "active",
				RuntimeStatus: store.SessionRuntimeUnbound,
				CreatedAt:     first.CreatedAt,
				UpdatedAt:     first.CreatedAt,
			},
		); err != nil {
			t.Fatal(err)
		}
		second, created, err := service.Create(t.Context(), agentinstance.CreateRequest{})
		if err != nil || !created || second.ID == first.ID || second.MainSessionID != nextMain {
			t.Fatalf("recreation: %#v, %v, %v", second, created, err)
		}
		archived, err := db.Instances().Get(t.Context(), first.ID)
		if err != nil || archived.ArchivedAt == nil || archived.MainSessionID != firstMain {
			t.Fatalf("archive: %#v, %v", archived, err)
		}
		var retained string
		if err := db.DB().
			QueryRowContext(t.Context(), "SELECT instance_id FROM sessions WHERE id=?", firstMain).
			Scan(&retained); err != nil {
			t.Fatal(err)
		}
		if retained != first.ID {
			t.Fatalf("previous session changed instance: %s", retained)
		}
	})
}

func TestInstanceAdoptionHistory(t *testing.T) {
	t.Run("Should preserve a live session transcript through adoption and reopening", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "adopt-history", "sess-adopt-history")
		owner := store.SessionDBOwner{SessionID: main, WorkspaceID: workspace}
		path := filepath.Join(t.TempDir(), "session.db")
		ledger, err := sessiondb.OpenSessionDB(t.Context(), owner, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := ledger.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		timestamp := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
		for n, content := range []string{"Original request", "Original reply"} {
			if err := ledger.Record(
				t.Context(),
				store.SessionEvent{
					ID:        fmt.Sprintf("adopt-event-%d", n),
					SessionID: main,
					TurnID:    "turn-before-adoption",
					Type:      "agent.message",
					AgentName: "coder",
					Content:   content,
					Timestamp: timestamp,
				},
			); err != nil {
				t.Fatal(err)
			}
		}
		before, err := ledger.Query(t.Context(), store.EventQuery{Limit: 10})
		if err != nil || len(before) != 2 {
			t.Fatalf("history before adoption: %#v, %v", before, err)
		}
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			WorkspaceID:       workspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		starter := &instanceServiceStarter{
			start: func(context.Context, agentinstance.StartMainSession) (string, error) {
				t.Error("adoption started a replacement session")
				return "", errors.New("unexpected start")
			},
		}
		service, err := agentinstance.NewService(
			agentinstance.WithStore(db.Instances()),
			agentinstance.WithAdmission(instanceServiceAdmission{input: input}),
			agentinstance.WithMainSessions(starter),
			agentinstance.WithActiveLimit(10),
		)
		if err != nil {
			t.Fatal(err)
		}
		adopted, err := service.Adopt(
			t.Context(),
			agentinstance.AdoptRequest{
				Actor:     workspaceaccess.ActorRef{Operator: true},
				SessionID: main,
			},
		)
		if err != nil || adopted.MainSessionID != main {
			t.Fatalf("adoption: %#v, %v", adopted, err)
		}
		after, err := ledger.Query(t.Context(), store.EventQuery{Limit: 10})
		if err != nil || !reflect.DeepEqual(after, before) {
			t.Fatalf("history after adoption: %#v, %v", after, err)
		}
		if err := ledger.Close(t.Context()); err != nil {
			t.Fatal(err)
		}
		reopened, err := sessiondb.OpenSessionDB(t.Context(), owner, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		persisted, err := reopened.Query(t.Context(), store.EventQuery{Limit: 10})
		if err != nil || !reflect.DeepEqual(persisted, before) {
			t.Fatalf("reopened history: %#v, %v", persisted, err)
		}
	})
}

func TestInstanceWorkspaceArchive(t *testing.T) {
	for _, state := range []string{"starting", "active", "paused"} {
		t.Run("Should block workspace archive for a "+state+" instance until it is archived", func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			workspaceID, main := registerHeartbeatWorkspaceAndSession(
				t,
				db,
				"archive-instance",
				"sess-archive-instance",
			)
			input := agentinstance.Instance{
				ProfileID:         store.DefaultProfileID,
				AgentName:         "coder",
				WorkspaceID:       workspaceID,
				PermissionCeiling: "{}",
				CreatedByKind:     "human",
			}
			instance, _, err := db.Instances().AdmitCreate(t.Context(), input, 10)
			if err != nil {
				t.Fatal(err)
			}
			if state != "starting" {
				if _, err := db.Instances().BindMain(t.Context(), instance.ID, main); err != nil {
					t.Fatal(err)
				}
			}
			if state == "paused" {
				if _, err := db.Instances().Pause(t.Context(), instance.ID, "manual"); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.DB().
				ExecContext(t.Context(), "UPDATE sessions SET state='stopped' WHERE id=?", main); err != nil {
				t.Fatal(err)
			}
			_, err = db.SetWorkspaceArchived(t.Context(), workspaceID, true)
			busy, ok := errors.AsType[*workspace.BusyError](err)
			if !ok || len(busy.Blockers) != 1 || busy.Blockers[0].Kind != "instance" ||
				busy.Blockers[0].ID != instance.ID ||
				busy.Blockers[0].Name != "coder" {
				t.Fatalf("archive blockers: %#v, %v", busy, err)
			}
			var archived bool
			if err := db.DB().
				QueryRowContext(t.Context(), "SELECT archived_at IS NOT NULL FROM workspaces WHERE id=?", workspaceID).
				Scan(&archived); err != nil {
				t.Fatal(err)
			}
			if archived {
				t.Fatal("blocked workspace was archived")
			}
			if _, err := db.Instances().Archive(t.Context(), instance.ID); err != nil {
				t.Fatal(err)
			}
			result, err := db.SetWorkspaceArchived(t.Context(), workspaceID, true)
			if err != nil || result.ArchivedAt == nil {
				t.Fatalf("archive after instance: %#v, %v", result, err)
			}
		})
	}
}

func TestInstanceCallerLineage(t *testing.T) {
	t.Run("Should inherit instance depth while retaining each descendants own permission ceiling", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspaceID, main := registerHeartbeatWorkspaceAndSession(t, db, "caller-lineage", "sess-lineage-main")
		instance, err := db.Instances().
			Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspaceID, PermissionCeiling: "{}", CreatedByKind: "human", Depth: 1}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		parent := main
		for _, id := range []string{"sess-lineage-child", "sess-lineage-grandchild"} {
			if err := db.RegisterSession(
				t.Context(),
				store.SessionInfo{
					ID:            id,
					ProfileID:     store.DefaultProfileID,
					AgentName:     "coder",
					WorkspaceID:   workspaceID,
					State:         "active",
					RuntimeStatus: store.SessionRuntimeUnbound,
					CreatedAt:     instance.CreatedAt,
					UpdatedAt:     instance.CreatedAt,
				},
			); err != nil {
				t.Fatal(err)
			}
			policy := `{"tools":["read"],"skills":["review"],"mcp_servers":["docs"],"workspace_paths":["/project"]}`
			if _, err := db.DB().
				ExecContext(t.Context(), "UPDATE sessions SET parent_session_id=?, permission_policy_json=? WHERE id=?", parent, policy, id); err != nil {
				t.Fatal(err)
			}
			caller, err := db.Instances().InstanceCaller(t.Context(), id)
			expected := store.SessionPermissionPolicy{
				Tools:          []string{"read"},
				Skills:         []string{"review"},
				MCPServers:     []string{"docs"},
				WorkspacePaths: []string{"/project"},
			}
			if err != nil || caller.InstanceID != instance.ID || caller.InstanceDepth != 1 || caller.Grant != "" ||
				!reflect.DeepEqual(caller.PermissionPolicy, expected) {
				t.Fatalf("descendant authority: %#v, %v", caller, err)
			}
			parent = id
		}
		attention, err := db.Instances().NeedsAttention(t.Context(), instance.ID)
		if err != nil || attention {
			t.Fatalf("initial attention=%v, %v", attention, err)
		}
		if _, err := db.DB().ExecContext(t.Context(), `INSERT INTO session_pending_interactions (interaction_id,session_id,kind,provider_request_id,status,created_at) VALUES ('instance-attention',?,'permission','provider-request','pending',?)`, parent, store.FormatTimestamp(instance.CreatedAt)); err != nil {
			t.Fatal(err)
		}
		attention, err = db.Instances().NeedsAttention(t.Context(), instance.ID)
		if err != nil || !attention {
			t.Fatalf("descendant attention=%v, %v", attention, err)
		}
		if _, err := db.DB().ExecContext(t.Context(), "UPDATE session_pending_interactions SET status='resolved' WHERE interaction_id='instance-attention'"); err != nil {
			t.Fatal(err)
		}
		attention, err = db.Instances().NeedsAttention(t.Context(), instance.ID)
		if err != nil || attention {
			t.Fatalf("resolved attention=%v, %v", attention, err)
		}
		if _, err := db.DB().ExecContext(t.Context(), `INSERT INTO agent_instance_reports (id,instance_id,source_key,kind,payload,created_at) VALUES ('attention-report',?,'attention-report','failure','{}',?)`, instance.ID, store.FormatTimestamp(instance.CreatedAt)); err != nil {
			t.Fatal(err)
		}
		attention, err = db.Instances().NeedsAttention(t.Context(), instance.ID)
		if err != nil || !attention {
			t.Fatalf("failure report attention=%v, %v", attention, err)
		}

	})
}

func TestInstanceEmptyStartCleanup(t *testing.T) {
	t.Run("Should remove an unstarted instance when the provider returns no session identity", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		input := agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}
		starter := &instanceServiceStarter{
			start: func(context.Context, agentinstance.StartMainSession) (string, error) { return "", nil },
		}
		service, err := agentinstance.NewService(
			agentinstance.WithStore(db.Instances()),
			agentinstance.WithAdmission(instanceServiceAdmission{input: input}),
			agentinstance.WithMainSessions(starter),
			agentinstance.WithActiveLimit(1),
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := service.Create(t.Context(), agentinstance.CreateRequest{}); err == nil {
			t.Fatal("empty provider identity was accepted")
		}
		rows, err := db.Instances().
			List(t.Context(), agentinstance.ListQuery{ProfileID: store.DefaultProfileID, AllWorkspaces: true, Limit: 10})
		if err != nil || len(rows) != 0 {
			t.Fatalf("unstarted row prevents retry: %#v, %v", rows, err)
		}
	})
}

func TestInstanceDefinitionReconciliation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
		reason  string
	}{
		{"Should pause an instance whose definition was removed", agentinstance.ErrDefinitionMissing, "definition_missing"},
		{"Should pause an instance whose definition became on demand", agentinstance.ErrNotStaysOn, "definition_not_stays_on"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "definition-change", "sess-definition-change")
			input := agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}
			instance, err := db.Instances().Adopt(t.Context(), input, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			service, err := agentinstance.NewService(agentinstance.WithStore(db.Instances()), agentinstance.WithAdmission(instanceServiceAdmission{resumeErr: tc.failure}), agentinstance.WithMainSessions(&instanceServiceStarter{}), agentinstance.WithActiveLimit(10))
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := service.Reconcile(t.Context()); err != nil {
					t.Fatal(err)
				}
				retained, err := db.Instances().Get(t.Context(), instance.ID)
				if err != nil {
					t.Fatal(err)
				}
				if retained.State != "paused" || retained.PauseReason != tc.reason || retained.MainSessionID != main {
					t.Fatalf("reconciled instance: %#v", retained)
				}
			}
		})
	}
}

type instanceHookRecorder struct {
	payloads []hooks.AgentInstancePayload
	err      error
}

var _ agentinstance.LifecycleHooks = (*instanceHookRecorder)(nil)

func (r *instanceHookRecorder) DispatchAgentInstance(_ context.Context, p hooks.AgentInstancePayload) (hooks.AgentInstancePayload, error) {
	r.payloads = append(r.payloads, p)
	return p, r.err
}

func TestInstanceLifecycleHooks(t *testing.T) {
	t.Run("Should observe committed lifecycle payloads without repeating sequential pauses", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "instance-hooks", "sess-instance-hooks")
		input := agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}
		recorder := &instanceHookRecorder{}
		service, err := agentinstance.NewService(agentinstance.WithStore(db.Instances()), agentinstance.WithAdmission(instanceServiceAdmission{input: input}), agentinstance.WithMainSessions(&instanceServiceStarter{start: func(context.Context, agentinstance.StartMainSession) (string, error) { return main, nil }}), agentinstance.WithActiveLimit(10), agentinstance.WithLifecycleHooks(recorder))
		if err != nil {
			t.Fatal(err)
		}
		instance, _, err := service.Create(t.Context(), agentinstance.CreateRequest{})
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err := service.Pause(t.Context(), instance.ID, "manual"); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := service.Resume(t.Context(), instance.ID, workspaceaccess.ActorRef{Operator: true}); err != nil {
			t.Fatal(err)
		}
		if err := service.Archive(t.Context(), instance.ID); err != nil {
			t.Fatal(err)
		}
		want := []hooks.HookEvent{hooks.HookAgentInstanceCreated, hooks.HookAgentInstancePaused, hooks.HookAgentInstanceResumed, hooks.HookAgentInstanceArchived}
		if len(recorder.payloads) != len(want) {
			t.Fatalf("payloads=%#v", recorder.payloads)
		}
		for i, p := range recorder.payloads {
			if p.Event != want[i] || p.InstanceID != instance.ID || p.MainSessionID != main || p.ProfileID != input.ProfileID || p.WorkspaceID != workspace || p.AgentName != "coder" || p.Timestamp.IsZero() {
				t.Fatalf("payload %d: %#v", i, p)
			}
		}
		if recorder.payloads[1].Reason != "manual" || recorder.payloads[1].State != "paused" {
			t.Fatalf("pause payload: %#v", recorder.payloads[1])
		}
	})
}

func TestInstanceAdoptionHooks(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "Should observe an adopted instance after persistence"
		if fail {
			name = "Should retain adoption when its observer fails"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "adopt-hooks", "sess-adopt-hooks")
			input := agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}
			recorder := &instanceHookRecorder{}
			if fail {
				recorder.err = errors.New("observer failed")
			}
			service, err := agentinstance.NewService(agentinstance.WithStore(db.Instances()), agentinstance.WithAdmission(instanceServiceAdmission{input: input}), agentinstance.WithMainSessions(&instanceServiceStarter{}), agentinstance.WithActiveLimit(10), agentinstance.WithLifecycleHooks(recorder))
			if err != nil {
				t.Fatal(err)
			}
			adopted, err := service.Adopt(t.Context(), agentinstance.AdoptRequest{CreateRequest: agentinstance.CreateRequest{Actor: workspaceaccess.ActorRef{Operator: true}}, SessionID: main})
			if err != nil {
				t.Fatal(err)
			}
			retained, err := db.Instances().Get(t.Context(), adopted.ID)
			if err != nil || retained.MainSessionID != main || retained.State != "active" {
				t.Fatalf("adoption: %#v %v", retained, err)
			}
			if len(recorder.payloads) != 1 {
				t.Fatalf("hooks=%d", len(recorder.payloads))
			}
			payload := recorder.payloads[0]
			if payload.Event != hooks.HookAgentInstanceAdopted || payload.InstanceID != adopted.ID || payload.MainSessionID != main || payload.ProfileID != input.ProfileID || payload.WorkspaceID != workspace {
				t.Fatalf("adoption payload: %#v", payload)
			}
			if _, err := service.Adopt(t.Context(), agentinstance.AdoptRequest{CreateRequest: agentinstance.CreateRequest{Actor: workspaceaccess.ActorRef{Operator: true}}, SessionID: main}); !errors.Is(err, agentinstance.ErrExists) {
				t.Fatalf("repeat adoption: %v", err)
			}
			if len(recorder.payloads) != 1 {
				t.Fatal("failed adoption emitted a success hook")
			}
		})
	}
}

// TestInstanceWakeMigration owns upgrade preservation and opt-in safety in the global store.
func TestInstanceWakeMigration(t *testing.T) {
	t.Run("Should preserve legacy events and require opt in only on upgrade", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		old, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00167_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		ctx := t.Context()
		for _, statement := range []string{
			`INSERT INTO agent_instances (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at)
    VALUES ('old-instance','00000000000000000000000000','coder','active','human','','2026-11-01','2026-11-01')`,
			`INSERT INTO agent_heartbeat_wake_events (id,workspace_id,agent_name,source,result,reason,created_at,expires_at)
    VALUES ('user-event','','coder','manual','skipped','heartbeat_disabled','2026-11-01','2027-11-01')`,
		} {
			if _, err := old.ExecContext(ctx, statement); err != nil {
				t.Fatal(err)
			}
		}
		if err := old.Close(); err != nil {
			t.Fatal(err)
		}
		db, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		var enabled int
		var next, occurrence string
		if err := db.db.QueryRowContext(ctx, `SELECT i.checks_enabled,w.next_wake_at,w.occurrence_id
   FROM agent_instances i JOIN agent_instance_wake_state w ON w.instance_id=i.id
   WHERE i.id='old-instance'`).Scan(&enabled, &next, &occurrence); err != nil {
			t.Fatal(err)
		}
		if enabled != 0 || next != "" || occurrence != "" {
			t.Fatalf("upgrade enabled=%d next=%q occurrence=%q", enabled, next, occurrence)
		}
		dispatcher := &instanceRecoveryDispatcher{}
		service, err := heartbeat.NewCheckService(ctx, db.Instances(), instanceCheckEnvironment{}, dispatcher, heartbeat.CheckConfig{Enabled: true, Interval: time.Hour, MaxInterval: 24 * time.Hour, Retention: 24 * time.Hour, Timeout: time.Minute, MaxConcurrent: 2, BatchSize: 10}, func() time.Time { return time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC) })
		if err != nil {
			t.Fatal(err)
		}
		if err := service.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		if err := service.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		if err := service.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		if dispatcher.calls.Load() != 0 {
			t.Fatal("upgrade dispatched triage without opt in")
		}
		var wakeCount int
		if err := db.db.QueryRowContext(ctx, "SELECT count(*) FROM agent_heartbeat_wake_events").Scan(&wakeCount); err != nil {
			t.Fatal(err)
		}
		if wakeCount != 1 {
			t.Fatalf("upgrade created wake events: %d", wakeCount)
		}
		var reason, detail string
		var denied int
		if err := db.db.QueryRowContext(ctx, `SELECT reason,detail,denied_calls FROM agent_heartbeat_wake_events WHERE id='user-event'`).Scan(&reason, &detail, &denied); err != nil {
			t.Fatal(err)
		}
		if reason != "heartbeat_disabled" || detail != "" || denied != 0 {
			t.Fatalf("legacy event: %q %q %d", reason, detail, denied)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_heartbeat_wake_events (id,workspace_id,agent_name,source,result,reason,created_at,expires_at)
   VALUES ('timer-event','','coder','timer','quiet','quiet','2026-11-01','2027-11-01')`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `UPDATE agent_instances SET checks_enabled=1 WHERE id='old-instance'`); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
		reopened, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := reopened.db.QueryRowContext(ctx, `SELECT checks_enabled FROM agent_instances WHERE id='old-instance'`).Scan(&enabled); err != nil {
			t.Fatal(err)
		}
		if enabled != 1 {
			t.Fatal("reopen reset operator opt in")
		}
	})
}

func TestInstanceCollectorFacts(t *testing.T) {
	t.Run("Should derive scope from durable identity and preserve failure cursor ties", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "collector-home", "sess-collector-main")
		outside, other := registerHeartbeatWorkspaceAndSession(t, db, "collector-outside", "sess-collector-other")
		instance, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range []struct{ id, workspace string }{{"a", workspace}, {"b", workspace}, {"outside", outside}} {
			_, err := db.DB().ExecContext(t.Context(), `INSERT INTO automation_runs
    (id,profile_id,workspace_id,status,ended_at) VALUES (?,?,?,'failed','2026-11-01T00:00:00Z')`,
				input.id, store.DefaultProfileID, input.workspace)
			if err != nil {
				t.Fatal(err)
			}
		}
		scope, facts, err := db.Instances().CollectorFacts(t.Context(), instance.ID, heartbeat.CollectorScope{
			ProfileID: "forged", WorkspaceID: outside, MainSessionID: other, Unrestricted: true, ReachableWorkspaces: []string{outside},
		})
		if err != nil {
			t.Fatal(err)
		}
		if scope.ProfileID != store.DefaultProfileID || scope.WorkspaceID != workspace || scope.MainSessionID != main || scope.Unrestricted {
			t.Fatalf("scope=%+v", scope)
		}
		if len(scope.ChainSessionIDs) != 1 || scope.ChainSessionIDs[0] != main || len(facts.Sessions) != 0 {
			t.Fatalf("chain=%v sessions=%v", scope.ChainSessionIDs, facts.Sessions)
		}
		if len(facts.Automations) != 2 || facts.Automations[0].Cursor == facts.Automations[1].Cursor {
			t.Fatalf("failures=%+v", facts.Automations)
		}
		digest, err := heartbeat.CollectDigest(scope, facts, nil, "policy", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(digest.Items) != 2 || digest.Items[0].Key != "auto:a" || digest.Items[1].Key != "auto:b" {
			t.Fatalf("digest=%+v", digest)
		}
	})
}

func TestInstanceWakeTransactions(t *testing.T) {
	t.Run("Should admit once finalize once and retain cancellation across resume", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "wake-transactions", "sess-wake-main")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		req := heartbeat.AdmitRequest{InstanceID: instance.ID, OccurrenceID: "occ-first", Now: now, NextWakeAt: now.Add(time.Hour), MaxConcurrent: 2}
		var wg sync.WaitGroup
		successes := make(chan bool, 8)
		for range 8 {
			wg.Go(func() {
				admitted, err := repo.Admit(t.Context(), req)
				if err != nil {
					t.Error(err)
				}
				successes <- admitted
			})
		}
		wg.Wait()
		close(successes)
		count := 0
		for admitted := range successes {
			if admitted {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("admissions=%d", count)
		}
		finish := heartbeat.FinishRequest{InstanceID: instance.ID, OccurrenceID: req.OccurrenceID, Source: "timer", Result: "quiet", Reason: "quiet", Now: now, ExpiresAt: now.Add(24 * time.Hour), Digest: "digest-one", AutomationCursor: "cursor-one"}
		if changed, err := repo.Finish(t.Context(), finish); err != nil || !changed {
			t.Fatalf("finish=%v %v", changed, err)
		}
		if changed, err := repo.Finish(t.Context(), finish); err != nil || changed {
			t.Fatalf("retry=%v %v", changed, err)
		}
		state, err := repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.QuietStreak != 1 || state.LastDigest != "digest-one" || state.OccurrenceID != "" {
			t.Fatalf("state=%+v", state)
		}
		scheduled := state.NextWakeAt
		req.ManualCheck = true
		req.OccurrenceID = "occ-cancelled"
		if admitted, err := repo.Admit(t.Context(), req); err != nil || !admitted {
			t.Fatalf("manual=%v %v", admitted, err)
		}
		state, err = repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.NextWakeAt != scheduled {
			t.Fatal("manual check moved cadence")
		}
		if _, err := repo.Pause(t.Context(), instance.ID, "manual"); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Resume(t.Context(), instance.ID, 10, true); err != nil {
			t.Fatal(err)
		}
		if admitted, err := repo.Admit(t.Context(), req); err != nil || admitted {
			t.Fatalf("cancelled slot released early: %v %v", admitted, err)
		}
		finish.OccurrenceID = req.OccurrenceID
		finish.Digest = "must-not-commit"
		finish.Report = &agentinstance.ReportInput{Kind: "failure", Payload: []byte(`{"reason":"test","detail":"cancelled"}`)}
		if changed, err := repo.Finish(t.Context(), finish); err != nil || !changed {
			t.Fatalf("cancel finish=%v %v", changed, err)
		}
		state, err = repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.LastDigest != "digest-one" || state.QuietStreak != 1 || state.Cancelled {
			t.Fatalf("cancelled state=%+v", state)
		}
		var events, reports int
		if err := db.DB().QueryRowContext(t.Context(), "SELECT count(*) FROM agent_heartbeat_wake_events WHERE occurrence_id IN ('occ-first','occ-cancelled')").Scan(&events); err != nil {
			t.Fatal(err)
		}
		if err := db.DB().QueryRowContext(t.Context(), "SELECT count(*) FROM agent_instance_reports WHERE instance_id=?", instance.ID).Scan(&reports); err != nil {
			t.Fatal(err)
		}
		if events != 2 || reports != 0 {
			t.Fatalf("events=%d reports=%d", events, reports)
		}
		req.OccurrenceID = "occ-failure"
		if admitted, err := repo.Admit(t.Context(), req); err != nil || !admitted {
			t.Fatalf("failure admission=%v %v", admitted, err)
		}
		finish.OccurrenceID = req.OccurrenceID
		finish.Result = "failed"
		finish.Reason = "invalid-reason"
		if _, err := repo.Finish(t.Context(), finish); err == nil {
			t.Fatal("invalid event accepted")
		}
		state, err = repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.OccurrenceID != req.OccurrenceID || state.FailureStreak != 0 {
			t.Fatalf("failed transaction changed state: %+v", state)
		}
		if err := db.DB().QueryRowContext(t.Context(), "SELECT count(*) FROM agent_instance_reports WHERE instance_id=?", instance.ID).Scan(&reports); err != nil {
			t.Fatal(err)
		}
		if reports != 0 {
			t.Fatal("report escaped rolled back transaction")
		}
		finish.Reason = "triage_failed"
		if changed, err := repo.Finish(t.Context(), finish); err != nil || !changed {
			t.Fatalf("failure finish=%v %v", changed, err)
		}
		state, err = repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.FailureStreak != 1 || state.LastDigest != "digest-one" || state.AutomationCursor != "cursor-one" {
			t.Fatalf("failure state=%+v", state)
		}

	})
}

type instanceCheckEnvironment struct {
	target heartbeat.CheckTarget
	hook   string
	digest heartbeat.CheckDigest
}

func (e instanceCheckEnvironment) ResolveCheck(context.Context, string) (heartbeat.CheckTarget, error) {
	return e.target, nil
}
func (e instanceCheckEnvironment) BeforeCheck(context.Context, heartbeat.CheckTarget, string) (string, error) {
	return e.hook, nil
}
func (e instanceCheckEnvironment) Collect(context.Context, string, heartbeat.CollectorScope, []string, string, string) (heartbeat.CheckDigest, error) {
	return e.digest, nil
}

type instanceRecoveryDispatcher struct {
	stopped []string
	calls   atomic.Int32
}

func (d *instanceRecoveryDispatcher) Dispatch(context.Context, heartbeat.CheckTarget, heartbeat.CheckDigest, func(string) error) (heartbeat.TriageOutcome, error) {
	d.calls.Add(1)
	return heartbeat.TriageOutcome{}, errors.New("unexpected dispatch")
}
func (d *instanceRecoveryDispatcher) Stop(_ context.Context, id string) error {
	d.stopped = append(d.stopped, id)
	return nil
}

func TestInstanceCheckRecovery(t *testing.T) {
	t.Run("Should recover one interruption and never replay its committed outcome", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "wake-recovery", "sess-recovery-main")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		if ok, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: instance.ID, OccurrenceID: "instance_check-recovery", Now: now, ManualCheck: true, MaxConcurrent: 2}); err != nil || !ok {
			t.Fatalf("admit=%v %v", ok, err)
		}
		if ok, err := repo.BindTriage(t.Context(), instance.ID, "instance_check-recovery", "hidden"); err != nil || !ok {
			t.Fatalf("bind=%v %v", ok, err)
		}
		dispatcher := &instanceRecoveryDispatcher{}
		environment := instanceCheckEnvironment{target: heartbeat.CheckTarget{Instance: instance, Policy: heartbeat.ResolvedPolicy{Enabled: true}}}
		config := heartbeat.CheckConfig{Enabled: true, Interval: time.Hour, MaxInterval: 24 * time.Hour, Retention: 24 * time.Hour, Timeout: time.Minute, MaxConcurrent: 2, BatchSize: 10}
		service, err := heartbeat.NewCheckService(t.Context(), repo, environment, dispatcher, config, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		if len(dispatcher.stopped) != 1 || dispatcher.stopped[0] != "hidden" {
			t.Fatalf("stopped=%v", dispatcher.stopped)
		}
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		state, err := repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.OccurrenceID != "" || state.FailureStreak != 1 || state.LastDigest != "" {
			t.Fatalf("recovered=%+v", state)
		}
		var count int
		if err := db.DB().QueryRowContext(t.Context(), "SELECT count(*) FROM agent_instance_reports WHERE source_key='occ:instance_check-recovery'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("reports=%d", count)
		}
		if err := service.Tick(t.Context()); err != nil {
			t.Fatal(err)
		}
		state, err = repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.NextWakeAt == "" || state.OccurrenceID != "" || state.FailureStreak != 1 {
			t.Fatalf("empty digest changed outcome=%+v", state)
		}
		if err := service.Tick(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := db.DB().QueryRowContext(t.Context(), "SELECT count(*) FROM agent_heartbeat_wake_events WHERE reason='digest_empty' AND agent_name='coder'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("empty skips=%d", count)
		}
	})
}

func TestInstanceCheckSlotPrefilter(t *testing.T) {
	t.Run("Should leave a due instance unchanged when its triage slot is occupied", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "wake-slot", "sess-slot-main")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		environment := instanceCheckEnvironment{target: heartbeat.CheckTarget{Instance: instance, Policy: heartbeat.ResolvedPolicy{Enabled: false}}}
		config := heartbeat.CheckConfig{Enabled: true, Interval: time.Hour, MaxInterval: 24 * time.Hour, Retention: 24 * time.Hour, Timeout: time.Minute, MaxConcurrent: 1, BatchSize: 10}
		service, err := heartbeat.NewCheckService(t.Context(), repo, environment, &instanceRecoveryDispatcher{}, config, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		if ok, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: instance.ID, OccurrenceID: "instance_check-slot", Now: now, ManualCheck: true, MaxConcurrent: 1}); err != nil || !ok {
			t.Fatalf("admit=%v %v", ok, err)
		}
		before, err := repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.Check(t.Context(), instance.ID, false)
		if err != nil || result.Gate != "no_slot" {
			t.Fatalf("result=%+v error=%v", result, err)
		}
		if err := service.Tick(t.Context()); err != nil {
			t.Fatal(err)
		}
		after, err := repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Fatalf("occupied slot mutated state: before=%+v after=%+v", before, after)
		}
		var count int
		if err := db.DB().QueryRowContext(t.Context(), "SELECT count(*) FROM agent_heartbeat_wake_events WHERE agent_name='coder'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("occupied slot emitted %d wake events", count)
		}
	})
}

func TestInstanceCheckBackoff(t *testing.T) {
	t.Run("Should persist capped backoff reset on reports and preserve manual cadence", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "wake-backoff", "sess-backoff-main")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		for i, tc := range []struct {
			result string
			delay  time.Duration
			manual bool
		}{
			{"quiet", time.Hour, false}, {"quiet", time.Hour, false}, {"quiet", 2 * time.Hour, false},
			{"quiet", 2 * time.Hour, false}, {"quiet", 2 * time.Hour, false}, {"quiet", 4 * time.Hour, false}, {"quiet", 4 * time.Hour, false},
			{"reported", time.Hour, false}, {"failed", 2 * time.Hour, false}, {"failed", 4 * time.Hour, false},
			{"quiet", 0, true},
		} {
			before, err := repo.WakeState(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			occurrence := fmt.Sprintf("occ-backoff-%d", i)
			ok, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: instance.ID, OccurrenceID: occurrence, Now: now, NextWakeAt: now.Add(time.Hour), ManualCheck: tc.manual, MaxConcurrent: 1})
			if err != nil || !ok {
				t.Fatalf("admit=%v %v", ok, err)
			}
			source := "timer"
			if tc.manual {
				source = "instance_check"
			}
			reason := tc.result
			if reason == "failed" {
				reason = "triage_failed"
			}
			req := heartbeat.FinishRequest{InstanceID: instance.ID, OccurrenceID: occurrence, Source: source, Result: tc.result, Reason: reason, Now: now, ExpiresAt: now.Add(24 * time.Hour), Interval: time.Hour, MaxInterval: 4 * time.Hour}
			if changed, err := repo.Finish(t.Context(), req); err != nil || !changed {
				t.Fatalf("finish=%v %v", changed, err)
			}
			state, err := repo.WakeState(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := store.FormatTimestamp(now.Add(tc.delay))
			if tc.manual {
				want = before.NextWakeAt
			}
			if state.NextWakeAt != want {
				t.Fatalf("step %d next=%s want=%s", i, state.NextWakeAt, want)
			}
			if changed, err := repo.Finish(t.Context(), req); err != nil || changed {
				t.Fatalf("retry=%v %v", changed, err)
			}
			repeated, err := repo.WakeState(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if repeated != state {
				t.Fatal("replayed finish increased backoff")
			}
			now = now.Add(tc.delay)
		}
	})
}

func TestInstanceProposalDedupe(t *testing.T) {
	t.Run("Should suppress matching proposal identities for 24 hours and retain novel proposals", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "wake-dedupe", "sess-dedupe-main")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		for i, tc := range []struct {
			payload string
			hours   int
			reports int
			reason  string
		}{
			{`{"proposals":[{"title":"First","body":"one","item_keys":["b","a"]},{"title":"Review Queue","body":"one","item_keys":[]}]}`, 0, 1, "reported"},
			{`{"proposals":[{"title":"Changed","body":"different","item_keys":["a","b","a"]},{"title":"  REVIEW   queue ","body":"different","item_keys":[]}]}`, 1, 1, "duplicate_suppressed"},
			{`{"proposals":[{"title":"First","body":"one","item_keys":["a","b"]},{"title":"New","body":"one","item_keys":["c"]}]}`, 2, 2, "reported"},
			{`{"proposals":[{"title":"First","body":"one","item_keys":["a","b"]}]}`, 24, 3, "reported"},
		} {
			at := now.Add(time.Duration(tc.hours) * time.Hour)
			occurrence := fmt.Sprintf("occ-dedupe-%d", i)
			admitted, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: instance.ID, OccurrenceID: occurrence, Now: at, ManualCheck: true, MaxConcurrent: 1})
			if err != nil || !admitted {
				t.Fatalf("admit=%v %v", admitted, err)
			}
			changed, err := repo.Finish(t.Context(), heartbeat.FinishRequest{InstanceID: instance.ID, OccurrenceID: occurrence, Source: "instance_check", Result: "reported", Reason: "reported", Now: at, ExpiresAt: at.Add(48 * time.Hour), Report: &agentinstance.ReportInput{Kind: "proposal", Payload: []byte(tc.payload)}})
			if err != nil || !changed {
				t.Fatalf("finish=%v %v", changed, err)
			}
			var count int
			if err := db.DB().QueryRowContext(t.Context(), "SELECT count(*) FROM agent_instance_reports WHERE instance_id=?", instance.ID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != tc.reports {
				t.Fatalf("step %d reports=%d want=%d", i, count, tc.reports)
			}
			var reason string
			if err := db.DB().QueryRowContext(t.Context(), "SELECT reason FROM agent_heartbeat_wake_events WHERE occurrence_id=?", occurrence).Scan(&reason); err != nil {
				t.Fatal(err)
			}
			if reason != tc.reason {
				t.Fatalf("reason=%s want=%s", reason, tc.reason)
			}
			if i == 2 {
				var size int
				if err := db.DB().QueryRowContext(t.Context(), "SELECT json_array_length(payload,'$.proposals') FROM agent_instance_reports WHERE source_key=?", "occ:"+occurrence).Scan(&size); err != nil {
					t.Fatal(err)
				}
				if size != 1 {
					t.Fatalf("partial duplicate retained %d proposals", size)
				}
			}
		}
	})
}

func TestInstanceCheckFatigue(t *testing.T) {
	t.Run("Should pause at the failure limit and persist one fatigue report on retry", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "wake-fatigue", "sess-fatigue-main")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		for i := 1; i <= 10; i++ {
			occurrence := fmt.Sprintf("occ-fatigue-%d", i)
			ok, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: instance.ID, OccurrenceID: occurrence, Now: now, ManualCheck: true, MaxConcurrent: 1})
			if err != nil || !ok {
				t.Fatalf("admit=%v %v", ok, err)
			}
			req := heartbeat.FinishRequest{InstanceID: instance.ID, OccurrenceID: occurrence, Source: "instance_check", Result: "failed", Reason: "triage_failed", Now: now, ExpiresAt: now.Add(time.Hour), FatigueLimit: 10, Report: &agentinstance.ReportInput{Kind: "failure", Payload: []byte(`{"reason":"triage_failed","detail":"Failed"}`)}}
			if ok, err := repo.Finish(t.Context(), req); err != nil || !ok {
				t.Fatalf("finish=%v %v", ok, err)
			}
			if ok, err := repo.Finish(t.Context(), req); err != nil || ok {
				t.Fatalf("retry=%v %v", ok, err)
			}
			finished, err := repo.Occurrence(t.Context(), instance.ID, occurrence)
			if err != nil || finished.State != "finished" || finished.Result != "failed" || finished.ReportID == "" {
				t.Fatalf("occurrence=%+v err=%v", finished, err)
			}
			if _, err := repo.Occurrence(t.Context(), "other-instance", occurrence); !errors.Is(err, agentinstance.ErrNotFound) {
				t.Fatalf("cross-instance read=%v", err)
			}
			current, err := repo.Get(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if i < 10 && current.State != "active" {
				t.Fatal("paused before limit")
			}
			if i == 10 && (current.State != "paused" || current.PauseReason != "fatigue") {
				t.Fatalf("not paused for fatigue: %+v", current)
			}
		}
		reports, err := db.InstanceReports().ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(reports) != 10 {
			t.Fatalf("reports=%d", len(reports))
		}
		fatigueReports := 0
		for _, report := range reports {
			if strings.Contains(string(report.Payload), `"reason":"fatigue"`) {
				fatigueReports++
			}
		}
		if fatigueReports != 1 {
			t.Fatalf("fatigue reports=%d", fatigueReports)
		}
	})
}

func TestInstanceChecksToggle(t *testing.T) {
	t.Run("Should schedule one cadence ahead only when enabling checks", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "checks-toggle", "sess-toggle-main")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		repo.now = func() time.Time { return now }
		off, err := repo.SetChecks(t.Context(), instance.ID, false, time.Hour)
		if err != nil || off.ChecksEnabled {
			t.Fatalf("off=%+v err=%v", off, err)
		}
		on, err := repo.SetChecks(t.Context(), instance.ID, true, time.Hour)
		if err != nil || !on.ChecksEnabled {
			t.Fatalf("on=%+v err=%v", on, err)
		}
		state, err := repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.NextWakeAt != store.FormatTimestamp(now.Add(time.Hour)) {
			t.Fatalf("next=%s", state.NextWakeAt)
		}
		now = now.Add(time.Minute)
		if _, err := repo.SetChecks(t.Context(), instance.ID, true, time.Hour); err != nil {
			t.Fatal(err)
		}
		repeated, err := repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if repeated.NextWakeAt != state.NextWakeAt {
			t.Fatal("retry postponed checks")
		}
	})
}

func TestInstanceCheckGates(t *testing.T) {
	t.Run("Should reject a paused real check while allowing a read-only dry run", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "paused-check", "sess-paused-check")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		instance, err = repo.Pause(t.Context(), instance.ID, "manual")
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		environment := instanceCheckEnvironment{target: heartbeat.CheckTarget{Instance: instance, Policy: heartbeat.ResolvedPolicy{Enabled: true, GuidanceMarkdown: "Check work"}}}
		dispatcher := &instanceRecoveryDispatcher{}
		service, err := heartbeat.NewCheckService(t.Context(), repo, environment, dispatcher, heartbeat.CheckConfig{Enabled: true, Interval: time.Hour, MaxInterval: 24 * time.Hour, Retention: 24 * time.Hour, Timeout: time.Minute, MaxConcurrent: 2, BatchSize: 10}, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		before, err := repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Check(t.Context(), instance.ID, false); !errors.Is(err, agentinstance.ErrPaused) {
			t.Fatalf("real check error = %v", err)
		}
		result, err := service.Check(t.Context(), instance.ID, true)
		if err != nil || result.Gate != "would_run" || result.Digest == nil {
			t.Fatalf("dry run = %+v, %v", result, err)
		}
		after, err := repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) || dispatcher.calls.Load() != 0 {
			t.Fatalf("check mutated wake state or dispatched: before=%+v after=%+v calls=%d", before, after, dispatcher.calls.Load())
		}
	})

	for _, tc := range []struct {
		name, gate, hook, body, reason string
		failed                         bool
	}{
		{name: "Should skip an empty digest without dispatch", reason: "digest_empty"},
		{name: "Should skip an on demand definition with heartbeat guidance", gate: "definition_not_stays_on", body: "Check work", reason: "definition_not_stays_on"},
		{name: "Should honor hook cancellation for timer and manual check", hook: "denied", body: "Check work", reason: "hook_cancelled"},
		{name: "Should count dispatcher errors as failures", body: "Check work", reason: "triage_failed", failed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "check-gates", "sess-gates")
			repo := db.Instances()
			instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
			environment := instanceCheckEnvironment{target: heartbeat.CheckTarget{Instance: instance, Gate: tc.gate, Policy: heartbeat.ResolvedPolicy{Enabled: true, GuidanceMarkdown: tc.body}}, hook: tc.hook}
			dispatcher := &instanceRecoveryDispatcher{}
			service, err := heartbeat.NewCheckService(t.Context(), repo, environment, dispatcher, heartbeat.CheckConfig{Enabled: true, Interval: time.Hour, MaxInterval: 24 * time.Hour, Retention: 24 * time.Hour, Timeout: time.Minute, MaxConcurrent: 2, BatchSize: 10}, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := service.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			if err := service.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
			if tc.hook != "" {
				result, err := service.Check(t.Context(), instance.ID, false)
				if err != nil || result.Gate != "hook_cancelled" || result.OccurrenceID != "" {
					t.Fatalf("manual=%+v err=%v", result, err)
				}
			}
			if err := service.Tick(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := service.Shutdown(t.Context()); err != nil {
				t.Fatal(err)
			}
			state, err := repo.WakeState(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.failed {
				if dispatcher.calls.Load() != 1 || state.FailureStreak != 1 || state.QuietStreak != 0 || state.LastDigest != "" {
					t.Fatalf("failed state=%+v calls=%d", state, dispatcher.calls.Load())
				}
			} else {
				if dispatcher.calls.Load() != 0 {
					t.Fatal("gate dispatched triage")
				}
				if err := service.Tick(t.Context()); err != nil {
					t.Fatal(err)
				}
				after, err := repo.WakeState(t.Context(), instance.ID)
				if err != nil {
					t.Fatal(err)
				}
				if after.NextWakeAt != state.NextWakeAt {
					t.Fatal("repeated tick advanced the skipped occurrence")
				}
			}
			var count int
			if err := db.db.QueryRowContext(t.Context(), "SELECT count(*) FROM agent_heartbeat_wake_events WHERE instance_id=?", instance.ID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("events=%d", count)
			}
		})
	}
}

func TestInstanceDigestSuppression(t *testing.T) {
	t.Run("Should suppress unchanged digests until max interval and preserve them across empty skips", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "digest-repeat", "sess-digest-repeat")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		ok, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: instance.ID, OccurrenceID: "seed", Now: start, ManualCheck: true, MaxConcurrent: 1})
		if err != nil || !ok {
			t.Fatalf("admit=%v %v", ok, err)
		}
		if _, err := repo.Finish(t.Context(), heartbeat.FinishRequest{InstanceID: instance.ID, OccurrenceID: "seed", Source: "instance_check", Result: "quiet", Reason: "quiet", Digest: "A", Now: start, ExpiresAt: start.Add(48 * time.Hour)}); err != nil {
			t.Fatal(err)
		}
		now := start.Add(time.Hour)
		environment := &instanceCheckEnvironment{target: heartbeat.CheckTarget{Instance: instance, Policy: heartbeat.ResolvedPolicy{Enabled: true}}, digest: heartbeat.CheckDigest{Fingerprint: "A", Items: []heartbeat.DigestItem{{Key: "task:blocked"}}}}
		dispatcher := &instanceRecoveryDispatcher{}
		service, err := heartbeat.NewCheckService(t.Context(), repo, environment, dispatcher, heartbeat.CheckConfig{Enabled: true, Interval: time.Hour, MaxInterval: 24 * time.Hour, Retention: 48 * time.Hour, Timeout: time.Minute, MaxConcurrent: 1, BatchSize: 10}, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		for i, reason := range []string{"digest_unchanged", "digest_empty", "digest_unchanged"} {
			now = start.Add(time.Duration(1+i*2) * time.Hour)
			environment.digest.Items = []heartbeat.DigestItem{{Key: "task:blocked"}}
			if i == 1 {
				environment.digest.Items = nil
			}
			if err := service.Tick(t.Context()); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := db.db.QueryRowContext(t.Context(), "SELECT count(*) FROM agent_heartbeat_wake_events WHERE instance_id=? AND reason=?", instance.ID, reason).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count == 0 || dispatcher.calls.Load() != 0 {
				t.Fatalf("reason=%s events=%d calls=%d", reason, count, dispatcher.calls.Load())
			}
		}
		now = start.Add(24 * time.Hour)
		if err := service.Tick(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := service.Shutdown(t.Context()); err != nil {
			t.Fatal(err)
		}
		if dispatcher.calls.Load() != 1 {
			t.Fatalf("expired digest calls=%d", dispatcher.calls.Load())
		}
	})
}

func TestInstanceMissedDueTimes(t *testing.T) {
	t.Run("Should run once after multiple missed cadences without a live main session", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "missed-due", "sess-missed-due")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		environment := instanceCheckEnvironment{target: heartbeat.CheckTarget{Instance: instance, Policy: heartbeat.ResolvedPolicy{Enabled: true, GuidanceMarkdown: "Review work"}}}
		dispatcher := &instanceRecoveryDispatcher{}
		service, err := heartbeat.NewCheckService(t.Context(), repo, environment, dispatcher, heartbeat.CheckConfig{Enabled: true, Interval: time.Hour, MaxInterval: 24 * time.Hour, Retention: 24 * time.Hour, Timeout: time.Minute, MaxConcurrent: 1, BatchSize: 10}, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		for range 3 {
			if err := service.Tick(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		if err := service.Shutdown(t.Context()); err != nil {
			t.Fatal(err)
		}
		if dispatcher.calls.Load() != 1 {
			t.Fatalf("dispatches=%d", dispatcher.calls.Load())
		}
		state, err := repo.WakeState(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		next, err := store.ParseTimestamp(state.NextWakeAt)
		if err != nil {
			t.Fatal(err)
		}
		if !next.After(now) {
			t.Fatalf("next=%s", state.NextWakeAt)
		}
	})
}

type blockingInstanceDispatcher struct {
	release chan struct{}
	calls   atomic.Int32
}

var _ heartbeat.TriageDispatcher = (*blockingInstanceDispatcher)(nil)

func (d *blockingInstanceDispatcher) Dispatch(ctx context.Context, _ heartbeat.CheckTarget, _ heartbeat.CheckDigest, _ func(string) error) (heartbeat.TriageOutcome, error) {
	d.calls.Add(1)
	select {
	case <-d.release:
		return heartbeat.TriageOutcome{Result: "quiet", Reason: "quiet"}, nil
	case <-ctx.Done():
		return heartbeat.TriageOutcome{}, ctx.Err()
	}
}
func (*blockingInstanceDispatcher) Stop(context.Context, string) error { return nil }

func TestInstanceManualCheckRace(t *testing.T) {
	for _, future := range []bool{false, true} {
		name := "Should admit one check when timer and manual check race"
		if future {
			name = "Should run a manual check before due without moving cadence"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "manual-race", "sess-manual-race")
			repo := db.Instances()
			instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
			repo.now = func() time.Time { return now }
			if future {
				if _, err := repo.SetChecks(t.Context(), instance.ID, false, time.Hour); err != nil {
					t.Fatal(err)
				}
				if _, err := repo.SetChecks(t.Context(), instance.ID, true, time.Hour); err != nil {
					t.Fatal(err)
				}
			}
			before, err := repo.WakeState(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			environment := instanceCheckEnvironment{target: heartbeat.CheckTarget{Instance: instance, Policy: heartbeat.ResolvedPolicy{Enabled: true, GuidanceMarkdown: "Check work"}}}
			dispatcher := &blockingInstanceDispatcher{release: make(chan struct{})}
			service, err := heartbeat.NewCheckService(t.Context(), repo, environment, dispatcher, heartbeat.CheckConfig{Enabled: true, Interval: time.Hour, MaxInterval: 24 * time.Hour, Retention: 24 * time.Hour, Timeout: time.Minute, MaxConcurrent: 1, BatchSize: 10}, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := service.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			if err := service.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			wg.Go(func() {
				if _, err := service.Check(t.Context(), instance.ID, false); err != nil {
					t.Error(err)
				}
			})
			wg.Go(func() {
				if err := service.Tick(t.Context()); err != nil {
					t.Error(err)
				}
			})
			wg.Wait()
			close(dispatcher.release)
			if err := service.Shutdown(t.Context()); err != nil {
				t.Fatal(err)
			}
			if dispatcher.calls.Load() != 1 {
				t.Fatalf("dispatches=%d", dispatcher.calls.Load())
			}
			after, err := repo.WakeState(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if future && before.NextWakeAt != after.NextWakeAt {
				t.Fatalf("cadence moved: %s -> %s", before.NextWakeAt, after.NextWakeAt)
			}
		})
	}
}

func TestInstanceCheckCapacity(t *testing.T) {
	t.Run("Should admit two of five due instances and leave three due", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		repo := db.Instances()
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		for i := range 5 {
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, fmt.Sprintf("capacity-%d", i), fmt.Sprintf("sess-capacity-%d", i))
			if _, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10); err != nil {
				t.Fatal(err)
			}
		}
		due, err := repo.Due(t.Context(), now, 10)
		if err != nil || len(due) != 5 {
			t.Fatalf("due=%d err=%v", len(due), err)
		}
		admitted := 0
		for i, state := range due {
			ok, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: state.InstanceID, OccurrenceID: fmt.Sprintf("capacity-%d", i), Now: now, NextWakeAt: now.Add(time.Hour), MaxConcurrent: 2})
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				admitted++
			}
		}
		remaining, err := repo.Due(t.Context(), now, 10)
		if err != nil || admitted != 2 || len(remaining) != 3 {
			t.Fatalf("admitted=%d remaining=%d err=%v", admitted, len(remaining), err)
		}
		for _, state := range remaining {
			if state.NextWakeAt != "" || state.OccurrenceID != "" {
				t.Fatalf("rejected changed=%+v", state)
			}
		}
	})
}

func TestInstanceProfileDedupe(t *testing.T) {
	t.Run("Should keep proposal suppression separate for the same agent and scope in two profiles", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		repo := db.Instances()
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		if _, err := db.db.ExecContext(t.Context(), `INSERT INTO profiles (id,name,color,icon,state,created_at) VALUES ('00000000000000000000000001','two','#E8572A','circle','active',?)`, store.FormatTimestamp(now)); err != nil {
			t.Fatal(err)
		}
		for i, profile := range []string{store.DefaultProfileID, "00000000000000000000000001"} {
			instance, _, err := repo.AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: profile, AgentName: "coder", PermissionCeiling: "{}", CreatedByKind: "human"}, 10)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.db.ExecContext(t.Context(), "UPDATE agent_instances SET state='active' WHERE id=?", instance.ID); err != nil {
				t.Fatal(err)
			}
			occurrence := fmt.Sprintf("profile-occ-%d", i)
			if ok, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: instance.ID, OccurrenceID: occurrence, Now: now, ManualCheck: true, MaxConcurrent: 2}); err != nil || !ok {
				t.Fatalf("admit=%v %v", ok, err)
			}
			if _, err := repo.Finish(t.Context(), heartbeat.FinishRequest{InstanceID: instance.ID, OccurrenceID: occurrence, Source: "instance_check", Result: "reported", Reason: "reported", Now: now, ExpiresAt: now.Add(time.Hour), Report: &agentinstance.ReportInput{Kind: "proposal", Payload: []byte(`{"proposals":[{"title":"Review","body":"Review work","item_keys":["task:same"]}]}`)}}); err != nil {
				t.Fatal(err)
			}
			result, err := repo.Occurrence(t.Context(), instance.ID, occurrence)
			if err != nil || result.Result != "reported" || result.ReportID == "" {
				t.Fatalf("profile %s result=%+v err=%v", profile, result, err)
			}
		}
	})
}

func TestInstanceFinishOutcomes(t *testing.T) {
	for _, tc := range []struct{ name, result, reason string }{
		{"Should persist quiet", "quiet", "quiet"},
		{"Should persist reported", "reported", "reported"},
		{"Should persist cancellation", "cancelled", "cancelled"},
		{"Should persist duplicate suppression", "quiet", "duplicate_suppressed"},
		{"Should persist unsupported triage", "failed", "triage_unsupported"},
		{"Should persist invalid output", "failed", "triage_output_invalid"},
		{"Should persist timeout", "failed", "triage_timeout"},
		{"Should persist interruption", "failed", "triage_interrupted"},
		{"Should persist dispatch failure", "failed", "triage_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "finish-outcomes", "sess-finish-main")
			repo := db.Instances()
			instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
			if admitted, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: instance.ID, OccurrenceID: "outcome", Now: now, NextWakeAt: now.Add(time.Hour), MaxConcurrent: 2}); err != nil || !admitted {
				t.Fatalf("admission=%v err=%v", admitted, err)
			}
			if changed, err := repo.Finish(t.Context(), heartbeat.FinishRequest{InstanceID: instance.ID, OccurrenceID: "outcome", Source: "timer", Result: tc.result, Reason: tc.reason, Now: now, ExpiresAt: now.Add(24 * time.Hour)}); err != nil || !changed {
				t.Fatalf("finish=%v err=%v", changed, err)
			}
			var result, reason string
			if err := db.DB().QueryRowContext(t.Context(), "SELECT result,reason FROM agent_heartbeat_wake_events WHERE instance_id=? AND occurrence_id='outcome'", instance.ID).Scan(&result, &reason); err != nil {
				t.Fatal(err)
			}
			if result != tc.result || reason != tc.reason {
				t.Fatalf("persisted=%s/%s want=%s/%s", result, reason, tc.result, tc.reason)
			}
			state, err := repo.WakeState(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if state.OccurrenceID != "" {
				t.Fatal("finished occurrence retained its slot")
			}
		})
	}
}

func TestInstanceCollectorCompletion(t *testing.T) {
	t.Run("Should preserve digest across reports and consume automation pages only on completion", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "collector-completion", "sess-collector-completion")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= 11; i++ {
			if _, err := db.DB().ExecContext(t.Context(), `INSERT INTO automation_runs (id,profile_id,workspace_id,status,ended_at) VALUES (?,?,?,'failed','2026-11-01T00:00:00Z')`, fmt.Sprintf("auto-%02d", i), store.DefaultProfileID, workspace); err != nil {
				t.Fatal(err)
			}
		}
		collect := func() heartbeat.CheckDigest {
			t.Helper()
			state, err := repo.WakeState(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			scope, facts, err := repo.CollectorFacts(t.Context(), instance.ID, heartbeat.CollectorScope{AutomationCursor: state.AutomationCursor})
			if err != nil {
				t.Fatal(err)
			}
			digest, err := heartbeat.CollectDigest(scope, facts, []string{"automations_failed"}, "policy", "")
			if err != nil {
				t.Fatal(err)
			}
			return digest
		}
		first := collect()
		if len(first.Items) != 10 {
			t.Fatalf("first page=%+v", first)
		}
		if _, err := db.InstanceReports().AppendReport(t.Context(), instance.ID, agentinstance.ReportInput{SourceKey: "turn:report", Kind: "failure", Payload: []byte(`{"reason":"failed","detail":"test"}`)}); err != nil {
			t.Fatal(err)
		}
		if after := collect(); after.Fingerprint != first.Fingerprint {
			t.Fatal("report changed digest")
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		for i, result := range []string{"failed", "quiet", "quiet"} {
			digest := collect()
			want := 10
			if i == 2 {
				want = 1
			}
			if len(digest.Items) != want {
				t.Fatalf("step %d items=%+v", i, digest.Items)
			}
			occurrence := fmt.Sprintf("collector-%d", i)
			if ok, err := repo.Admit(t.Context(), heartbeat.AdmitRequest{InstanceID: instance.ID, OccurrenceID: occurrence, Now: now, ManualCheck: true, MaxConcurrent: 1}); err != nil || !ok {
				t.Fatalf("admit=%v err=%v", ok, err)
			}
			reason := result
			if result == "failed" {
				reason = "triage_failed"
			}
			if changed, err := repo.Finish(t.Context(), heartbeat.FinishRequest{InstanceID: instance.ID, OccurrenceID: occurrence, Source: "instance_check", Result: result, Reason: reason, Digest: digest.Fingerprint, AutomationCursor: digest.AutomationCursor, Now: now, ExpiresAt: now.Add(24 * time.Hour)}); err != nil || !changed {
				t.Fatalf("finish=%v err=%v", changed, err)
			}
			if i == 0 && collect().Fingerprint != first.Fingerprint {
				t.Fatal("failed check consumed cursor")
			}
		}
		if final := collect(); len(final.Items) != 0 {
			t.Fatalf("unconsumed failures=%+v", final.Items)
		}
	})
}

func TestInstanceDueFiltering(t *testing.T) {
	t.Run("Should return only active instances whose wake time has arrived", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		repo := db.Instances()
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		var wanted string
		for _, name := range []string{"due", "future", "paused"} {
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "due-"+name, "sess-due-"+name)
			instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			at := now.Add(-time.Minute)
			if name == "future" {
				at = now.Add(time.Minute)
			}
			if _, err := db.DB().ExecContext(t.Context(), "UPDATE agent_instance_wake_state SET next_wake_at=? WHERE instance_id=?", store.FormatTimestamp(at), instance.ID); err != nil {
				t.Fatal(err)
			}
			if name == "paused" {
				if _, err := repo.Pause(t.Context(), instance.ID, "manual"); err != nil {
					t.Fatal(err)
				}
			}
			if name == "due" {
				wanted = instance.ID
			}
		}
		due, err := repo.Due(t.Context(), now, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(due) != 1 || due[0].InstanceID != wanted {
			t.Fatalf("due=%+v want=%s", due, wanted)
		}
	})
}

func TestInstanceCollectorSecrets(t *testing.T) {
	t.Run("Should omit task claim tokens from facts and digest", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "collector-secrets", "sess-collector-secrets")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().ExecContext(t.Context(), `INSERT INTO tasks (id,profile_id,scope,workspace_id,title,status,owner_kind,owner_ref,created_by_kind,created_by_ref,origin_kind,origin_ref,created_at,updated_at) VALUES ('secret-task',?,'workspace',?,'Blocked task','blocked','agent_session',?,'human','test','cli','test','2026-11-01','2026-11-01')`, store.DefaultProfileID, workspace, main); err != nil {
			t.Fatal(err)
		}
		const secret = "private-task-claim-token"
		if _, err := db.DB().ExecContext(t.Context(), `INSERT INTO task_runs (id,task_id,workspace_id,status,attempt,origin_kind,origin_ref,queued_at,claim_token) VALUES ('secret-run','secret-task',?,'queued',1,'cli','test','2026-11-01',?)`, workspace, secret); err != nil {
			t.Fatal(err)
		}
		scope, facts, err := repo.CollectorFacts(t.Context(), instance.ID, heartbeat.CollectorScope{})
		if err != nil {
			t.Fatal(err)
		}
		digest, err := heartbeat.CollectDigest(scope, facts, []string{"task"}, "policy", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(digest.Items) != 2 {
			t.Fatalf("missing task/run facts: %+v", digest)
		}
		for _, value := range []any{facts, digest} {
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "claim_token") {
				t.Fatal("collector exposed claim token")
			}
		}
	})
}

func TestInstanceActivity(t *testing.T) {
	t.Run("Should isolate activity by profile workspace agent and time window", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		since := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
		foreign := "uuuuuuuuuuuuuuuuuuuuuuuuuu"
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO profiles (id,name,color,icon,state,created_at) VALUES (?, 'activity-foreign', '#8E8EB5', 'circle', 'active', ?)`, foreign, store.FormatTimestamp(since)); err != nil {
			t.Fatal(err)
		}
		for index, row := range []struct {
			profile, workspace, agent, day string
			tokens                         int64
		}{
			{store.DefaultProfileID, "", "coder", "2026-10-02", 10},
			{store.DefaultProfileID, "ws-other", "coder", "2026-10-02", 20},
			{foreign, "", "coder", "2026-10-02", 40},
			{store.DefaultProfileID, "", "other", "2026-10-02", 80},
			{store.DefaultProfileID, "", "coder", "2026-09-01", 160},
		} {
			if err := db.UpsertTokenUsageDaily(ctx, store.TokenUsageDailyUpdate{ProfileID: row.profile, WorkspaceID: row.workspace, AgentName: row.agent, Day: row.day, TotalTokens: &row.tokens, CostStatus: store.TokenCostStatusIncluded, CostSource: store.TokenCostSourceNone}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().ExecContext(ctx, `INSERT INTO event_summaries(id,profile_id,workspace_id,agent_name,type,timestamp) VALUES (?,?,?,?,?,?)`, fmt.Sprintf("activity-%d", index), row.profile, row.workspace, row.agent, "session.done", row.day+"T12:00:00Z"); err != nil {
				t.Fatal(err)
			}
		}
		got, err := db.Instances().ReadActivity(ctx, agentinstance.ActivityQuery{ProfileID: store.DefaultProfileID, AgentName: "coder", Since: since})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.CostByProject) != 1 || got.CostByProject[0].Tokens != 10 || got.CostByProject[0].WorkspaceID != "" {
			t.Fatalf("cost = %+v", got.CostByProject)
		}
		if len(got.BusyByHour) != 1 || got.BusyByHour[0].Events != 1 {
			t.Fatalf("hours = %+v", got.BusyByHour)
		}
		if got.ReportsByDay == nil {
			t.Fatal("empty reports must serialize as an array")
		}
	})
}

func TestInstanceSeen(t *testing.T) {
	t.Run("Should preserve monotonic operator-specific seen times across reopen", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), "seen.db")
		db, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", PermissionCeiling: "{}", CreatedByKind: "human"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		before, err := db.Instances().LastSeen(ctx, instance.ID, "first")
		if err != nil || before != nil {
			t.Fatalf("initial seen = %v, %v", before, err)
		}
		for _, next := range []time.Time{at, at.Add(time.Second), at.Add(-time.Hour)} {
			if _, err := db.Instances().MarkSeen(ctx, instance.ID, "first", next); err != nil {
				t.Fatal(err)
			}
		}
		other, err := db.Instances().LastSeen(ctx, instance.ID, "second")
		if err != nil || other != nil {
			t.Fatalf("other operator seen = %v, %v", other, err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
		db, err = OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := db.Instances().LastSeen(ctx, instance.ID, "first")
		if err != nil || got == nil || !got.Equal(at.Add(time.Second)) {
			t.Fatalf("reopened seen = %v, %v", got, err)
		}
	})
}

func TestInstanceCheckIns(t *testing.T) {
	t.Run("Should page real wake events in chronological order within one instance", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "check-ins", "sess-check-ins")
		repo := db.Instances()
		instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		for index, id := range []string{"z-old", "a-new"} {
			at := time.Date(2026, 10, 1, index, 0, 0, 0, time.UTC)
			_, err := db.DB().ExecContext(t.Context(), `INSERT INTO agent_heartbeat_wake_events (id,workspace_id,agent_name,instance_id,source,result,reason,created_at,expires_at) VALUES (?,?,?,?,?,?,?,?,?)`, id, workspace, "coder", instance.ID, "timer", "skipped", "quiet", store.FormatTimestamp(at), store.FormatTimestamp(at.Add(time.Hour)))
			if err != nil {
				t.Fatal(err)
			}
		}
		rows, err := repo.ListCheckIns(t.Context(), instance.ID, "", 1)
		if err != nil || len(rows) != 1 || rows[0].ID != "a-new" || rows[0].Reason != "quiet" {
			t.Fatalf("first page = %#v, %v", rows, err)
		}
		rows, err = repo.ListCheckIns(t.Context(), instance.ID, rows[0].ID, 1)
		if err != nil || len(rows) != 1 || rows[0].ID != "z-old" {
			t.Fatalf("second page = %#v, %v", rows, err)
		}
		rows, err = repo.ListCheckIns(t.Context(), "other-instance", "", 10)
		if err != nil || len(rows) != 0 {
			t.Fatalf("foreign events = %#v, %v", rows, err)
		}
	})
}

func TestInstanceSupersededAdmission(t *testing.T) {
	t.Run("Should fence new prompts by the current pointer while preserving prior receipts", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "rotate-admit", "sess-rotate-old")
		_, successor := registerHeartbeatWorkspaceAndSession(t, db, "rotate-successor", "sess-rotate-new")
		instance, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		req := promptAdmissionRequest(workspace, main, "before-rotation", now)
		first, created, err := db.ClaimSessionPromptAdmission(t.Context(), req)
		if err != nil || !created {
			t.Fatalf("initial admission = %#v, %v", first, err)
		}
		if _, err := db.DB().ExecContext(t.Context(), "UPDATE agent_instances SET main_session_id=? WHERE id=?", successor, instance.ID); err != nil {
			t.Fatal(err)
		}
		replay, created, err := db.ClaimSessionPromptAdmission(t.Context(), req)
		if err != nil || created || replay.ID != first.ID {
			t.Fatalf("replay = %#v, %v", replay, err)
		}
		_, created, err = db.ClaimSessionPromptAdmission(t.Context(), promptAdmissionRequest(workspace, main, "after-rotation", now.Add(time.Second)))
		if !errors.Is(err, store.ErrSessionSuperseded) || created {
			t.Fatalf("superseded admission = %v, %v", created, err)
		}
		if _, err := db.DB().ExecContext(t.Context(), "UPDATE agent_instances SET main_session_id=? WHERE id=?", main, instance.ID); err != nil {
			t.Fatal(err)
		}
		_, created, err = db.ClaimSessionPromptAdmission(t.Context(), promptAdmissionRequest(workspace, main, "current-again", now.Add(2*time.Second)))
		if err != nil || !created {
			t.Fatalf("current pointer admission = %v, %v", created, err)
		}
	})
}

func TestInstanceRotationPublication(t *testing.T) {
	for _, scenario := range []string{"publish", "stale pointer", "new admission", "busy predecessor", "busy successor", "pending interaction", "live descendant", "pending child delivery", "open child wake", "armed reply watch", "fired reply watch"} {
		t.Run("Should fence rotation publication for "+scenario, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "rotation-old", "sess-rotation-old")
			_, next := registerHeartbeatWorkspaceAndSession(t, db, "rotation-new", "sess-rotation-new")
			if _, err := db.DB().ExecContext(t.Context(), "UPDATE sessions SET workspace_id=? WHERE id=?", workspace, next); err != nil {
				t.Fatal(err)
			}
			repo := db.Instances()
			instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			count, err := repo.RotationAdmissionCount(t.Context(), main)
			if err != nil {
				t.Fatal(err)
			}
			req := agentinstance.RotationPublication{InstanceID: instance.ID, PredecessorID: main, SuccessorID: next, AdmissionCount: count}
			switch scenario {
			case "stale pointer":
				_, err = db.DB().ExecContext(t.Context(), "UPDATE agent_instances SET main_session_id=? WHERE id=?", next, instance.ID)
			case "new admission":
				_, _, err = db.ClaimSessionPromptAdmission(t.Context(), promptAdmissionRequest(workspace, main, "rotation-race", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)))
			case "busy predecessor", "busy successor":
				id := main
				if scenario == "busy successor" {
					id = next
				}
				_, err = db.DB().ExecContext(t.Context(), `UPDATE sessions SET activity_json='{"turn_id":"busy"}' WHERE id=?`, id)
			case "pending interaction":
				_, err = db.DB().ExecContext(t.Context(), `INSERT INTO session_pending_interactions (interaction_id,session_id,kind,provider_request_id,status,created_at) VALUES ('interaction',?,'permission','req','pending','2026-11-01T00:00:00Z')`, main)

			case "armed reply watch", "fired reply watch":
				state := "armed"
				if scenario == "fired reply watch" {
					state = "fired"
				}
				_, err = db.DB().ExecContext(t.Context(), `INSERT INTO session_prompt_reply_watches
                    (id,workspace_id,sender_session_id,target_workspace_id,target_session_id,message_id,admission_id,hop,state,outcome,created_at)
                    VALUES ('rotation-watch',?,?,?,?,'msg-watch','adm-watch',1,?,'unknown','2026-11-01T00:00:00Z')`, workspace, main, workspace, next, state)
			case "pending child delivery":
				now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
				row := reserveSubagent(t, db.SessionRepo, workspace, main, "late-result", now)
				finalizeSubagent(t, db.SessionRepo, row.ID, now)
			case "open child wake":
				_, err = db.DB().ExecContext(t.Context(), `INSERT INTO session_subagent_wakes
                    (wake_message_id,workspace_id,parent_session_id,state,route,created_at,updated_at)
                    VALUES ('late-wake',?,?,'open','queue','2026-11-01T00:00:00Z','2026-11-01T00:00:00Z')`, workspace, main)
			case "live descendant":
				_, child := registerHeartbeatWorkspaceAndSession(t, db, "rotation-child", "sess-rotation-child")
				_, err = db.DB().ExecContext(t.Context(), "UPDATE sessions SET parent_session_id=? WHERE id=?", main, child)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = repo.PublishRotation(t.Context(), req)
			if scenario == "publish" && err != nil {
				t.Fatal(err)
			}
			if scenario != "publish" && !errors.Is(err, agentinstance.ErrRotationChanged) {
				t.Fatalf("publication = %v", err)
			}
			got, err := repo.Get(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := main
			if scenario == "publish" || scenario == "stale pointer" {
				want = next
			}
			if got.MainSessionID != want {
				t.Fatalf("pointer = %s, want %s", got.MainSessionID, want)
			}
			if scenario != "publish" {
				var stamped sql.NullString
				if err := db.DB().QueryRowContext(t.Context(), "SELECT instance_id FROM sessions WHERE id=?", next).Scan(&stamped); err != nil {
					t.Fatal(err)
				}
				if stamped.Valid {
					t.Fatalf("failed publication left successor stamped: %s", stamped.String)
				}
			}
		})
	}
}

func TestInstanceRotationCandidate(t *testing.T) {
	t.Run("Should reserve only one live successor without changing the main pointer", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		repo := db.Instances()
		instance, _, err := repo.AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", PermissionCeiling: "{}", CreatedByKind: "human"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		info := store.SessionInfo{ID: "main", ProfileID: store.DefaultProfileID, AgentName: "coder", State: "active", RuntimeStatus: store.SessionRuntimeUnbound, SessionType: "user"}
		if _, err := db.RegisterInstanceSession(t.Context(), info, instance.ID, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.BindMain(t.Context(), instance.ID, info.ID); err != nil {
			t.Fatal(err)
		}
		info.ID = "candidate"
		info.State = "starting"
		if _, err := db.RegisterInstanceSession(t.Context(), info, instance.ID, nil); err != nil {
			t.Fatal(err)
		}
		got, err := repo.Get(t.Context(), instance.ID)
		if err != nil || got.MainSessionID != "main" {
			t.Fatalf("main pointer = %#v, %v", got, err)
		}
		info.ID = "another-candidate"
		if _, err := db.RegisterInstanceSession(t.Context(), info, instance.ID, nil); !errors.Is(err, agentinstance.ErrStarting) {
			t.Fatalf("concurrent candidate = %v", err)
		}
		pending, err := repo.SupersededSessions(t.Context(), instance.ID)
		if err != nil || len(pending) != 1 || pending[0] != "candidate" {
			t.Fatalf("cleanup = %v, %v", pending, err)
		}
		if _, err := db.DB().ExecContext(t.Context(), "UPDATE sessions SET state='stopped' WHERE id='candidate'"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.RegisterInstanceSession(t.Context(), info, instance.ID, nil); err != nil {
			t.Fatalf("retry candidate = %v", err)
		}
	})
}

func TestInstanceResumeAuthority(t *testing.T) {
	for _, reason := range []string{"manual", "fatigue"} {
		t.Run("Should retain an intervening "+reason+" pause during agent resume", func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "resume-authority", "sess-resume-authority")
			repo := db.Instances()
			instance, err := repo.Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			service, err := agentinstance.NewService(
				agentinstance.WithStore(repo),
				agentinstance.WithAdmission(instanceServiceAdmission{validateResume: func(ctx context.Context, before agentinstance.Instance) error {
					if before.State != "active" {
						t.Fatalf("initial state: %+v", before)
					}
					_, err := repo.Pause(ctx, before.ID, reason)
					return err
				}}),
				agentinstance.WithMainSessions(&instanceServiceStarter{}),
				agentinstance.WithActiveLimit(10),
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Resume(t.Context(), instance.ID, workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: main}); !errors.Is(err, agentinstance.ErrForbidden) {
				t.Fatalf("agent resume: %v", err)
			}
			current, err := repo.Get(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if current.State != "paused" || current.PauseReason != reason {
				t.Fatalf("pause overwritten: %+v", current)
			}
			resumed, err := repo.Resume(t.Context(), instance.ID, 10, true)
			if err != nil || resumed.State != "active" || resumed.PauseReason != "" {
				t.Fatalf("operator resume: %+v, %v", resumed, err)
			}
		})
	}
}

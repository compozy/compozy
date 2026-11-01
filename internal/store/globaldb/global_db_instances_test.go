package globaldb

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
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
	input     agentinstance.Instance
	resumeErr error
}

var _ agentinstance.Admission = instanceServiceAdmission{}

func (a instanceServiceAdmission) ResolveCreate(
	context.Context,
	agentinstance.CreateRequest,
) (agentinstance.Instance, error) {
	return a.input, nil
}
func (a instanceServiceAdmission) ValidateResume(context.Context, agentinstance.Instance) error {
	return a.resumeErr
}

func (a instanceServiceAdmission) ResolveAdopt(
	context.Context,
	agentinstance.AdoptRequest,
) (agentinstance.Instance, error) {
	return a.input, nil
}

type instanceServiceStarter struct {
	start   func(context.Context, agentinstance.StartMainSession) (string, error)
	outcome agentinstance.StartOutcome
	discard func(context.Context, agentinstance.Instance, string) error
}

var _ agentinstance.MainSessionStarter = (*instanceServiceStarter)(nil)
var _ agentinstance.ReconciliationSessions = (*instanceServiceStarter)(nil)

func (s *instanceServiceStarter) Start(ctx context.Context, request agentinstance.StartMainSession) (string, error) {
	return s.start(ctx, request)
}
func (*instanceServiceStarter) Stop(context.Context, string) error { return nil }

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
		if _, err := service.Resume(t.Context(), instance.ID); !errors.Is(err, agentinstance.ErrDefinitionMissing) {
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
		if _, err := service.Resume(t.Context(), instance.ID); err != nil {
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
		if recorder.payloads[1].PauseReason != "manual" || recorder.payloads[1].State != "paused" {
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

//go:build integration

package globaldb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const callIntegrationRaceIterations = 50

func TestGlobalDBCallSettlementIsAtomicAndRetainsVerifiedOverflow(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back a failed settlement and commit one retry", func(t *testing.T) {
		t.Parallel()

		fixture := newCallSettlementFixture(t)
		record := fixture.create(t, "atomic", callSettlementExpectSchema, nil)
		trigger := fmt.Sprintf(`CREATE TRIGGER inject_call_result_blob_failure BEFORE INSERT ON payload_blobs
			WHEN NEW.ref <> '%s' BEGIN SELECT RAISE(ABORT, 'injected_result_blob_failure'); END`, record.PromptRef)
		if _, err := fixture.database.db.ExecContext(fixture.ctx, trigger); err != nil {
			t.Fatalf("install result blob failure trigger error = %v", err)
		}
		_, err := fixture.service.Return(fixture.ctx, callspkg.ReturnInput{
			Scope: record.OwnerScope(), CallID: record.CallID, Result: json.RawMessage(`{"answer":42}`),
			Actor: callspkg.SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err == nil || !strings.Contains(err.Error(), "injected_result_blob_failure") {
			t.Fatalf("Return(injected failure) error = %v", err)
		}
		stored, err := fixture.database.GetCall(fixture.ctx, fixture.scope(), record.CallID)
		if err != nil {
			t.Fatalf("GetCall(after rollback) error = %v", err)
		}
		var completionRows int
		if err := fixture.database.db.QueryRowContext(fixture.ctx, `SELECT COUNT(*) FROM call_deliveries
			WHERE subject_id = ? AND kind = 'completion'`, record.CallID).Scan(&completionRows); err != nil {
			t.Fatalf("count rolled-back completion rows error = %v", err)
		}
		if stored.State != callspkg.StateRunning || stored.ResultRef != "" || completionRows != 0 {
			t.Fatalf("settlement rollback = call %#v completion rows %d", stored, completionRows)
		}
		if _, err := fixture.database.db.ExecContext(
			fixture.ctx,
			`DROP TRIGGER inject_call_result_blob_failure`,
		); err != nil {
			t.Fatalf("drop result blob failure trigger error = %v", err)
		}
		settled, err := fixture.service.Return(fixture.ctx, callspkg.ReturnInput{
			Scope: record.OwnerScope(), CallID: record.CallID, Result: json.RawMessage(`{"answer":42}`),
			Actor: callspkg.SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(valid) error = %v", err)
		}
		_, secondErr := fixture.service.Return(fixture.ctx, callspkg.ReturnInput{
			Scope: record.OwnerScope(), CallID: record.CallID, Result: json.RawMessage(`{"answer":99}`),
			Actor: callspkg.SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if !callspkg.IsCode(secondErr, callspkg.CodeAlreadySettled) {
			t.Fatalf("Return(second) error = %v, want %s", secondErr, callspkg.CodeAlreadySettled)
		}
		if err := fixture.database.db.QueryRowContext(fixture.ctx, `SELECT COUNT(*) FROM call_deliveries
			WHERE subject_id = ? AND kind = 'completion'`, record.CallID).Scan(&completionRows); err != nil {
			t.Fatalf("count completion rows error = %v", err)
		}
		if settled.Call.ResultRef == "" || completionRows != 1 {
			t.Fatalf("committed settlement = %#v completion rows %d", settled, completionRows)
		}
	})

	t.Run("Should retain verified overflow bytes", func(t *testing.T) {
		t.Parallel()

		fixture := newCallSettlementFixture(t)
		budget := contracts.ByteBudget{MaxBytes: 256 << 10, Overflow: contracts.OverflowStore}
		record := fixture.create(t, "overflow", nil, &budget)
		payload, err := json.Marshal(map[string]string{"payload": strings.Repeat("x", 300<<10)})
		if err != nil {
			t.Fatalf("json.Marshal(overflow payload) error = %v", err)
		}
		settled, err := fixture.service.Return(fixture.ctx, callspkg.ReturnInput{
			Scope: record.OwnerScope(), CallID: record.CallID, Result: payload,
			Actor: callspkg.SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(overflow store) error = %v", err)
		}
		verified, err := fixture.database.loadVerifiedCallPayload(
			fixture.ctx,
			fixture.workspaceID,
			settled.Call.ResultRef,
		)
		if err != nil {
			t.Fatalf("loadVerifiedCallPayload(overflow) error = %v", err)
		}
		if !bytes.Equal(verified, payload) || settled.Call.ResultBytes != len(payload) {
			t.Fatalf("overflow payload bytes = %d/%d record=%#v", len(verified), len(payload), settled.Call)
		}
		projected, err := fixture.database.GetCallPayloads(
			fixture.ctx, fixture.workspaceID, []string{record.PromptRef, settled.Call.ResultRef},
		)
		if err != nil {
			t.Fatalf("GetCallPayloads() error = %v", err)
		}
		if len(projected) != 2 || !bytes.Equal(projected[settled.Call.ResultRef], payload) {
			t.Fatalf("GetCallPayloads() = %#v, want prompt and exact overflow result", projected)
		}
	})

	t.Run("Should extract a contracted result and persist its schema", func(t *testing.T) {
		t.Parallel()

		fixture := newCallSettlementFixture(t)
		record := fixture.create(t, "extracted", callSettlementExpectSchema, nil)
		settled, err := fixture.service.Return(fixture.ctx, callspkg.ReturnInput{
			Scope: record.OwnerScope(), CallID: record.CallID, FinalText: "Done. ```json\n{\"answer\":7}\n```",
			Actor: callspkg.SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(extracted) error = %v", err)
		}
		if settled.Call.Verdict != callspkg.VerdictExtracted {
			t.Fatalf("Return(extracted) = %#v", settled)
		}
		if _, err := fixture.database.loadVerifiedCallPayload(
			fixture.ctx, fixture.workspaceID, settled.Call.ResultRef,
		); err != nil {
			t.Fatalf("loadVerifiedCallPayload(extracted) error = %v", err)
		}
		verdict, err := contracts.NewRegistry(fixture.database).Validate(
			fixture.ctx, settled.Call.ExpectDigest, json.RawMessage(`{"answer":8}`),
		)
		if err != nil {
			t.Fatalf("fresh registry Validate() error = %v", err)
		}
		if !verdict.Valid {
			t.Fatalf("fresh registry Validate() = %#v, want persisted contract", verdict)
		}
	})

	t.Run("Should resolve attention without removing truly resultless history", func(t *testing.T) {
		t.Parallel()

		fixture := newCallSettlementFixture(t)
		record := fixture.create(t, "silent", callSettlementExpectSchema, nil)
		settled, err := fixture.service.Return(fixture.ctx, callspkg.ReturnInput{
			Scope: record.OwnerScope(), CallID: record.CallID, FinalText: "",
			Actor: callspkg.SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return(silent) error = %v", err)
		}
		if settled.Call.State != callspkg.StateCompletedWithoutResult || settled.Call.FinalProsePreview != "" {
			t.Fatalf(
				"Return(silent) = state %s preview bytes %d",
				settled.Call.State,
				len(settled.Call.FinalProsePreview),
			)
		}
		query := callspkg.CallListQuery{CallReadQuery: fixture.readQuery(), Attention: true}
		before, err := fixture.service.List(fixture.ctx, query)
		if err != nil {
			t.Fatalf("List(unresolved attention) error = %v", err)
		}
		if before.Total != 1 || len(before.Items) != 1 || before.Items[0].CallID != settled.Call.CallID {
			t.Fatalf("List(unresolved attention) = %#v, want silent call", before)
		}
		later, err := callspkg.NewService(
			callspkg.WithStore(fixture.database),
			callspkg.WithDirectory(callIntegrationDirectory{database: fixture.database}),
			callspkg.WithConfig(config.DefaultCallsConfig()),
			callspkg.WithClock(func() time.Time { return fixture.now.Add(time.Minute) }),
			callspkg.WithIDGenerator(store.NewID),
		)
		if err != nil {
			t.Fatalf("calls.NewService(attention resolution) error = %v", err)
		}
		if _, err := later.SendMessage(fixture.ctx, callspkg.SendMessageInput{
			ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace,
			WorkspaceID: fixture.workspaceID,
			From:        callspkg.MessageSender{Kind: "operator", ID: "operator:test"},
			To:          settled.Call.ChildSessionID, CallID: settled.Call.CallID,
			Body: "Please return the missing result.",
		}); err != nil {
			t.Fatalf("SendMessage(resolve attention) error = %v", err)
		}
		after, err := fixture.service.List(fixture.ctx, query)
		if err != nil {
			t.Fatalf("List(resolved attention) error = %v", err)
		}
		if after.Total != 0 || len(after.Items) != 0 {
			t.Fatalf("List(resolved attention) = %#v, want no unresolved cause", after)
		}
		history, err := fixture.service.List(fixture.ctx, callspkg.CallListQuery{
			CallReadQuery: query.CallReadQuery, State: []callspkg.State{callspkg.StateCompletedWithoutResult},
		})
		if err != nil {
			t.Fatalf("List(attention history) error = %v", err)
		}
		if history.Total != 1 || len(history.Items) != 1 || history.Items[0].CallID != settled.Call.CallID {
			t.Fatalf("List(attention history) = %#v, want retained silent call", history)
		}
	})
}

var callSettlementExpectSchema = json.RawMessage(
	`{"type":"object","required":["answer"],"properties":{"answer":{"type":"integer"}},"additionalProperties":false}`,
)

type callSettlementFixture struct {
	ctx         context.Context
	database    *GlobalDB
	service     *callspkg.Service
	workspaceID string
	parentID    string
	now         time.Time
}

func newCallSettlementFixture(t *testing.T) callSettlementFixture {
	t.Helper()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		database,
		"calls-settlement",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID := "ses_calls_settlement_parent"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
		}, CreatedAt: now, UpdatedAt: now,
	})
	tasks, err := taskpkg.NewManager(taskpkg.WithStore(database), taskpkg.WithGovernedRootActiveRunCap(32))
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	service, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithActivationClaimer(tasks), callspkg.WithActivationRunCanceler(tasks),
		callspkg.WithSessionInvoker(&callIntegrationInvoker{database: database, workspaceID: workspaceID, now: now}),
		callspkg.WithConfig(config.DefaultCallsConfig()), callspkg.WithClock(func() time.Time { return now }),
		callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}
	return callSettlementFixture{
		ctx: ctx, database: database, service: service, workspaceID: workspaceID, parentID: parentID, now: now,
	}
}

func (f callSettlementFixture) create(
	t *testing.T,
	key string,
	contract json.RawMessage,
	budget *contracts.ByteBudget,
) callspkg.CallRecord {
	t.Helper()
	record, err := f.service.Create(f.ctx, callspkg.CreateInput{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: f.workspaceID,
		Caller: participation.OwnerRef{
			Kind: participation.OwnerKindSession, ID: f.parentID, WorkspaceID: f.workspaceID,
		},
		Target: callspkg.Target{Agent: "reviewer"}, Prompt: "settle " + key, Expect: contract,
		ResultBudget: budget, IdempotencyKey: key, Actor: callspkg.Actor{Kind: "human", ID: "operator:test"},
		Narrow: callspkg.PermissionAtoms{Skills: []string{"review"}},
	})
	if err != nil {
		t.Fatalf("Create(%q) error = %v", key, err)
	}
	if record.State != callspkg.StateQueued || record.ActivationRunID == "" {
		t.Fatalf("Create(%q) = %#v, want queued activation", key, record)
	}
	if dispatched, dispatchErr := f.service.DispatchQueued(f.ctx, 1); dispatchErr != nil || dispatched != 1 {
		t.Fatalf("DispatchQueued(%q) = %d, %v, want one activation", key, dispatched, dispatchErr)
	}
	activated, err := f.database.GetCall(f.ctx, record.OwnerScope(), record.CallID)
	if err != nil {
		t.Fatalf("GetCall(%q, activated) error = %v", key, err)
	}
	if activated.State != callspkg.StateRunning || activated.ChildSessionID == "" {
		t.Fatalf("GetCall(%q, activated) = %#v, want running child", key, activated)
	}
	return activated
}

func (f callSettlementFixture) scope() callspkg.CallScope {
	return callspkg.CallScope{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: f.workspaceID,
	}
}

func (f callSettlementFixture) readQuery() callspkg.CallReadQuery {
	return callspkg.CallReadQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
		Scope:     callspkg.ScopeWorkspace, WorkspaceID: f.workspaceID,
	}
}

func TestGlobalDBCallRuntimeRecoversClaimedActivationAndDurableAwait(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		database,
		"calls-recovery",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID := "ses_calls_recovery_parent"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
		}, CreatedAt: now, UpdatedAt: now,
	})
	tasks, err := taskpkg.NewManager(
		taskpkg.WithStore(database), taskpkg.WithGovernedRootActiveRunCap(32),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	queuedService, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithActivationRunCanceler(tasks), callspkg.WithConfig(config.DefaultCallsConfig()),
		callspkg.WithClock(func() time.Time { return now }), callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService(queued) error = %v", err)
	}
	input := callspkg.CreateInput{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
		Caller: participation.OwnerRef{Kind: participation.OwnerKindSession, ID: parentID, WorkspaceID: workspaceID},
		Target: callspkg.Target{Agent: "reviewer"}, Prompt: "recover me", IdempotencyKey: "recover",
		Actor:  callspkg.Actor{Kind: "human", ID: "operator:test"},
		Narrow: callspkg.PermissionAtoms{Skills: []string{"review"}},
	}
	record, err := queuedService.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	actor := taskpkg.ActorContext{
		Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "calls.activation"},
		Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "calls.activation"},
		Authority: taskpkg.Authority{Read: true, Write: true},
		Scope:     taskpkg.CallerScope{WorkspaceID: workspaceID},
	}
	if _, err := tasks.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID: record.ActivationRunID, RunKind: taskpkg.RunKindCallActivation,
		Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
	}, actor); err != nil {
		t.Fatalf("ClaimNextRun(crash window) error = %v", err)
	}
	invoker := &callIntegrationInvoker{database: database, workspaceID: workspaceID, now: now}
	recoveredService, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithActivationClaimer(tasks), callspkg.WithActivationRunCanceler(tasks),
		callspkg.WithSessionInvoker(invoker), callspkg.WithConfig(config.DefaultCallsConfig()),
		callspkg.WithClock(func() time.Time { return now.Add(time.Minute) }), callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService(recovered) error = %v", err)
	}
	if err := recoveredService.RecoverCallRuntime(ctx); err != nil {
		t.Fatalf("RecoverCallRuntime() error = %v", err)
	}
	recovered, err := database.GetCall(ctx, callspkg.CallScope{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
	}, record.CallID)
	if err != nil {
		t.Fatalf("GetCall(recovered) error = %v", err)
	}
	var runStatus string
	if err := database.db.QueryRowContext(ctx, `SELECT status FROM task_runs WHERE id = ?`, record.ActivationRunID).
		Scan(&runStatus); err != nil {
		t.Fatalf("read recovered activation status error = %v", err)
	}
	if recovered.State != callspkg.StateRunning || recovered.ChildSessionID == "" ||
		runStatus != taskpkg.TaskRunStatusCompleted.String() || invoker.spawnCount() != 1 {
		t.Fatalf("recovered call = %#v run=%s spawns=%d", recovered, runStatus, invoker.spawnCount())
	}
	if _, err := recoveredService.Return(ctx, callspkg.ReturnInput{
		Scope: recovered.OwnerScope(), CallID: recovered.CallID, Result: json.RawMessage(`{"done":true}`),
		Actor: callspkg.SettlementActor{Kind: "agent_session", ID: recovered.ChildSessionID},
	}); err != nil {
		t.Fatalf("Return(recovered) error = %v", err)
	}
	resumedService, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithActivationRunCanceler(tasks), callspkg.WithConfig(config.DefaultCallsConfig()),
		callspkg.WithClock(func() time.Time { return now.Add(2 * time.Minute) }), callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService(resumed await) error = %v", err)
	}
	awaited, err := resumedService.Await(ctx, callspkg.AwaitInput{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
		CallIDs: []string{record.CallID}, Timeout: 0, Resume: "opaque-previous-process-token",
	})
	if err != nil {
		t.Fatalf("Await(restart) error = %v", err)
	}
	if awaited.Outcome != "complete" || len(awaited.Settled) != 1 || len(awaited.Pending) != 0 {
		t.Fatalf("Await(restart) = %#v", awaited)
	}

	deadline := now.Add(3 * time.Minute)
	deadlineInput := input
	deadlineInput.Prompt = "expire while queued"
	deadlineInput.IdempotencyKey = "deadline"
	deadlineInput.Deadline = &deadline
	deadlineRecord, err := queuedService.Create(ctx, deadlineInput)
	if err != nil {
		t.Fatalf("Create(deadline) error = %v", err)
	}
	report, err := queuedService.SweepDeadlines(ctx, deadline.Add(time.Second))
	if err != nil {
		t.Fatalf("SweepDeadlines() error = %v", err)
	}
	if len(report.TimedOut) != 1 || report.TimedOut[0] != deadlineRecord.CallID {
		t.Fatalf("SweepDeadlines() = %#v", report)
	}
	deadlineStored, err := database.GetCall(ctx, callspkg.CallScope{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
	}, deadlineRecord.CallID)
	if err != nil {
		t.Fatalf("GetCall(deadline) error = %v", err)
	}
	if err := database.db.QueryRowContext(ctx, `SELECT status FROM task_runs WHERE id = ?`, deadlineRecord.ActivationRunID).
		Scan(&runStatus); err != nil {
		t.Fatalf("read deadline activation status error = %v", err)
	}
	if deadlineStored.State != callspkg.StateTimeout || runStatus != taskpkg.TaskRunStatusCanceled.String() {
		t.Fatalf("deadline outcome = call %s run %s", deadlineStored.State, runStatus)
	}
}

func TestGlobalDBCallOwnershipIsolationAndCycleSafeDrain(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		database,
		"calls-ownership",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	secondProfileID := strings.Repeat("Q", 26)
	if _, err := database.db.ExecContext(ctx, `INSERT INTO profiles
		(id, name, color, icon, state, created_at) VALUES (?, 'calls-secondary', '#8e8eb5', 'circle', 'active', ?)`,
		secondProfileID, store.FormatTimestamp(now)); err != nil {
		t.Fatalf("insert secondary profile error = %v", err)
	}
	parents := map[string]string{
		store.DefaultProfileID: "ses_calls_owner_default",
		secondProfileID:        "ses_calls_owner_secondary",
	}
	for profileID, parentID := range parents {
		registerCallSession(ctx, t, database, store.SessionInfo{
			ID: parentID, ProfileID: profileID, AgentName: "coordinator",
			WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
			Lineage: &store.SessionLineage{
				RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
				PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
			}, CreatedAt: now, UpdatedAt: now,
		})
	}
	directory := callIntegrationDirectoryFunc(func(
		_ context.Context,
		input callspkg.CreateInput,
	) (callspkg.TargetContext, []callspkg.AgentRosterEntry, error) {
		parentID := parents[input.ProfileID]
		return callspkg.TargetContext{
			ProfileID: input.ProfileID, WorkspaceID: workspaceID, ParentSessionID: parentID,
			AgentName: "reviewer", GovernedRootID: parentID, Depth: 1, Allowed: true,
			CallerPolicy: callIntegrationPermissionPolicy(),
		}, []callspkg.AgentRosterEntry{{Name: "reviewer"}}, nil
	})
	tasks, err := taskpkg.NewManager(
		taskpkg.WithStore(database), taskpkg.WithGovernedRootActiveRunCap(32),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	service, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(directory),
		callspkg.WithActivationRunCanceler(tasks), callspkg.WithConfig(config.DefaultCallsConfig()),
		callspkg.WithClock(func() time.Time { return now }), callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}
	createForProfile := func(profileID string) callspkg.CallRecord {
		t.Helper()
		record, createErr := service.Create(ctx, callspkg.CreateInput{
			ProfileID: profileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
			Caller: participation.OwnerRef{
				Kind: participation.OwnerKindTaskRun, ID: "shared-caller", WorkspaceID: workspaceID,
			},
			Target: callspkg.Target{Agent: "reviewer"}, Prompt: "same bytes", IdempotencyKey: "same-key",
			Actor:  callspkg.Actor{Kind: "human", ID: "operator:test"},
			Narrow: callspkg.PermissionAtoms{Skills: []string{"review"}},
		})
		if createErr != nil {
			t.Fatalf("Create(profile %q) error = %v", profileID, createErr)
		}
		return record
	}
	defaultCall, secondaryCall := createForProfile(store.DefaultProfileID), createForProfile(secondProfileID)
	if defaultCall.CallID == secondaryCall.CallID {
		t.Fatalf("profile-isolated calls share id %q", defaultCall.CallID)
	}
	t.Run("Should isolate profile-owned reads bindings and immutable ownership", func(t *testing.T) {
		if _, err := database.db.ExecContext(ctx, `UPDATE calls
		SET state = 'running', child_session_id = ?, started_at = ?, updated_at = ? WHERE call_id = ?`,
			parents[store.DefaultProfileID], store.FormatTimestamp(now), store.FormatTimestamp(now), defaultCall.CallID,
		); err != nil {
			t.Fatalf("bind default call child for read projection error = %v", err)
		}
		exactPage, err := service.List(ctx, callspkg.CallListQuery{
			CallReadQuery: callspkg.CallReadQuery{
				ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
				Scope:     callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
			},
			ChildSessionID: parents[store.DefaultProfileID], RootSessionID: parents[store.DefaultProfileID],
			Agent: "reviewer", Limit: 1,
		})
		if err != nil {
			t.Fatalf("List(exact received root) error = %v", err)
		}
		if exactPage.Total != 1 || len(exactPage.Items) != 1 || exactPage.Items[0].CallID != defaultCall.CallID {
			t.Fatalf("List(exact received root) = %#v", exactPage)
		}
		aggregatePage, err := service.List(ctx, callspkg.CallListQuery{
			CallReadQuery: callspkg.CallReadQuery{
				ReadScope: store.ReadScope{AllProfiles: true}, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
			},
			Agent: "reviewer", Limit: 1,
		})
		if err != nil {
			t.Fatalf("List(aggregate agent summary) error = %v", err)
		}
		if aggregatePage.Total != 2 || len(aggregatePage.Items) != 1 || aggregatePage.NextCursor == "" {
			t.Fatalf("List(aggregate agent summary) = %#v", aggregatePage)
		}
		for profileID, parentID := range parents {
			binding, bindErr := database.ResolveOperatorCaller(ctx, callspkg.OperatorCallerBinding{
				ProfileID: profileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID, SessionID: parentID,
			})
			if bindErr != nil {
				t.Fatalf("ResolveOperatorCaller(%q) error = %v", profileID, bindErr)
			}
			if !binding.Created || binding.ProfileID != profileID || binding.SessionID != parentID {
				t.Fatalf("ResolveOperatorCaller(%q) = %#v", profileID, binding)
			}
		}
		if _, err := database.db.ExecContext(
			ctx,
			`UPDATE calls SET profile_id = ? WHERE call_id = ?`,
			secondProfileID,
			defaultCall.CallID,
		); err == nil || !strings.Contains(err.Error(), "profile_owner_immutable") {
			t.Fatalf("mutate call profile owner error = %v", err)
		}
	})

	t.Run("Should reject foreign archived and lifecycle-fenced owners without writes", func(t *testing.T) {
		var rowsBefore int
		if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calls`).Scan(&rowsBefore); err != nil {
			t.Fatalf("count calls before denial error = %v", err)
		}
		foreignDirectory := callIntegrationDirectoryFunc(func(
			_ context.Context,
			_ callspkg.CreateInput,
		) (callspkg.TargetContext, []callspkg.AgentRosterEntry, error) {
			return callspkg.TargetContext{
				ProfileID: secondProfileID, WorkspaceID: workspaceID,
				ParentSessionID: parents[secondProfileID], AgentName: "reviewer",
				GovernedRootID: parents[secondProfileID], Depth: 1, Allowed: true,
				CallerPolicy: callIntegrationPermissionPolicy(),
			}, []callspkg.AgentRosterEntry{{Name: "reviewer"}}, nil
		})
		foreignService, err := callspkg.NewService(
			callspkg.WithStore(database), callspkg.WithDirectory(foreignDirectory),
			callspkg.WithActivationRunCanceler(tasks), callspkg.WithConfig(config.DefaultCallsConfig()),
			callspkg.WithClock(func() time.Time { return now }), callspkg.WithIDGenerator(store.NewID),
		)
		if err != nil {
			t.Fatalf("calls.NewService(foreign) error = %v", err)
		}
		foreignInput := callspkg.CreateInput{
			ProfileID:   store.DefaultProfileID,
			Scope:       callspkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Caller: participation.OwnerRef{
				Kind:        participation.OwnerKindTaskRun,
				ID:          "shared-caller",
				WorkspaceID: workspaceID,
			},
			Target:         callspkg.Target{Agent: "reviewer"},
			Prompt:         "cross profile",
			IdempotencyKey: "foreign",
			Actor:          callspkg.Actor{Kind: "human", ID: "operator:test"},
			Narrow:         callspkg.PermissionAtoms{Skills: []string{"review"}},
		}
		if _, err := foreignService.Create(ctx, foreignInput); !callspkg.IsCode(err, callspkg.CodeTargetDenied) {
			t.Fatalf("Create(cross profile) error = %v, want %s", err, callspkg.CodeTargetDenied)
		}
		if _, err := database.db.ExecContext(
			ctx,
			`UPDATE profiles SET state = 'archived', archived_at = ? WHERE id = ?`,
			store.FormatTimestamp(now),
			secondProfileID,
		); err != nil {
			t.Fatalf("archive secondary profile error = %v", err)
		}
		archivedInput := foreignInput
		archivedInput.ProfileID = secondProfileID
		archivedInput.Prompt = "archived owner"
		archivedInput.IdempotencyKey = "archived"
		if _, err := service.Create(
			ctx,
			archivedInput,
		); err == nil ||
			!strings.Contains(err.Error(), "profile_archived") {
			t.Fatalf("Create(archived profile) error = %v", err)
		}
		actor := taskpkg.ActorContext{
			Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "calls.activation"},
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "calls.activation"},
			Authority: taskpkg.Authority{Read: true, Write: true},
			Scope:     taskpkg.CallerScope{WorkspaceID: workspaceID},
		}
		if _, err := tasks.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: secondaryCall.ActivationRunID, RunKind: taskpkg.RunKindCallActivation,
			Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
		}, actor); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(archived owner) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}
		if _, err := database.db.ExecContext(
			ctx,
			`UPDATE profiles SET state = 'active', archived_at = NULL WHERE id = ?`,
			secondProfileID,
		); err != nil {
			t.Fatalf("unarchive secondary profile error = %v", err)
		}
		if _, err := database.db.ExecContext(ctx, `INSERT INTO profile_lifecycle_ops
		(id, kind, profile_id, old_name, plan_revision, status, created_at, updated_at)
		VALUES ('op_calls_profile_freeze', 'archive', ?, 'calls-secondary', 'revision', 'applied', ?, ?)`,
			secondProfileID, store.FormatTimestamp(now), store.FormatTimestamp(now)); err != nil {
			t.Fatalf("insert profile lifecycle fence error = %v", err)
		}
		if _, err := tasks.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID: secondaryCall.ActivationRunID, RunKind: taskpkg.RunKindCallActivation,
			Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
		}, actor); !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(lifecycle-fenced owner) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
		}
		var rowsAfter int
		if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calls`).Scan(&rowsAfter); err != nil {
			t.Fatalf("count calls after denials error = %v", err)
		}
		if rowsAfter != rowsBefore {
			t.Fatalf("denied ownership writes changed calls from %d to %d", rowsBefore, rowsAfter)
		}
	})

	t.Run("Should drain every persisted call through a forged lineage cycle", func(t *testing.T) {
		nodes := []string{"ses_calls_cycle_d1", "ses_calls_cycle_d2", "ses_calls_cycle_d3"}
		for index, sessionID := range nodes {
			lineage := &store.SessionLineage{
				RootSessionID: nodes[0], SpawnDepth: index,
				SpawnBudget:      store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
				PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
			}
			if index > 0 {
				lineage.ParentSessionID = nodes[index-1]
			}
			registerCallSession(ctx, t, database, store.SessionInfo{
				ID: sessionID, ProfileID: store.DefaultProfileID, AgentName: "reviewer",
				WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
				Lineage: lineage, CreatedAt: now, UpdatedAt: now,
			})
		}
		cycleParents := map[string]string{"node1": nodes[0], "node2": nodes[1], "node3": nodes[2]}
		cycleDepths := map[string]int{"node1": 1, "node2": 2, "node3": 3}
		cycleDirectory := callIntegrationDirectoryFunc(func(
			_ context.Context,
			input callspkg.CreateInput,
		) (callspkg.TargetContext, []callspkg.AgentRosterEntry, error) {
			parentID := cycleParents[input.Target.Agent]
			return callspkg.TargetContext{
				ProfileID:       store.DefaultProfileID,
				WorkspaceID:     workspaceID,
				ParentSessionID: parentID,
				AgentName:       input.Target.Agent,
				GovernedRootID:  nodes[0],
				Depth:           cycleDepths[input.Target.Agent],
				Allowed:         true,
				CallerPolicy:    callIntegrationPermissionPolicy(),
			}, []callspkg.AgentRosterEntry{{Name: input.Target.Agent}}, nil
		})
		cycleService, err := callspkg.NewService(
			callspkg.WithStore(database), callspkg.WithDirectory(cycleDirectory),
			callspkg.WithActivationRunCanceler(tasks), callspkg.WithConfig(config.DefaultCallsConfig()),
			callspkg.WithClock(func() time.Time { return now }), callspkg.WithIDGenerator(store.NewID),
		)
		if err != nil {
			t.Fatalf("calls.NewService(cycle) error = %v", err)
		}
		cycleCallIDs := make([]string, 0, len(nodes))
		for index, agentName := range []string{"node1", "node2", "node3"} {
			cycleCall, createErr := cycleService.Create(ctx, callspkg.CreateInput{
				ProfileID:   store.DefaultProfileID,
				Scope:       callspkg.ScopeWorkspace,
				WorkspaceID: workspaceID,
				Caller: participation.OwnerRef{
					Kind:        participation.OwnerKindTaskRun,
					ID:          "cycle-caller",
					WorkspaceID: workspaceID,
				},
				Target:         callspkg.Target{Agent: agentName},
				Prompt:         "cycle " + agentName,
				IdempotencyKey: fmt.Sprintf("cycle-%d", index),
				Actor:          callspkg.Actor{Kind: "daemon", ID: "cycle-test"},
				Narrow:         callspkg.PermissionAtoms{Skills: []string{"review"}},
			})
			if createErr != nil {
				t.Fatalf("Create(%s) error = %v", agentName, createErr)
			}
			cycleCallIDs = append(cycleCallIDs, cycleCall.CallID)
		}
		if _, err := database.db.ExecContext(ctx, `UPDATE sessions SET parent_session_id = ? WHERE id = ?`,
			nodes[2], nodes[0]); err != nil {
			t.Fatalf("forge lineage cycle error = %v", err)
		}
		cycleCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		openCalls, err := database.ListOpenSubtreeCalls(cycleCtx, nodes[0])
		if err != nil {
			t.Fatalf("ListOpenSubtreeCalls(cycle) error = %v", err)
		}
		if len(openCalls) != 3 {
			t.Fatalf("ListOpenSubtreeCalls(cycle) = %d, want 3", len(openCalls))
		}
		drain, err := cycleService.DrainSubtree(
			cycleCtx,
			nodes[0],
			callspkg.Actor{Kind: "daemon", ID: "recovery"},
			"cycle-safe drain",
		)
		if err != nil {
			t.Fatalf("DrainSubtree(cycle) error = %v", err)
		}
		if len(drain.CanceledCalls) != 3 {
			t.Fatalf("DrainSubtree(cycle) = %#v", drain)
		}
		for _, callID := range cycleCallIDs {
			storedCall, getErr := database.GetCall(cycleCtx, callspkg.CallScope{
				ProfileID:   store.DefaultProfileID,
				Scope:       callspkg.ScopeWorkspace,
				WorkspaceID: workspaceID,
			}, callID)
			if getErr != nil {
				t.Fatalf("GetCall(%q) after cycle drain error = %v", callID, getErr)
			}
			if storedCall.State != callspkg.StateCanceled {
				t.Fatalf(
					"call %q after cycle drain state = %q, want %q",
					callID,
					storedCall.State,
					callspkg.StateCanceled,
				)
			}
		}
		remainingOpen, err := database.ListOpenSubtreeCalls(cycleCtx, nodes[0])
		if err != nil {
			t.Fatalf("ListOpenSubtreeCalls(after cycle drain) error = %v", err)
		}
		if len(remainingOpen) != 0 {
			t.Fatalf("open subtree calls after cycle drain = %#v, want none", remainingOpen)
		}
	})
}

func TestGlobalDBCallActivationQueueEnforcesExactKindAndGovernedRootBudget(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(t, database, "calls-queue", filepath.Join(t.TempDir(), "workspace"))
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID := "ses_calls_queue_parent"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
		},
		CreatedAt: now, UpdatedAt: now,
	})
	service, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithConfig(config.DefaultCallsConfig()), callspkg.WithClock(func() time.Time { return now }),
		callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}
	create := func(key string) callspkg.CallRecord {
		t.Helper()
		record, createErr := service.Create(ctx, callspkg.CreateInput{
			ProfileID:   store.DefaultProfileID,
			Scope:       callspkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Caller: participation.OwnerRef{
				Kind:        participation.OwnerKindSession,
				ID:          parentID,
				WorkspaceID: workspaceID,
			},
			Target:         callspkg.Target{Agent: "reviewer"},
			Prompt:         "queued " + key,
			IdempotencyKey: key,
			Actor:          callspkg.Actor{Kind: "human", ID: "operator:test"},
			Narrow:         callspkg.PermissionAtoms{Skills: []string{"review"}},
		})
		if createErr != nil {
			t.Fatalf("Create(%q) error = %v", key, createErr)
		}
		if record.State != callspkg.StateQueued || record.ActivationRunID == "" {
			t.Fatalf("Create(%q) = %#v, want queued activation", key, record)
		}
		return record
	}
	first, second := create("first"), create("second")

	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(database), taskpkg.WithGovernedRootActiveRunCap(1),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	actor := taskpkg.ActorContext{
		Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "calls.activation"},
		Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "calls.activation"},
		Authority: taskpkg.Authority{Read: true, Write: true},
		Scope:     taskpkg.CallerScope{WorkspaceID: workspaceID},
	}
	claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID: first.ActivationRunID, RunKind: taskpkg.RunKindCallActivation,
		Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
	}, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(first) error = %v", err)
	}
	if claim.Task != nil || claim.Run.RunKind != taskpkg.RunKindCallActivation {
		t.Fatalf("ClaimNextRun(first) = %#v, want taskless call activation", claim)
	}
	_, err = manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID: second.ActivationRunID, RunKind: taskpkg.RunKindCallActivation,
		Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
	}, actor)
	if !errors.Is(err, taskpkg.ErrNoClaimableRun) {
		t.Fatalf("ClaimNextRun(second) error = %v, want %v", err, taskpkg.ErrNoClaimableRun)
	}
	secondStored, err := database.GetCall(ctx, callspkg.CallScope{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
	}, second.CallID)
	if err != nil {
		t.Fatalf("GetCall(second) error = %v", err)
	}
	if secondStored.State != callspkg.StateQueued {
		t.Fatalf("second call state = %s, want queued at root budget", secondStored.State)
	}
	if _, err := database.db.ExecContext(
		ctx,
		`UPDATE task_runs SET run_kind = 'unknown' WHERE id = ?`,
		second.ActivationRunID,
	); err == nil || !strings.Contains(err.Error(), "run_kind") || !strings.Contains(err.Error(), "CHECK") {
		t.Fatalf("unknown task run kind update error = %v, want run_kind CHECK rejection", err)
	}
}

func TestGlobalDBWorkspaceDeletionTerminalizesQueuedCalls(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(t, database, "calls-delete", filepath.Join(t.TempDir(), "workspace"))
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID := "ses_calls_delete_parent"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
		},
		CreatedAt: now, UpdatedAt: now,
	})
	service, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithConfig(config.DefaultCallsConfig()), callspkg.WithClock(func() time.Time { return now }),
		callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}
	record, err := service.Create(ctx, callspkg.CreateInput{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
		Caller: participation.OwnerRef{Kind: participation.OwnerKindSession, ID: parentID, WorkspaceID: workspaceID},
		Target: callspkg.Target{Agent: "reviewer"}, Prompt: "queued for removal",
		Actor:  callspkg.Actor{Kind: "human", ID: "operator:test"},
		Narrow: callspkg.PermissionAtoms{Skills: []string{"review"}},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := database.db.ExecContext(
		ctx,
		`UPDATE sessions SET state = 'stopped' WHERE id = ?`,
		parentID,
	); err != nil {
		t.Fatalf("stop parent fixture error = %v", err)
	}
	activationRunID := record.ActivationRunID
	if err := database.DeleteWorkspace(ctx, workspaceID); err != nil {
		t.Fatalf("DeleteWorkspace() error = %v", err)
	}
	stored, err := database.GetCall(ctx, callspkg.CallScope{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
	}, record.CallID)
	if err != nil {
		t.Fatalf("GetCall() after workspace deletion error = %v", err)
	}
	if stored.State != callspkg.StateFailed || stored.FailureCode != "call_workspace_removed" ||
		stored.ChildSessionID != "" || stored.ActivationRunID != "" {
		t.Fatalf("call after workspace deletion = %#v", stored)
	}
	var runStatus string
	var runError sql.NullString
	if err := database.db.QueryRowContext(
		ctx,
		`SELECT status, error FROM task_runs WHERE id = ?`,
		activationRunID,
	).Scan(&runStatus, &runError); err != nil {
		t.Fatalf("read activation after workspace deletion error = %v", err)
	}
	if runStatus != taskpkg.TaskRunStatusCanceled.String() || !runError.Valid ||
		runError.String != "workspace removed" {
		t.Fatalf("activation after workspace deletion = status %q error %#v", runStatus, runError)
	}
}

func TestGlobalDBOperatorCallerBindingConvergesAndSurvivesReopen(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		database,
		"calls-operator",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	for _, sessionID := range []string{"ses_operator_candidate_a", "ses_operator_candidate_b", "ses_operator_candidate_c"} {
		registerCallSession(ctx, t, database, store.SessionInfo{
			ID: sessionID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
			WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
			Lineage: &store.SessionLineage{
				RootSessionID: sessionID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			},
			CreatedAt: now, UpdatedAt: now,
		})
	}

	type result struct {
		binding callspkg.OperatorCallerBinding
		err     error
	}
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for _, sessionID := range []string{"ses_operator_candidate_a", "ses_operator_candidate_b"} {
		wait.Add(1)
		go func(candidateID string) {
			defer wait.Done()
			binding, bindErr := database.ResolveOperatorCaller(ctx, callspkg.OperatorCallerBinding{
				ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace,
				WorkspaceID: workspaceID, SessionID: candidateID,
			})
			results <- result{binding: binding, err: bindErr}
		}(sessionID)
	}
	wait.Wait()
	close(results)
	winnerID := ""
	created := 0
	for outcome := range results {
		if outcome.err != nil {
			t.Fatalf("ResolveOperatorCaller() error = %v", outcome.err)
		}
		if winnerID == "" {
			winnerID = outcome.binding.SessionID
		}
		if outcome.binding.SessionID != winnerID {
			t.Fatalf("operator callers diverged: %q and %q", winnerID, outcome.binding.SessionID)
		}
		if outcome.binding.Created {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("created winner count = %d, want 1", created)
	}
	path := database.path
	if err := database.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	reopened := openGlobalDBForTest(t, path)
	preserved, err := reopened.ResolveOperatorCaller(ctx, callspkg.OperatorCallerBinding{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace,
		WorkspaceID: workspaceID, SessionID: "ses_operator_candidate_c",
	})
	if err != nil {
		t.Fatalf("ResolveOperatorCaller(reopen) error = %v", err)
	}
	if preserved.Created || preserved.SessionID != winnerID {
		t.Fatalf("reopened binding = %#v, want preserved winner %q", preserved, winnerID)
	}
	excluded, err := reopened.IsOperatorCallerSession(ctx, winnerID)
	if err != nil {
		t.Fatalf("IsOperatorCallerSession() error = %v", err)
	}
	if !excluded {
		t.Fatalf("winner %q is not fenced as operator caller", winnerID)
	}
	drainedAt := now.Add(time.Minute)
	if _, err := reopened.db.ExecContext(ctx, `UPDATE sessions
		SET state = 'stopped', draining_at = ? WHERE id = ?`, store.FormatTimestamp(drainedAt), winnerID); err != nil {
		t.Fatalf("mark operator caller drained error = %v", err)
	}
	if err := reopened.ReopenOperatorCaller(ctx, winnerID, drainedAt.Add(time.Second)); err != nil {
		t.Fatalf("ReopenOperatorCaller() error = %v", err)
	}
	var state string
	var drainingAt sql.NullString
	if err := reopened.db.QueryRowContext(ctx, `SELECT state, draining_at FROM sessions WHERE id = ?`, winnerID).
		Scan(&state, &drainingAt); err != nil {
		t.Fatalf("read reopened operator caller error = %v", err)
	}
	if state != "stopped" || drainingAt.Valid {
		t.Fatalf(
			"reopened operator caller = state %q draining %#v, want stopped with no drain fence",
			state,
			drainingAt,
		)
	}
}

func TestGlobalDBCallIdempotencyRaceAndContractBudgetSnapshots(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		database,
		"calls-idempotency",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID := "ses_calls_idempotency_parent"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
		},
		CreatedAt: now, UpdatedAt: now,
	})
	service, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithConfig(config.DefaultCallsConfig()), callspkg.WithClock(func() time.Time { return now }),
		callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}
	expect := json.RawMessage(
		`{"type":"object","required":["answer"],"properties":{"answer":{"type":"integer"}},"additionalProperties":false}`,
	)
	base := callspkg.CreateInput{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
		Caller: participation.OwnerRef{Kind: participation.OwnerKindSession, ID: parentID, WorkspaceID: workspaceID},
		Target: callspkg.Target{Agent: "reviewer"}, Prompt: "race", Expect: expect,
		IdempotencyKey: "same-key", Actor: callspkg.Actor{Kind: "human", ID: "operator:test"},
		Narrow: callspkg.PermissionAtoms{Skills: []string{"review"}},
	}
	type result struct {
		record callspkg.CallRecord
		err    error
	}
	results := make(chan result, 20)
	var wait sync.WaitGroup
	for range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			record, createErr := service.Create(ctx, base)
			results <- result{record: record, err: createErr}
		}()
	}
	wait.Wait()
	close(results)
	winnerID := ""
	for outcome := range results {
		if outcome.err != nil {
			t.Fatalf("Create(race) error = %v", outcome.err)
		}
		if winnerID == "" {
			winnerID = outcome.record.CallID
		}
		if outcome.record.CallID != winnerID {
			t.Fatalf("idempotency race diverged: %q and %q", winnerID, outcome.record.CallID)
		}
	}
	var callCount, runCount int
	if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calls WHERE idempotency_key = 'same-key'`).
		Scan(&callCount); err != nil {
		t.Fatalf("count idempotent calls error = %v", err)
	}
	if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM call_activation_runs WHERE call_id = ?`, winnerID).
		Scan(&runCount); err != nil {
		t.Fatalf("count idempotent activation runs error = %v", err)
	}
	if callCount != 1 || runCount != 1 {
		t.Fatalf("idempotency rows = calls %d activations %d, want 1 each", callCount, runCount)
	}

	budgets := []contracts.ByteBudget{
		{MaxBytes: 1024, Overflow: contracts.OverflowStore},
		{MaxBytes: 2048, Overflow: contracts.OverflowReject},
	}
	records := make([]callspkg.CallRecord, 0, len(budgets))
	for index := range budgets {
		input := base
		input.Prompt = "budget " + string(rune('a'+index))
		input.IdempotencyKey = "budget-" + string(rune('a'+index))
		input.ResultBudget = &budgets[index]
		record, createErr := service.Create(ctx, input)
		if createErr != nil {
			t.Fatalf("Create(budget %d) error = %v", index, createErr)
		}
		records = append(records, record)
	}
	if records[0].ExpectDigest != records[1].ExpectDigest || records[0].ResultBudget == records[1].ResultBudget {
		t.Fatalf("budget snapshots = %#v / %#v", records[0], records[1])
	}
	var contractRows int
	if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM contract_schemas WHERE digest = ?`, records[0].ExpectDigest).
		Scan(&contractRows); err != nil {
		t.Fatalf("count contract rows error = %v", err)
	}
	if contractRows != 1 {
		t.Fatalf("contract rows = %d, want one deduplicated schema", contractRows)
	}
	conflict := base
	conflict.IdempotencyKey = "budget-a"
	conflict.Prompt = "budget a"
	conflict.ResultBudget = &budgets[1]
	if _, err := service.Create(ctx, conflict); !callspkg.IsCode(err, callspkg.CodeIdempotencyConflict) {
		t.Fatalf("Create(changed replay budget) error = %v, want %s", err, callspkg.CodeIdempotencyConflict)
	}
}

func TestGlobalDBFollowUpAndCompletionUseDistinctDurableDeliveries(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		database,
		"calls-follow-up",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID, childID := "ses_calls_follow_parent", "ses_calls_follow_child"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
		}, CreatedAt: now, UpdatedAt: now,
	})
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: childID, ProfileID: store.DefaultProfileID, AgentName: "reviewer",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			ParentSessionID: parentID, RootSessionID: parentID, SpawnDepth: 1,
			SpawnBudget:      store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
		}, CreatedAt: now, UpdatedAt: now,
	})
	manager, err := taskpkg.NewManager(taskpkg.WithStore(database), taskpkg.WithGovernedRootActiveRunCap(32))
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	service, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithActivationRunCanceler(manager), callspkg.WithConfig(config.DefaultCallsConfig()),
		callspkg.WithClock(func() time.Time { return now }), callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}
	record, err := service.Create(ctx, callspkg.CreateInput{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
		Caller: participation.OwnerRef{Kind: participation.OwnerKindSession, ID: parentID, WorkspaceID: workspaceID},
		Target: callspkg.Target{SessionID: childID}, Prompt: "one more thing",
		Actor:  callspkg.Actor{Kind: "agent_session", ID: parentID},
		Narrow: callspkg.PermissionAtoms{Skills: []string{"review"}},
	})
	if err != nil {
		t.Fatalf("Create(follow-up) error = %v", err)
	}
	if record.State != callspkg.StateRunning || record.ActivationRunID != "" {
		t.Fatalf("Create(follow-up) = %#v", record)
	}
	settlement, err := service.Return(ctx, callspkg.ReturnInput{
		Scope: record.OwnerScope(), CallID: record.CallID, Result: json.RawMessage(`{"done":true}`),
		Actor: callspkg.SettlementActor{Kind: "agent_session", ID: childID},
	})
	if err != nil {
		t.Fatalf("Return(follow-up) error = %v", err)
	}
	if settlement.Call.State != callspkg.StateCompleted {
		t.Fatalf("Return(follow-up) = %#v", settlement)
	}
	rows, err := database.db.QueryContext(ctx, `SELECT kind, delivery_id, wake_event_id FROM call_deliveries
		WHERE subject_id = ? ORDER BY kind`, record.CallID)
	if err != nil {
		t.Fatalf("list follow-up deliveries error = %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Errorf("close follow-up deliveries error = %v", closeErr)
		}
	}()
	seen := make(map[string]string)
	seenDeliveryIDs := make(map[string]struct{})
	rowCount := 0
	for rows.Next() {
		var kind, deliveryID, wakeID string
		if err := rows.Scan(&kind, &deliveryID, &wakeID); err != nil {
			t.Fatalf("scan follow-up delivery error = %v", err)
		}
		rowCount++
		if _, duplicateKind := seen[kind]; duplicateKind {
			t.Fatalf("duplicate follow-up delivery kind %q", kind)
		}
		if _, duplicateID := seenDeliveryIDs[deliveryID]; duplicateID {
			t.Fatalf("duplicate follow-up delivery id %q", deliveryID)
		}
		seen[kind] = deliveryID + ":" + wakeID
		seenDeliveryIDs[deliveryID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate follow-up deliveries error = %v", err)
	}
	if rowCount != 2 || len(seen) != 2 || seen["follow-up"] == "" || seen["completion"] == "" ||
		seen["follow-up"] == seen["completion"] {
		t.Fatalf("follow-up deliveries = %#v", seen)
	}
}

func TestGlobalDBCallActivationClaimCancelRaceHasOneFencedOutcome(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		database,
		"calls-cancel-race",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID := "ses_calls_cancel_race_parent"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
		}, CreatedAt: now, UpdatedAt: now,
	})
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(database), taskpkg.WithGovernedRootActiveRunCap(32),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	service, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithActivationRunCanceler(manager), callspkg.WithConfig(config.DefaultCallsConfig()),
		callspkg.WithClock(func() time.Time { return now }), callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}
	actor := taskpkg.ActorContext{
		Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "calls.activation"},
		Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "calls.activation"},
		Authority: taskpkg.Authority{Read: true, Write: true},
		Scope:     taskpkg.CallerScope{WorkspaceID: workspaceID},
	}
	for iteration := 0; iteration < callIntegrationRaceIterations; iteration++ {
		record, createErr := service.Create(ctx, callspkg.CreateInput{
			ProfileID:   store.DefaultProfileID,
			Scope:       callspkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Caller: participation.OwnerRef{
				Kind:        participation.OwnerKindSession,
				ID:          parentID,
				WorkspaceID: workspaceID,
			},
			Target:         callspkg.Target{Agent: "reviewer"},
			Prompt:         "race",
			IdempotencyKey: fmt.Sprintf("race-%d", iteration),
			Actor:          callspkg.Actor{Kind: "human", ID: "operator:test"},
			Narrow:         callspkg.PermissionAtoms{Skills: []string{"review"}},
		})
		if createErr != nil {
			t.Fatalf("iteration %d Create() error = %v", iteration, createErr)
		}
		start := make(chan struct{})
		claimResult := make(chan error, 1)
		cancelResult := make(chan error, 1)
		go func() {
			<-start
			_, claimErr := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				RunID: record.ActivationRunID, RunKind: taskpkg.RunKindCallActivation,
				Scope: taskpkg.ScopeWorkspace, WorkspaceID: workspaceID,
			}, actor)
			claimResult <- claimErr
		}()
		go func() {
			<-start
			_, cancelErr := service.Cancel(ctx, callspkg.CancelInput{
				Scope: record.OwnerScope(), CallID: record.CallID, Reason: "race cancellation", Actor: record.Actor,
			})
			cancelResult <- cancelErr
		}()
		close(start)
		claimErr, cancelErr := <-claimResult, <-cancelResult
		if claimErr != nil && !errors.Is(claimErr, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("iteration %d ClaimNextRun() error = %v", iteration, claimErr)
		}
		if cancelErr != nil {
			t.Fatalf("iteration %d Cancel() error = %v", iteration, cancelErr)
		}
		stored, getErr := database.GetCall(ctx, callspkg.CallScope{
			ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
		}, record.CallID)
		if getErr != nil {
			t.Fatalf("iteration %d GetCall() error = %v", iteration, getErr)
		}
		var runStatus string
		if getErr := database.db.QueryRowContext(ctx, `SELECT status FROM task_runs WHERE id = ?`, record.ActivationRunID).
			Scan(&runStatus); getErr != nil {
			t.Fatalf("iteration %d read activation status error = %v", iteration, getErr)
		}
		if stored.State != callspkg.StateCanceled || runStatus != taskpkg.TaskRunStatusCanceled.String() {
			t.Fatalf("iteration %d result = call %s run %s", iteration, stored.State, runStatus)
		}
	}
}

func TestGlobalDBCallCancelReturnRacePreservesOneTerminalOutcome(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		database,
		"calls-return-race",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID := "ses_calls_return_race_parent"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 100, MaxDepth: 3},
			PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
		}, CreatedAt: now, UpdatedAt: now,
	})
	tasks, err := taskpkg.NewManager(
		taskpkg.WithStore(database), taskpkg.WithGovernedRootActiveRunCap(100),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	cfg := config.DefaultCallsConfig()
	cfg.MaxChildren = 100
	invoker := &callIntegrationInvoker{database: database, workspaceID: workspaceID, now: now}
	service, err := callspkg.NewService(
		callspkg.WithStore(database), callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithActivationClaimer(tasks), callspkg.WithActivationRunCanceler(tasks),
		callspkg.WithSessionInvoker(invoker), callspkg.WithConfig(cfg),
		callspkg.WithClock(func() time.Time { return now }), callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}
	for iteration := 0; iteration < callIntegrationRaceIterations; iteration++ {
		record, createErr := service.Create(ctx, callspkg.CreateInput{
			ProfileID:   store.DefaultProfileID,
			Scope:       callspkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Caller: participation.OwnerRef{
				Kind:        participation.OwnerKindSession,
				ID:          parentID,
				WorkspaceID: workspaceID,
			},
			Target:         callspkg.Target{Agent: "reviewer"},
			Prompt:         "return race",
			IdempotencyKey: fmt.Sprintf("return-race-%d", iteration),
			Actor:          callspkg.Actor{Kind: "human", ID: "operator:test"},
			Narrow:         callspkg.PermissionAtoms{Skills: []string{"review"}},
		})
		if createErr != nil {
			t.Fatalf("iteration %d Create() error = %v", iteration, createErr)
		}
		start := make(chan struct{})
		cancelResult := make(chan error, 1)
		returnResult := make(chan error, 1)
		go func() {
			<-start
			_, cancelErr := service.Cancel(ctx, callspkg.CancelInput{
				Scope: record.OwnerScope(), CallID: record.CallID, Reason: "operator race", Actor: record.Actor,
			})
			cancelResult <- cancelErr
		}()
		go func() {
			<-start
			_, returnErr := service.Return(ctx, callspkg.ReturnInput{
				Scope: record.OwnerScope(), CallID: record.CallID, Result: json.RawMessage(`{"done":true}`),
				Actor: callspkg.SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
			})
			returnResult <- returnErr
		}()
		close(start)
		cancelErr, returnErr := <-cancelResult, <-returnResult
		if cancelErr != nil {
			t.Fatalf("iteration %d Cancel() error = %v", iteration, cancelErr)
		}
		stored, getErr := database.GetCall(ctx, callspkg.CallScope{
			ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
		}, record.CallID)
		if getErr != nil {
			t.Fatalf("iteration %d GetCall() error = %v", iteration, getErr)
		}
		switch stored.State {
		case callspkg.StateCompleted:
			if returnErr != nil || stored.ResultRef == "" {
				t.Fatalf("iteration %d completed winner error=%v call=%#v", iteration, returnErr, stored)
			}
		case callspkg.StateCanceled:
			if !callspkg.IsCode(returnErr, callspkg.CodeAlreadySettled) || stored.SupersededRef == "" {
				t.Fatalf("iteration %d canceled winner error=%v call=%#v", iteration, returnErr, stored)
			}
		default:
			t.Fatalf("iteration %d terminal state = %s", iteration, stored.State)
		}
		var deliveries int
		if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM call_deliveries
			WHERE subject_id = ? AND kind = 'completion'`, record.CallID).Scan(&deliveries); err != nil {
			t.Fatalf("iteration %d count completion deliveries error = %v", iteration, err)
		}
		if deliveries != 1 {
			t.Fatalf("iteration %d completion deliveries = %d, want 1", iteration, deliveries)
		}
	}
}

func TestGlobalDBCallsAdmissionActivationSettlementAndReplay(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		database,
		"calls-runtime",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID := "ses_calls_parent"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID:    parentID,
			SpawnBudget:      store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			PermissionPolicy: store.SessionPermissionPolicy{Skills: []string{"review"}},
		},
		CreatedAt: now, UpdatedAt: now,
	})

	tasks, err := taskpkg.NewManager(
		taskpkg.WithStore(database),
		taskpkg.WithGovernedRootActiveRunCap(32),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	invoker := &callIntegrationInvoker{database: database, workspaceID: workspaceID, now: now}
	service, err := callspkg.NewService(
		callspkg.WithStore(database),
		callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithActivationClaimer(tasks),
		callspkg.WithActivationRunCanceler(tasks),
		callspkg.WithSessionInvoker(invoker),
		callspkg.WithConfig(config.DefaultCallsConfig()),
		callspkg.WithClock(func() time.Time { return now }),
		callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}

	prompt := strings.Repeat("inspect this carefully; ", 2048)
	expect := json.RawMessage(
		`{"type":"object","required":["answer"],"properties":{"answer":{"type":"integer"}},"additionalProperties":false}`,
	)
	input := callspkg.CreateInput{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
		Caller: participation.OwnerRef{Kind: participation.OwnerKindSession, ID: parentID, WorkspaceID: workspaceID},
		Target: callspkg.Target{Agent: "reviewer"}, Prompt: prompt, Expect: expect,
		IdempotencyKey: "calls-admission-replay", Actor: callspkg.Actor{Kind: "human", ID: "operator:test"},
		Narrow: callspkg.PermissionAtoms{Skills: []string{"review"}},
	}
	record, err := service.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if record.State != callspkg.StateQueued || record.ActivationRunID == "" || record.ExpectDigest == "" {
		t.Fatalf("Create() = %#v, want queued contracted activation", record)
	}
	if got := invoker.spawnCount(); got != 0 {
		t.Fatalf("SpawnChild() calls before dispatch = %d, want 0", got)
	}
	if dispatched, dispatchErr := service.DispatchQueued(ctx, 1); dispatchErr != nil || dispatched != 1 {
		t.Fatalf("DispatchQueued() = %d, %v, want one activation", dispatched, dispatchErr)
	}
	record, err = database.GetCall(ctx, record.OwnerScope(), record.CallID)
	if err != nil {
		t.Fatalf("GetCall(activated) error = %v", err)
	}
	if record.State != callspkg.StateRunning || record.ChildSessionID == "" {
		t.Fatalf("GetCall(activated) = %#v, want running child", record)
	}
	if got := invoker.spawnCount(); got != 1 {
		t.Fatalf("SpawnChild() calls = %d, want 1", got)
	}
	persistedPrompt, err := database.CallRepo.queries.GetCallPayload(ctx, sqlcgen.GetCallPayloadParams{
		WorkspaceID: workspaceID, Ref: record.PromptRef,
	})
	if err != nil {
		t.Fatalf("GetCallPayload(prompt) error = %v", err)
	}
	if string(persistedPrompt.Bytes) != prompt {
		t.Fatalf("persisted prompt bytes changed: got %d, want %d", len(persistedPrompt.Bytes), len(prompt))
	}

	replayed, err := service.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create(replay) error = %v", err)
	}
	if !replayed.Replayed || replayed.CallID != record.CallID || invoker.spawnCount() != 1 {
		t.Fatalf("Create(replay) = %#v, spawns=%d", replayed, invoker.spawnCount())
	}
	followUpInput := input
	followUpInput.Target = callspkg.Target{SessionID: record.ChildSessionID}
	followUpInput.Prompt = "Inspect the follow-up without interrupting the current turn."
	followUpInput.IdempotencyKey = "calls-follow-up-boundary"
	followUp, err := service.Create(ctx, followUpInput)
	if err != nil {
		t.Fatalf("Create(follow-up) error = %v", err)
	}
	if followUp.State != callspkg.StateRunning || followUp.ActivationRunID != "" {
		t.Fatalf("Create(follow-up) = %#v, want durable running boundary delivery", followUp)
	}
	if err := service.DrainDeliveries(ctx, record.ChildSessionID, 10); err != nil {
		t.Fatalf("DrainDeliveries(follow-up) error = %v", err)
	}
	delivered := invoker.recordedDeliveries()
	if len(delivered) != 1 || delivered[0].Body != followUpInput.Prompt ||
		delivered[0].Kind != callspkg.DeliveryKindFollowUp ||
		delivered[0].Metadata.CallID != followUp.CallID ||
		delivered[0].Metadata.Reason != "call_follow_up" {
		t.Fatalf("follow-up deliveries = %#v, want persisted prompt with structured identity", delivered)
	}
	repair, err := service.Return(ctx, callspkg.ReturnInput{
		Scope: record.OwnerScope(), CallID: record.CallID, ChildSessionID: record.ChildSessionID,
		Result: json.RawMessage(`{"wrong":true}`), ChildLive: true,
		Actor: callspkg.SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
	})
	if err != nil {
		t.Fatalf("Return(invalid) error = %v", err)
	}
	if repair.Call.State != callspkg.StateRunning || repair.Call.RepairAttempts != 1 || repair.RepairPrompt == "" {
		t.Fatalf("Return(invalid) = %#v, want one repair", repair)
	}
	var repairDeliveries int
	if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM call_deliveries
		WHERE subject_id = ? AND kind = 'repair'`, record.CallID).Scan(&repairDeliveries); err != nil {
		t.Fatalf("count repair deliveries error = %v", err)
	}
	if repairDeliveries != 1 {
		t.Fatalf("repair delivery rows = %d, want 1", repairDeliveries)
	}
	invoker.setDeliveryOutcome(callspkg.DeliveryOutcome{State: "injected", Reason: "turn_boundary"})
	if err := service.DrainDeliveries(ctx, record.ChildSessionID, 10); err != nil {
		t.Fatalf("DrainDeliveries(repair) error = %v", err)
	}
	followUpSettlement, err := service.Return(ctx, callspkg.ReturnInput{
		Scope: followUp.OwnerScope(), CallID: followUp.CallID, ChildSessionID: record.ChildSessionID,
		Result: json.RawMessage(`{"answer":7}`),
		Actor:  callspkg.SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
	})
	if err != nil || followUpSettlement.Call.State != callspkg.StateCompleted {
		t.Fatalf("Return(follow-up) = %#v, %v, want completed before final park", followUpSettlement, err)
	}

	settlement, err := service.Return(ctx, callspkg.ReturnInput{
		Scope: record.OwnerScope(), CallID: record.CallID, ChildSessionID: record.ChildSessionID,
		Result: json.RawMessage(`{"answer":42}`),
		Actor:  callspkg.SettlementActor{Kind: "agent_session", ID: record.ChildSessionID},
	})
	if err != nil {
		t.Fatalf("Return() error = %v", err)
	}
	if settlement.Call.State != callspkg.StateCompleted || settlement.Call.Verdict != callspkg.VerdictRepaired {
		t.Fatalf("Return() = %#v, want completed/repaired", settlement)
	}
	var childState string
	var parkedAt, idleExpiresAt sql.NullString
	if err := database.db.QueryRowContext(ctx, `SELECT state, parked_at, idle_expires_at
		FROM sessions WHERE id = ?`, record.ChildSessionID).Scan(&childState, &parkedAt, &idleExpiresAt); err != nil {
		t.Fatalf("read parked call child error = %v", err)
	}
	wantIdleExpiry := store.FormatTimestamp(now.Add(record.IdleTTL))
	if childState != "stopped" || !parkedAt.Valid || parkedAt.String != store.FormatTimestamp(now) ||
		!idleExpiresAt.Valid || idleExpiresAt.String != wantIdleExpiry {
		t.Fatalf(
			"parked child lifecycle = state %q parked %q idle %q, want stopped/%q/%q",
			childState,
			parkedAt.String,
			idleExpiresAt.String,
			store.FormatTimestamp(now),
			wantIdleExpiry,
		)
	}
	var deliveries int
	if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM call_deliveries
		WHERE subject_id = ? AND kind = 'completion'`, record.CallID).
		Scan(&deliveries); err != nil {
		t.Fatalf("count completion deliveries error = %v", err)
	}
	if deliveries != 1 {
		t.Fatalf("completion delivery rows = %d, want 1", deliveries)
	}
}

func registerCallSession(ctx context.Context, t *testing.T, database *GlobalDB, info store.SessionInfo) {
	t.Helper()
	if info.Lineage != nil && len(info.Lineage.PermissionPolicy.Tools) == 0 {
		lineage := *info.Lineage
		lineage.PermissionPolicy.Tools = callIntegrationHelperTools()
		info.Lineage = &lineage
	}
	if err := database.RegisterSession(ctx, info); err != nil {
		t.Fatalf("RegisterSession(%q) error = %v", info.ID, err)
	}
}

func callIntegrationPermissionPolicy() store.SessionPermissionPolicy {
	return store.SessionPermissionPolicy{
		Tools:  callIntegrationHelperTools(),
		Skills: []string{"review"},
	}
}

func callIntegrationHelperTools() []string {
	return []string{
		toolspkg.ToolIDAgentCall.String(),
		toolspkg.ToolIDAgentMessage.String(),
		toolspkg.ToolIDCallAwait.String(),
		toolspkg.ToolIDCallCancel.String(),
		toolspkg.ToolIDCallResult.String(),
		toolspkg.ToolIDCallReturn.String(),
	}
}

type globalDBCallMailboxFixture struct {
	ctx         context.Context
	database    *GlobalDB
	service     *callspkg.Service
	input       callspkg.SendMessageInput
	workspaceID string
	parentID    string
	childID     string
	now         time.Time
}

func newGlobalDBCallMailboxFixture(t *testing.T) globalDBCallMailboxFixture {
	t.Helper()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t, database, "calls-mailbox", filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID, childID := "ses_mailbox_parent", "ses_mailbox_child"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
		}, CreatedAt: now, UpdatedAt: now,
	})
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: childID, ProfileID: store.DefaultProfileID, AgentName: "reviewer",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			ParentSessionID: parentID, RootSessionID: parentID, SpawnDepth: 1,
			SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
		}, CreatedAt: now, UpdatedAt: now,
	})
	invoker := &callIntegrationInvoker{
		database: database, workspaceID: workspaceID, now: now,
		deliveryOutcome: callspkg.DeliveryOutcome{State: "woken", Reason: "recipient_revived"},
	}
	service, err := callspkg.NewService(
		callspkg.WithStore(database),
		callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithSessionInvoker(invoker),
		callspkg.WithConfig(config.DefaultCallsConfig()),
		callspkg.WithClock(func() time.Time { return now }),
		callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}
	bindingCall, err := service.Create(ctx, callspkg.CreateInput{
		ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
		Caller: participation.OwnerRef{
			Kind: participation.OwnerKindSession, ID: parentID, WorkspaceID: workspaceID,
		},
		Target: callspkg.Target{SessionID: childID}, Prompt: "bind child mailbox to parent",
		Actor: callspkg.Actor{Kind: "human", ID: "operator:test"},
	})
	if err != nil {
		t.Fatalf("Create(mailbox binding call) error = %v", err)
	}
	if bindingCall.ChildSessionID != childID || bindingCall.State != callspkg.StateRunning {
		t.Fatalf("mailbox binding call = %#v, want running call for child %q", bindingCall, childID)
	}
	if _, err := database.db.ExecContext(ctx, `UPDATE sessions
		SET state = 'stopped', parked_at = ?, idle_expires_at = ? WHERE id = ?`,
		store.FormatTimestamp(now.Add(-time.Minute)), store.FormatTimestamp(now.Add(time.Hour)), parentID); err != nil {
		t.Fatalf("park message target error = %v", err)
	}
	return globalDBCallMailboxFixture{
		ctx: ctx, database: database, service: service,
		workspaceID: workspaceID, parentID: parentID, childID: childID, now: now,
		input: callspkg.SendMessageInput{
			ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
			From: callspkg.MessageSender{Kind: "session", ID: childID}, To: "parent",
		},
	}
}

func TestGlobalDBCallMailboxCommitsBeforeDeliveryAndEnforcesLoopBreakers(t *testing.T) {
	t.Parallel()

	t.Run("Should commit sanitize deduplicate and wake a durable message", func(t *testing.T) {
		fixture := newGlobalDBCallMailboxFixture(t)
		ctx, database, service := fixture.ctx, fixture.database, fixture.service
		workspaceID, parentID, childID := fixture.workspaceID, fixture.parentID, fixture.childID
		input := fixture.input
		input.Body = "blocked on COMPOZY_CLAIM_secret-value"
		record, err := service.SendMessage(ctx, input)
		if err != nil {
			t.Fatalf("SendMessage() error = %v", err)
		}
		if record.ToSessionID != parentID || record.Delivery != "queued" ||
			strings.Contains(record.Body, "secret-value") {
			t.Fatalf("SendMessage() = %#v, want sanitized durable queued receipt", record)
		}
		var messageRows, deliveryRows int
		if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM call_messages WHERE message_id = ?`,
			record.MessageID).Scan(&messageRows); err != nil {
			t.Fatalf("count call message rows error = %v", err)
		}
		if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM call_deliveries
		WHERE kind = 'message' AND subject_id = ?`, record.MessageID).Scan(&deliveryRows); err != nil {
			t.Fatalf("count call delivery rows error = %v", err)
		}
		if messageRows != 1 || deliveryRows != 1 {
			t.Fatalf("committed rows = message %d delivery %d, want 1/1", messageRows, deliveryRows)
		}
		var parkedAt, idleExpiresAt sql.NullString
		if err := database.db.QueryRowContext(ctx, `SELECT parked_at, idle_expires_at FROM sessions WHERE id = ?`,
			parentID).Scan(&parkedAt, &idleExpiresAt); err != nil {
			t.Fatalf("read revived message target error = %v", err)
		}
		if !parkedAt.Valid || idleExpiresAt.Valid {
			t.Fatalf(
				"queued message target lifecycle = parked %q idle %q, want parked with suspended idle clock",
				parkedAt.String,
				idleExpiresAt.String,
			)
		}
		_, err = service.SendMessage(ctx, input)
		var duplicate *callspkg.Error
		if !errors.As(err, &duplicate) || duplicate.Code != callspkg.CodeMessageDuplicate ||
			duplicate.OriginalID != record.MessageID {
			t.Fatalf("SendMessage(duplicate) error = %#v, want original %q", err, record.MessageID)
		}
		pending, err := database.ListPendingDeliveries(ctx, parentID, 10)
		if err != nil || len(pending) != 1 {
			t.Fatalf("ListPendingDeliveries() = %#v, %v, want one", pending, err)
		}
		if pending[0].OwnerKey != "session:"+childID || pending[0].ProfileID != store.DefaultProfileID ||
			pending[0].Scope != callspkg.ScopeWorkspace || pending[0].WorkspaceID != workspaceID {
			t.Fatalf("message delivery ownership = %#v, want sender and exact workspace", pending[0])
		}
		if err := service.DrainDeliveries(ctx, parentID, 10); err != nil {
			t.Fatalf("DrainDeliveries() error = %v", err)
		}
		receipt, err := service.Message(ctx, callspkg.CallScope{
			ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
		}, record.MessageID)
		if err != nil || receipt.Delivery != "woke" {
			t.Fatalf("Message() = %#v, %v, want woke", receipt, err)
		}
		if _, err := service.Message(ctx, callspkg.CallScope{
			ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: "ws-other",
		}, record.MessageID); !callspkg.IsCode(err, callspkg.CodeMessageNotFound) {
			t.Fatalf("Message(other workspace) error = %v, want %s", err, callspkg.CodeMessageNotFound)
		}
		if err := database.db.QueryRowContext(ctx, `SELECT parked_at, idle_expires_at FROM sessions WHERE id = ?`,
			parentID).Scan(&parkedAt, &idleExpiresAt); err != nil {
			t.Fatalf("read woken message target error = %v", err)
		}
		if parkedAt.Valid || idleExpiresAt.Valid {
			t.Fatalf(
				"woken message target lifecycle = parked %q idle %q, want active",
				parkedAt.String,
				idleExpiresAt.String,
			)
		}
		var callWakeRows, networkWakeRows int
		if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM call_deliveries
		WHERE wake_event_id = ? AND state = 'woken' AND owner_key = ?`,
			pending[0].WakeEventID, pending[0].OwnerKey).Scan(&callWakeRows); err != nil {
			t.Fatalf("count accounted call wake rows error = %v", err)
		}
		if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM network_live_wakes WHERE wake_id = ?`,
			pending[0].WakeEventID).Scan(&networkWakeRows); err != nil {
			t.Fatalf("count double-booked network wake rows error = %v", err)
		}
		if callWakeRows != 1 || networkWakeRows != 0 {
			t.Fatalf(
				"wake accounting = call %d network %d, want one owner-keyed call row only",
				callWakeRows,
				networkWakeRows,
			)
		}
	})

	t.Run("Should fail a delivery after the retry budget", func(t *testing.T) {
		fixture := newGlobalDBCallMailboxFixture(t)
		input := fixture.input
		input.Body = "retry probe"
		record, err := fixture.service.SendMessage(fixture.ctx, input)
		if err != nil {
			t.Fatalf("SendMessage(retry probe) error = %v", err)
		}
		pending, err := fixture.database.ListPendingDeliveries(fixture.ctx, fixture.parentID, 10)
		if err != nil || len(pending) != 1 || pending[0].SubjectID != record.MessageID {
			t.Fatalf("ListPendingDeliveries(retry probe) = %#v, %v", pending, err)
		}
		for attempt := 1; attempt <= 3; attempt++ {
			updated, updateErr := fixture.database.RecordDelivery(fixture.ctx, callspkg.DeliveryUpdate{
				DeliveryID: pending[0].DeliveryID, State: "pending", Reason: "recipient_unavailable",
				At: fixture.now.Add(time.Duration(attempt) * time.Second), MaxAttempts: 3,
			})
			if updateErr != nil {
				t.Fatalf("RecordDelivery(attempt %d) error = %v", attempt, updateErr)
			}
			if attempt < 3 && updated.State != "pending" || attempt == 3 && updated.State != "failed" {
				t.Fatalf("RecordDelivery(attempt %d) state = %q", attempt, updated.State)
			}
		}
		failed, err := fixture.service.Message(fixture.ctx, callspkg.CallScope{
			ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: fixture.workspaceID,
		}, record.MessageID)
		if err != nil || failed.Delivery != "failed" || failed.DeliveryReason != "recipient_unavailable" {
			t.Fatalf("Message(failed) = %#v, %v", failed, err)
		}
	})

	t.Run("Should reject an oversized message before admission", func(t *testing.T) {
		fixture := newGlobalDBCallMailboxFixture(t)
		configWithTinyBody := config.DefaultCallsConfig()
		configWithTinyBody.Messages.MaxBytes = "4B"
		tinyService, err := callspkg.NewService(
			callspkg.WithStore(fixture.database),
			callspkg.WithDirectory(callIntegrationDirectory{database: fixture.database}),
			callspkg.WithConfig(configWithTinyBody),
			callspkg.WithClock(func() time.Time { return fixture.now.Add(time.Minute) }),
			callspkg.WithIDGenerator(store.NewID),
		)
		if err != nil {
			t.Fatalf("calls.NewService(tiny body) error = %v", err)
		}
		input := fixture.input
		input.Body = "12345"
		if _, err := tinyService.SendMessage(fixture.ctx, input); !callspkg.IsCode(err, callspkg.CodeMessageTooLarge) {
			t.Fatalf("SendMessage(too large) error = %v, want %s", err, callspkg.CodeMessageTooLarge)
		}
	})

	t.Run("Should return a typed missing-message error", func(t *testing.T) {
		fixture := newGlobalDBCallMailboxFixture(t)
		if _, err := fixture.service.Message(fixture.ctx, callspkg.CallScope{
			ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: fixture.workspaceID,
		}, "msg_missing"); !callspkg.IsCode(err, callspkg.CodeMessageNotFound) {
			t.Fatalf("Message(missing) error = %v, want %s", err, callspkg.CodeMessageNotFound)
		}
	})

	t.Run("Should reject a blocked message target", func(t *testing.T) {
		fixture := newGlobalDBCallMailboxFixture(t)
		if _, err := fixture.database.db.ExecContext(
			fixture.ctx,
			`UPDATE sessions SET pending_permission_count = 1 WHERE id = ?`,
			fixture.parentID,
		); err != nil {
			t.Fatalf("mark message target blocked error = %v", err)
		}
		input := fixture.input
		input.Body = "blocked target probe"
		if _, err := fixture.service.SendMessage(
			fixture.ctx,
			input,
		); !callspkg.IsCode(
			err,
			callspkg.CodeMessageTargetBlocked,
		) {
			t.Fatalf("SendMessage(blocked target) error = %v, want %s", err, callspkg.CodeMessageTargetBlocked)
		}
	})

	t.Run("Should distinguish a missing mailbox target from policy denial", func(t *testing.T) {
		fixture := newGlobalDBCallMailboxFixture(t)
		input := fixture.input
		input.To = "ses_mailbox_missing"
		input.Body = "missing target probe"

		_, err := fixture.service.SendMessage(fixture.ctx, input)

		if !callspkg.IsCode(err, callspkg.CodeNotFound) {
			t.Fatalf("SendMessage(missing target) error = %v, want %s", err, callspkg.CodeNotFound)
		}
	})

	t.Run("Should reject a target outside the sender lineage", func(t *testing.T) {
		fixture := newGlobalDBCallMailboxFixture(t)
		outsideID := "ses_mailbox_outside"
		registerCallSession(fixture.ctx, t, fixture.database, store.SessionInfo{
			ID: outsideID, ProfileID: store.DefaultProfileID, AgentName: "outsider",
			WorkspaceID: fixture.workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
			Lineage: &store.SessionLineage{
				RootSessionID: outsideID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			}, CreatedAt: fixture.now, UpdatedAt: fixture.now,
		})
		input := fixture.input
		input.To = outsideID
		input.Body = "cross-lineage probe"
		if _, err := fixture.service.SendMessage(
			fixture.ctx,
			input,
		); !callspkg.IsCode(
			err,
			callspkg.CodeMessageTargetDenied,
		) {
			t.Fatalf("SendMessage(outside lineage) error = %v, want %s", err, callspkg.CodeMessageTargetDenied)
		}
	})
}

// Invariant: the pending delivery ceiling protects one recipient's backlog,
// regardless of how many different senders contributed to it. Owning layer:
// CallRepo message admission. Canonical suite: the GlobalDB calls integration
// suite, which owns durable mailbox admission and delivery state.
func TestGlobalDBCallMailboxPendingCapIsPerRecipient(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t, database, "calls-mailbox-recipient-cap", filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID := "ses_mailbox_cap_parent"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
		}, CreatedAt: now, UpdatedAt: now,
	})
	for _, childID := range []string{"ses_mailbox_cap_child_a", "ses_mailbox_cap_child_b"} {
		registerCallSession(ctx, t, database, store.SessionInfo{
			ID: childID, ProfileID: store.DefaultProfileID, AgentName: "worker",
			WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
			Lineage: &store.SessionLineage{
				ParentSessionID: parentID, RootSessionID: parentID, SpawnDepth: 1,
				SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			}, CreatedAt: now, UpdatedAt: now,
		})
	}

	admit := func(messageID, senderID string, offset time.Duration) error {
		_, err := database.AcceptMessage(ctx, callspkg.MessageAdmission{
			Record: callspkg.MessageRecord{
				MessageID: messageID, ProfileID: store.DefaultProfileID,
				Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
				From:        callspkg.MessageSender{Kind: "session", ID: senderID},
				ToSessionID: parentID, Body: messageID, DedupHash: "hash_" + messageID,
				CreatedAt: now.Add(offset),
			},
			Target: parentID, DedupWindow: time.Second, RateLimit: 10, PendingCap: 2,
		})
		return err
	}
	if err := admit("msg_cap_a", "ses_mailbox_cap_child_a", 0); err != nil {
		t.Fatalf("AcceptMessage(first sender) error = %v", err)
	}
	if err := admit("msg_cap_b", "ses_mailbox_cap_child_b", time.Second); err != nil {
		t.Fatalf("AcceptMessage(second sender) error = %v", err)
	}
	err := admit("msg_cap_c", "ses_mailbox_cap_child_a", 2*time.Second)
	if !callspkg.IsCode(err, callspkg.CodeMessagePendingCap) {
		t.Fatalf("AcceptMessage(third recipient delivery) error = %v, want %s", err, callspkg.CodeMessagePendingCap)
	}
}

func TestGlobalDBCallReapCleanupMatrix(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		database,
		"calls-reap-cleanup",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

	t.Run("Should fail every pending delivery kind for the reaped recipient", func(t *testing.T) {
		recipientID := "ses_reap_delivery_recipient"
		registerCallSession(ctx, t, database, store.SessionInfo{
			ID: recipientID, ProfileID: store.DefaultProfileID, AgentName: "worker",
			WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
			Lineage: &store.SessionLineage{
				RootSessionID: recipientID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			}, CreatedAt: now, UpdatedAt: now,
		})
		for _, kind := range []callspkg.DeliveryKind{
			callspkg.DeliveryKindMessage,
			callspkg.DeliveryKindCompletion,
			callspkg.DeliveryKindRepair,
		} {
			if _, err := database.db.ExecContext(ctx, `INSERT INTO call_deliveries (
				delivery_id, kind, subject_id, recipient_session_id, owner_key, wake_event_id,
				state, created_at, updated_at
			) VALUES (?, ?, ?, ?, 'session:sender', ?, 'pending', ?, ?)`,
				"delivery_reap_"+string(kind), kind, "subject_"+string(kind), recipientID,
				"wake_reap_"+string(kind), store.FormatTimestamp(now), store.FormatTimestamp(now),
			); err != nil {
				t.Fatalf("insert %s delivery error = %v", kind, err)
			}
		}

		if err := database.FailPendingDeliveriesForRecipient(
			ctx,
			recipientID,
			"session reaped",
			now.Add(time.Second),
		); err != nil {
			t.Fatalf("FailPendingDeliveriesForRecipient() error = %v", err)
		}
		var failed int
		if err := database.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM call_deliveries
			WHERE recipient_session_id = ? AND state = 'failed' AND reason = 'session reaped'`, recipientID).
			Scan(&failed); err != nil {
			t.Fatalf("count failed reap deliveries error = %v", err)
		}
		if failed != 3 {
			t.Fatalf("failed reap deliveries = %d, want all three kinds", failed)
		}
	})

	t.Run("Should preserve exact reap detail while mapping runtime stop reasons", func(t *testing.T) {
		cases := []struct {
			reason string
			want   string
		}{
			{reason: "ttl_expired", want: "timeout"},
			{reason: "parent_stopped", want: "user_canceled"},
			{reason: "runtime_unavailable", want: "error"},
		}
		for index, tc := range cases {
			sessionID := fmt.Sprintf("ses_reap_reason_%d", index)
			registerCallSession(ctx, t, database, store.SessionInfo{
				ID: sessionID, ProfileID: store.DefaultProfileID, AgentName: "worker",
				WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
				Lineage: &store.SessionLineage{
					RootSessionID: sessionID,
					SpawnBudget:   store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
				}, CreatedAt: now, UpdatedAt: now,
			})
			if err := database.FinalizeReapedSession(ctx, sessionID, tc.reason, now.Add(time.Second)); err != nil {
				t.Fatalf("FinalizeReapedSession(%q) error = %v", tc.reason, err)
			}
			var state, stopReason, stopDetail string
			if err := database.db.QueryRowContext(ctx, `SELECT state, stop_reason, stop_detail
				FROM sessions WHERE id = ?`, sessionID).Scan(&state, &stopReason, &stopDetail); err != nil {
				t.Fatalf("read reaped session %q error = %v", tc.reason, err)
			}
			if state != "stopped" || stopReason != tc.want || stopDetail != tc.reason {
				t.Fatalf(
					"reaped session %q = state %q reason %q detail %q, want stopped/%s/%s",
					tc.reason,
					state,
					stopReason,
					stopDetail,
					tc.want,
					tc.reason,
				)
			}
		}
	})
}

func TestGlobalDBCallAdmissionRacingReaperHasOneDurableWinner(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	database := openFreshTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t, database, "calls-reaper-race", filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	parentID := "ses_reaper_race_parent"
	registerCallSession(ctx, t, database, store.SessionInfo{
		ID: parentID, ProfileID: store.DefaultProfileID, AgentName: "coordinator",
		WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			RootSessionID: parentID, SpawnBudget: store.SessionSpawnBudget{MaxChildren: 100, MaxDepth: 3},
		}, CreatedAt: now, UpdatedAt: now,
	})
	service, err := callspkg.NewService(
		callspkg.WithStore(database),
		callspkg.WithDirectory(callIntegrationDirectory{database: database}),
		callspkg.WithConfig(config.DefaultCallsConfig()),
		callspkg.WithClock(func() time.Time { return now }),
		callspkg.WithIDGenerator(store.NewID),
	)
	if err != nil {
		t.Fatalf("calls.NewService() error = %v", err)
	}
	for iteration := range callIntegrationRaceIterations {
		childID := fmt.Sprintf("ses_reaper_race_child_%02d", iteration)
		registerCallSession(ctx, t, database, store.SessionInfo{
			ID: childID, ProfileID: store.DefaultProfileID, AgentName: "reviewer",
			WorkspaceID: workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
			Lineage: &store.SessionLineage{
				ParentSessionID: parentID, RootSessionID: parentID, SpawnDepth: 1,
				SpawnBudget: store.SessionSpawnBudget{MaxChildren: 100, MaxDepth: 3},
			}, CreatedAt: now, UpdatedAt: now,
		})
		parked, parkErr := database.ParkCallChild(ctx, childID, now, now.Add(time.Minute))
		if parkErr != nil || !parked {
			t.Fatalf("ParkCallChild(%d) = %v, %v", iteration, parked, parkErr)
		}
		if _, err := database.db.ExecContext(
			ctx,
			`UPDATE sessions SET state = 'stopped' WHERE id = ?`,
			childID,
		); err != nil {
			t.Fatalf("mark parked child stopped (%d) error = %v", iteration, err)
		}
		start := make(chan struct{})
		createResult := make(chan error, 1)
		fenceResult := make(chan struct {
			won bool
			err error
		}, 1)
		go func() {
			<-start
			_, createErr := service.Create(ctx, callspkg.CreateInput{
				ProfileID: store.DefaultProfileID, Scope: callspkg.ScopeWorkspace, WorkspaceID: workspaceID,
				Caller: participation.OwnerRef{
					Kind: participation.OwnerKindSession, ID: parentID, WorkspaceID: workspaceID,
				},
				Target: callspkg.Target{SessionID: childID}, Prompt: "follow up",
				Actor: callspkg.Actor{Kind: "agent_session", ID: parentID},
			})
			createResult <- createErr
		}()
		go func() {
			<-start
			won, fenceErr := database.FenceSessionReap(ctx, childID, now)
			fenceResult <- struct {
				won bool
				err error
			}{won: won, err: fenceErr}
		}()
		close(start)
		createErr := <-createResult
		fence := <-fenceResult
		if fence.err != nil {
			t.Fatalf("FenceSessionReap(%d) error = %v", iteration, fence.err)
		}
		switch {
		case createErr == nil && !fence.won:
		case callspkg.IsCode(createErr, callspkg.CodeTargetExpired) && fence.won:
		default:
			t.Fatalf("race %d = create %v fence %v, want exactly one winner", iteration, createErr, fence.won)
		}
	}
}

type callIntegrationDirectory struct{ database *GlobalDB }

type callIntegrationDirectoryFunc func(
	context.Context,
	callspkg.CreateInput,
) (callspkg.TargetContext, []callspkg.AgentRosterEntry, error)

func (f callIntegrationDirectoryFunc) ResolveCallTarget(
	ctx context.Context,
	input callspkg.CreateInput,
) (callspkg.TargetContext, []callspkg.AgentRosterEntry, error) {
	return f(ctx, input)
}

func (d callIntegrationDirectory) ResolveCallTarget(
	ctx context.Context,
	input callspkg.CreateInput,
) (callspkg.TargetContext, []callspkg.AgentRosterEntry, error) {
	target, err := d.database.ResolveCallTargetContext(ctx, input)
	if err != nil {
		return callspkg.TargetContext{}, nil, err
	}
	roster := []callspkg.AgentRosterEntry{{Name: "reviewer", Description: "Reviews changes"}}
	if input.Target.Agent == "reviewer" {
		target.AgentName = "reviewer"
	}
	return target, roster, nil
}

type callIntegrationInvoker struct {
	mu              sync.Mutex
	database        *GlobalDB
	workspaceID     string
	now             time.Time
	deliveryOutcome callspkg.DeliveryOutcome
	spawns          int
	deliveries      []callspkg.Delivery
}

func (i *callIntegrationInvoker) SpawnChild(
	ctx context.Context,
	spec callspkg.ChildSpec,
) (callspkg.SessionRef, error) {
	i.mu.Lock()
	i.spawns++
	childID := "ses_calls_child_" + strings.TrimPrefix(spec.CallID, "call_")
	i.mu.Unlock()
	err := i.database.RegisterSession(ctx, store.SessionInfo{
		ID: childID, ProfileID: store.DefaultProfileID, AgentName: spec.AgentName,
		WorkspaceID: i.workspaceID, State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &store.SessionLineage{
			ParentSessionID: spec.ParentSessionID, RootSessionID: spec.ParentSessionID,
			SpawnDepth: 1, SpawnRole: "worker", AutoStopOnParent: true,
			SpawnBudget:      store.SessionSpawnBudget{MaxChildren: 5, MaxDepth: 3},
			PermissionPolicy: spec.Permissions.Policy(),
		},
		CreatedAt: i.now, UpdatedAt: i.now,
	})
	if err != nil {
		return callspkg.SessionRef{}, err
	}
	return callspkg.SessionRef{ID: childID}, nil
}

func (i *callIntegrationInvoker) Revive(context.Context, string, string, string) error { return nil }

func (i *callIntegrationInvoker) DeliverAtBoundary(
	ctx context.Context,
	delivery callspkg.Delivery,
) (callspkg.DeliveryOutcome, error) {
	i.mu.Lock()
	i.deliveries = append(i.deliveries, delivery)
	outcome := i.deliveryOutcome
	i.mu.Unlock()
	if outcome.State == "woken" {
		if _, err := i.database.db.ExecContext(
			ctx,
			`UPDATE sessions SET state = 'active' WHERE id = ?`,
			delivery.RecipientSessionID,
		); err != nil {
			return callspkg.DeliveryOutcome{}, err
		}
	}
	if outcome.State == "" {
		outcome.State = "pending"
	}
	return outcome, nil
}
func (i *callIntegrationInvoker) StopManaged(ctx context.Context, sessionID string, _ string) error {
	_, err := i.database.db.ExecContext(ctx, `UPDATE sessions SET state = 'stopped' WHERE id = ?`, sessionID)
	return err
}
func (i *callIntegrationInvoker) spawnCount() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.spawns
}

func (i *callIntegrationInvoker) recordedDeliveries() []callspkg.Delivery {
	i.mu.Lock()
	defer i.mu.Unlock()
	return append([]callspkg.Delivery(nil), i.deliveries...)
}

func (i *callIntegrationInvoker) setDeliveryOutcome(outcome callspkg.DeliveryOutcome) {
	i.mu.Lock()
	i.deliveryOutcome = outcome
	i.mu.Unlock()
}

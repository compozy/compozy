package globaldb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

func budgetBoundSession(t *testing.T, db *GlobalDB, at time.Time) string {
	t.Helper()
	ctx := t.Context()
	sessionID := registerInputQueueSession(t, db)
	stamp := store.FormatTimestamp(at)
	if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
		id, profile_id, agent_name, workspace_id, state,
		created_by_kind, created_by_id, created_at, updated_at
	) VALUES ('daily-dispatch-instance', ?, 'coder', '', 'active', 'human', 'operator', ?, ?)`,
		store.DefaultProfileID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx,
		`UPDATE sessions SET instance_id = 'daily-dispatch-instance' WHERE id = ?`, sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx,
		`INSERT INTO agent_instance_wake_state (instance_id) VALUES ('daily-dispatch-instance')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Instances().SetDailyBudgetLimits(ctx, "daily-dispatch-instance",
		agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts (
		profile_id, session_id, turn_id, instance_id, day, total_tokens, cost_status, created_at
	) VALUES (?, ?, 'already-spent', 'daily-dispatch-instance', ?, 100, 'unknown', ?)`,
		store.DefaultProfileID, sessionID, store.LocalDay(at), stamp); err != nil {
		t.Fatal(err)
	}
	return sessionID
}

func TestDailyBudgetQueueClaimKeepsAutonomousInputQueued(t *testing.T) {
	t.Run("Should retain exhausted autonomous queue entries", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		sessionID := budgetBoundSession(t, db, at)
		entry, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "budget-waiting-input", SessionID: sessionID, Text: "continue working",
			OriginClass: store.SessionInputOriginClassSynthetic, QueueCap: 10, Now: at,
		})
		if err != nil {
			t.Fatal(err)
		}
		claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, at)
		if err != nil || ok || claimed.PausedInstance == nil ||
			claimed.PausedInstance.Reason != "daily_budget" {
			t.Fatalf("budgeted claim: entry=%+v, ok=%t, err=%v", claimed, ok, err)
		}
		queued, err := db.GetSessionInputQueueEntry(ctx, sessionID, entry.ID)
		if err != nil || queued.Status != store.SessionInputQueueStatusQueued || queued.AttemptCount != 0 {
			t.Fatalf("held queue: entry=%+v, err=%v", queued, err)
		}
		instance, err := db.Instances().Get(ctx, "daily-dispatch-instance")
		if err != nil || instance.State != "paused" || instance.PauseReason != "daily_budget" {
			t.Fatalf("durable pause: instance=%+v, err=%v", instance, err)
		}
	})
	t.Run("Should release a directly bound queue independently of its paused root", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		parent := budgetBoundSession(t, db, at)
		if allowed, _, err := db.Instances().AdmitDailyBudgetTurn(ctx, parent); err != nil || allowed {
			t.Fatalf("parent at cap = %t, %v; want blocked", allowed, err)
		}
		stamp := store.FormatTimestamp(at)
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instances (
			id, profile_id, agent_name, workspace_id, state, created_by_kind, created_by_id, created_at, updated_at
		) VALUES ('independent-instance', ?, 'reviewer', '', 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		const child = "independent-session"
		if _, err := db.DB().ExecContext(ctx, `INSERT INTO sessions (
			id, profile_id, scope, agent_name, workspace_id, session_type, state,
			parent_session_id, root_session_id, instance_id, lineage_kind, created_at, updated_at
		) VALUES (?, ?, 'global', 'reviewer', '', 'user', 'active', ?, ?, 'independent-instance', 'spawn', ?, ?)`,
			child, store.DefaultProfileID, parent, parent, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "independent-input", SessionID: child, Text: "resume work",
			OriginClass: store.SessionInputOriginClassSynthetic, QueueCap: 2, Now: at,
		}); err != nil {
			t.Fatal(err)
		}
		if ids, err := db.Instances().QueuedSessions(ctx, "independent-instance", ""); err != nil ||
			len(ids) != 1 || ids[0] != child {
			t.Fatalf("independent resumed queues = %v, %v; want %s", ids, err, child)
		}
	})
}

func TestDailyBudgetDirectDispatchKeepsAdmissionReserved(t *testing.T) {
	t.Run("Should fence direct dispatch at the budget cap", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		sessionID := budgetBoundSession(t, db, at)
		workspaceID := "ws-input-queue-workspace"
		request := promptAdmissionRequest(workspaceID, sessionID, "budget-direct", at)
		admission, _, err := db.ClaimSessionPromptAdmission(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		err = db.CommitSessionPromptDispatch(ctx, workspaceID, sessionID, request.IdempotencyKey, at)
		if err == nil {
			t.Fatal("autonomous over-budget dispatch was committed")
		}
		if !errors.Is(err, store.ErrDailyBudgetTurnBlocked) {
			t.Fatalf("dispatch error = %v, want daily budget rejection", err)
		}
		current, found, err := db.LookupSessionPromptAdmission(ctx, workspaceID, sessionID, request.IdempotencyKey)
		if err != nil || !found || current.State != store.SessionPromptAdmissionReserved ||
			current.ID != admission.ID {
			t.Fatalf("unchanged admission: %+v found=%t err=%v", current, found, err)
		}
	})
}

func TestDailyBudgetAppCredentialDoesNotBypassDispatch(t *testing.T) {
	t.Run("Should hold direct and queued input authenticated with an application token", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		sessionID := budgetBoundSession(t, db, at)
		ctx := t.Context()
		request := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "budget-app", at)
		request.HumanIngress, request.AppIngress = true, true
		if _, _, err := db.ClaimSessionPromptAdmission(ctx, request); err != nil {
			t.Fatal(err)
		}
		if err := db.CommitSessionPromptDispatch(ctx, request.WorkspaceID,
			sessionID, request.IdempotencyKey, at); !errors.Is(err, store.ErrDailyBudgetTurnBlocked) {
			t.Fatalf("direct app dispatch = %v, want budget rejection", err)
		}
		entry, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:           "daily-budget-app-input",
			SessionID:    sessionID,
			Text:         "application request",
			HumanIngress: true,
			AppIngress:   true,
			OriginClass:  store.SessionInputOriginClassHuman,
			QueueCap:     10,
			Now:          at,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok, err := db.ClaimNextSessionInput(ctx, sessionID, at); err != nil || ok {
			t.Fatalf("queued app dispatch ok=%t, err=%v", ok, err)
		}
		queued, err := db.GetSessionInputQueueEntry(ctx, sessionID, entry.ID)
		if err != nil || queued.Status != store.SessionInputQueueStatusQueued || !queued.AppIngress {
			t.Fatalf("queued app input = %+v, err=%v", queued, err)
		}
	})
}

func TestDailyBudgetAppRetryCannotInheritOperatorAdmission(t *testing.T) {
	t.Run(
		"Should preserve a human reservation without granting an application its budget exemption",
		func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
			db.now = func() time.Time { return at }
			sessionID := budgetBoundSession(t, db, at)
			ctx := t.Context()
			human := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "budget-replay", at)
			human.HumanIngress = true
			admission, created, err := db.ClaimSessionPromptAdmission(ctx, human)
			if err != nil || !created {
				t.Fatalf("human reservation = %+v, created=%t, err=%v", admission, created, err)
			}
			app := human
			app.AppIngress = true
			if _, _, err := db.ReplaySessionPromptAdmission(
				ctx,
				app,
			); !errors.Is(
				err,
				store.ErrSessionPromptIdempotencyConflict,
			) {
				t.Fatalf("app replay error=%v, want idempotency conflict", err)
			}
			if _, _, err := db.ClaimSessionPromptAdmission(
				ctx,
				app,
			); !errors.Is(
				err,
				store.ErrSessionPromptIdempotencyConflict,
			) {
				t.Fatalf("app claim error=%v, want idempotency conflict", err)
			}
			replayed, found, err := db.ReplaySessionPromptAdmission(ctx, human)
			if err != nil || !found || replayed.ID != admission.ID ||
				replayed.State != store.SessionPromptAdmissionReserved {
				t.Fatalf("operator reservation changed: %+v, found=%t, err=%v", replayed, found, err)
			}
			if err := db.CommitSessionPromptDispatch(
				ctx,
				human.WorkspaceID,
				sessionID,
				human.IdempotencyKey,
				at,
			); err != nil {
				t.Fatalf("operator dispatch refused after app retry: %v", err)
			}
		},
	)
}

func TestDailyBudgetHumanDispatchRemainsAvailable(t *testing.T) {
	t.Run("Should allow human dispatch while autonomous turns are held", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		sessionID := budgetBoundSession(t, db, at)
		ctx := t.Context()
		entry, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "human-budget-input", SessionID: sessionID, Text: "operator override",
			HumanIngress: true, OriginClass: store.SessionInputOriginClassHuman, QueueCap: 10, Now: at,
		})
		if err != nil {
			t.Fatal(err)
		}
		claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, at)
		if err != nil || !ok || claimed.ID != entry.ID {
			t.Fatalf("human queue claim: %+v ok=%t err=%v", claimed, ok, err)
		}
		request := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "budget-human", at)
		request.HumanIngress = true
		if _, _, err := db.ClaimSessionPromptAdmission(ctx, request); err != nil {
			t.Fatal(err)
		}
		if err := db.CommitSessionPromptDispatch(ctx, request.WorkspaceID,
			sessionID, request.IdempotencyKey, at); err != nil {
			t.Fatalf("human direct dispatch refused: %v", err)
		}
		instance, err := db.Instances().Get(ctx, "daily-dispatch-instance")
		if err != nil || instance.State != "active" {
			t.Fatalf("human input paused instance: %+v err=%v", instance, err)
		}
	})
}

func TestDailyBudgetNonhumanQueueInputCannotClaimHumanPriority(t *testing.T) {
	t.Run("Should prevent autonomous queue input from claiming human priority", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		sessionID := budgetBoundSession(t, db, at)
		ctx := t.Context()
		entry, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "extension-without-origin", SessionID: sessionID, Text: "extension prompt",
			// Legacy/default queue origin classification is priority, not human authentication.
			OriginClass: store.SessionInputOriginClassHuman, HumanIngress: false, QueueCap: 10, Now: at,
		})
		if err != nil {
			t.Fatal(err)
		}
		claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, at)
		if err != nil || ok || claimed.PausedInstance == nil {
			t.Fatalf("unauthenticated human-priority claim: %+v ok=%t err=%v", claimed, ok, err)
		}
		queued, err := db.GetSessionInputQueueEntry(ctx, sessionID, entry.ID)
		if err != nil || queued.Status != store.SessionInputQueueStatusQueued {
			t.Fatalf("extension budget bypass: %+v err=%v", queued, err)
		}
	})
}

func TestDailyBudgetPausedQueuePrioritizesAuthenticatedOperator(t *testing.T) {
	t.Run("Should prioritize authenticated operator input", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		sessionID := budgetBoundSession(t, db, at)
		ctx := t.Context()
		for _, req := range []store.SessionInputQueueInsert{
			{ID: "aaa-agent-with-human-priority", SessionID: sessionID, Text: "self-sent agent",
				OriginClass: store.SessionInputOriginClassHuman, QueueCap: 10, Now: at},
			{ID: "zzz-real-operator", SessionID: sessionID, Text: "operator action",
				HumanIngress: true, OriginClass: store.SessionInputOriginClassHuman,
				QueueCap: 10, Now: at.Add(time.Minute)},
		} {
			if _, _, err := db.EnqueueSessionInput(ctx, req); err != nil {
				t.Fatal(err)
			}
		}
		if _, ok, err := db.ClaimNextSessionInput(ctx, sessionID, at); err != nil || ok {
			t.Fatalf("budget must initially hold agent work: ok=%t err=%v", ok, err)
		}
		selected, ok, err := db.PeekNextSessionInput(ctx, sessionID)
		if err != nil || !ok || selected.ID != "zzz-real-operator" {
			t.Fatalf("budget-paused peek: %+v ok=%t err=%v", selected, ok, err)
		}
		claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, at.Add(time.Minute))
		if err != nil || !ok || claimed.ID != "zzz-real-operator" {
			t.Fatalf("budget-paused claim: %+v ok=%t err=%v", claimed, ok, err)
		}
		held, err := db.GetSessionInputQueueEntry(ctx, sessionID, "aaa-agent-with-human-priority")
		if err != nil || held.Status != store.SessionInputQueueStatusQueued || held.AttemptCount != 0 {
			t.Fatalf("agent queue was consumed: %+v err=%v", held, err)
		}
	})
}

func TestDailyBudgetQueuedInputResumesOnlyAfterOperatorAndRollover(t *testing.T) {
	t.Run("Should resume held work only after operator action and day rollover", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		sessionID := budgetBoundSession(t, db, at)
		ctx := t.Context()
		entry, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "budget-rollover-input", SessionID: sessionID, Text: "keep waiting",
			OriginClass: store.SessionInputOriginClassSynthetic, QueueCap: 10, Now: at,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok, err := db.ClaimNextSessionInput(ctx, sessionID, at); err != nil || ok {
			t.Fatalf("exhausted claim: ok=%t err=%v", ok, err)
		}
		at = at.AddDate(0, 0, 1)
		if _, ok, err := db.ClaimNextSessionInput(ctx, sessionID, at); err != nil || ok {
			t.Fatalf("rollover auto-resumed: ok=%t err=%v", ok, err)
		}
		if _, err := db.Instances().Resume(ctx, "daily-dispatch-instance", 10, true); err != nil {
			t.Fatalf("operator resume after rollover: %v", err)
		}
		claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, at)
		if err != nil || !ok || claimed.ID != entry.ID || claimed.AttemptCount != 1 {
			t.Fatalf("recovered pending input: %+v ok=%t err=%v", claimed, ok, err)
		}
	})
}

func TestDailyBudgetPausedNestedInstanceUsesNearestOwnerForQueueEligibility(t *testing.T) {
	t.Run("Should charge nested queue work to its closest instance", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
		db.now = func() time.Time { return at }
		rootID := budgetBoundSession(t, db, at)
		ctx := t.Context()
		childID := "budget-nested-child"
		stamp := store.FormatTimestamp(at)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instances (
		id, profile_id, agent_name, workspace_id, state,
		created_by_kind, created_by_id, created_at, updated_at
	) VALUES ('daily-nested-owner', ?, 'nested-coder', '', 'active', 'human', 'operator', ?, ?)`,
			store.DefaultProfileID, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO agent_instance_wake_state (instance_id)
		VALUES ('daily-nested-owner')`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions
		(id, profile_id, scope, agent_name, workspace_id, session_type, state,
		 parent_session_id, root_session_id, lineage_kind, instance_id, created_at, updated_at)
		SELECT ?, profile_id, scope, 'nested-coder', workspace_id, session_type, state,
		id, id, 'spawn', 'daily-nested-owner', ?, ?
		FROM sessions WHERE id = ?`, childID, stamp, stamp, rootID); err != nil {
			t.Fatal(err)
		}
		if err := db.Instances().SetDailyBudgetLimits(ctx, "daily-nested-owner",
			agentinstance.DailyBudgetLimits{Tokens: new(int64(100))}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `INSERT INTO token_usage_receipts
		(profile_id, session_id, turn_id, instance_id, day, total_tokens, cost_status, created_at)
		VALUES (?, ?, 'nested-spend', 'daily-nested-owner', ?, 100, 'unknown', ?)`,
			store.DefaultProfileID, childID, store.LocalDay(at), stamp); err != nil {
			t.Fatal(err)
		}
		for _, request := range []store.SessionInputQueueInsert{
			{ID: "nested-agent-first", SessionID: childID, Text: "autonomous",
				OriginClass: store.SessionInputOriginClassHuman, QueueCap: 10, Now: at},
			{ID: "nested-operator-next", SessionID: childID, Text: "human",
				HumanIngress: true, OriginClass: store.SessionInputOriginClassHuman,
				QueueCap: 10, Now: at.Add(time.Minute)},
		} {
			if _, _, err := db.EnqueueSessionInput(ctx, request); err != nil {
				t.Fatal(err)
			}
		}
		if _, ok, err := db.ClaimNextSessionInput(ctx, childID, at); err != nil || ok {
			t.Fatalf("first nested autonomous claim: ok=%t err=%v", ok, err)
		}
		owner, err := db.Instances().Get(ctx, "daily-nested-owner")
		if err != nil || owner.State != "paused" || owner.PauseReason != "daily_budget" {
			t.Fatalf("nested instance was not paused: %+v err=%v", owner, err)
		}
		selected, ok, err := db.PeekNextSessionInput(ctx, childID)
		if err != nil || !ok || selected.ID != "nested-operator-next" {
			t.Fatalf("nested paused peek=%+v ok=%t err=%v", selected, ok, err)
		}
		selected, ok, err = db.ClaimNextSessionInput(ctx, childID, at.Add(time.Minute))
		if err != nil || !ok || selected.ID != "nested-operator-next" {
			t.Fatalf("nested human claim=%+v ok=%t err=%v", selected, ok, err)
		}
	})
}

func TestDailyBudgetDefinitionPausedDescendantLatchesUntilOperatorResume(t *testing.T) {
	for _, reason := range []string{"definition_missing", "definition_not_always_on"} {
		t.Run("Should preserve the original reason while latching spend under a "+reason+" pause", func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			at := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
			db.now = func() time.Time { return at }
			main := budgetBoundSession(t, db, at)
			child := "budget-definition-paused-child"
			ctx := t.Context()
			stamp := store.FormatTimestamp(at)
			if _, err := db.db.ExecContext(ctx, `INSERT INTO sessions
			(id, profile_id, scope, agent_name, workspace_id, session_type, state,
			 parent_session_id, root_session_id, lineage_kind, created_at, updated_at)
			SELECT ?, profile_id, scope, agent_name, workspace_id, session_type, state,
			 id, id, 'spawn', ?, ? FROM sessions WHERE id=?`, child, stamp, stamp, main); err != nil {
				t.Fatal(err)
			}
			if _, err := db.db.ExecContext(ctx,
				`UPDATE agent_instances SET state='paused', pause_reason=? WHERE id='daily-dispatch-instance'`,
				reason); err != nil {
				t.Fatal(err)
			}
			allowed, trip, err := db.Instances().AdmitDailyBudgetTurn(ctx, child)
			if err != nil || allowed || trip != nil {
				t.Fatalf(
					"exhausted child allowed=%t trip=%+v err=%v, want refusal without a second pause",
					allowed,
					trip,
					err,
				)
			}
			paused, err := db.Instances().Get(ctx, "daily-dispatch-instance")
			if err != nil || paused.PauseReason != reason {
				t.Fatalf("existing pause=%q err=%v, want %q", paused.PauseReason, err, reason)
			}
			at = at.AddDate(0, 0, 1)
			allowed, _, err = db.Instances().AdmitDailyBudgetTurn(ctx, child)
			if err != nil || allowed {
				t.Fatalf("next-day descendant allowed=%t err=%v before operator resume", allowed, err)
			}
			if _, err := db.Instances().
				Resume(ctx, "daily-dispatch-instance", 10, false); !errors.Is(
				err,
				agentinstance.ErrForbidden,
			) {
				t.Fatalf("agent resume of budget latch=%v, want forbidden", err)
			}
		})
	}
}

package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
)

// Invariant: delegation validates its semantic request before any effect; owner: session service.
func TestSubagentRequest(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, task string
		valid      bool
	}{
		{"Should reject whitespace UT-006", " \n", false},
		{"Should accept 120000 runes UT-007", strings.Repeat("界", 120000), true},
		{"Should reject 120001 runes UT-007", strings.Repeat("界", 120001), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := subagentTestRequest()
			req.Task = tc.task
			_, err := normalizeSubagentRequest(req)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
	t.Run("Should derive full stored title and reject explicit overflow UT-008", func(t *testing.T) {
		t.Parallel()
		req := subagentTestRequest()
		req.Title = ""
		req.Task = strings.Repeat("x", 100) + "\nrest"
		normalized, err := normalizeSubagentRequest(req)
		if err != nil || normalized.Title != strings.Repeat("x", 100) {
			t.Fatalf("%+v %v", normalized, err)
		}
		req.Title = strings.Repeat("x", 513)
		if _, err := normalizeSubagentRequest(req); !errors.Is(err, ErrSubagentInvalidRequest) {
			t.Fatal(err)
		}
	})
	t.Run("Should prefix only non-general roles UT-009", func(t *testing.T) {
		t.Parallel()
		if subagentPrompt("review", "task") != "Act as the review subagent for this task.\n\ntask" ||
			subagentPrompt("general", "task") != "task" {
			t.Fatal("role prompt mismatch")
		}
	})
	for _, tc := range []struct {
		name        string
		input, want time.Duration
		mode        string
	}{{"Should default wait UT-010", 0, 10 * time.Minute, "wait"}, {"Should clamp short wait UT-010", 10 * time.Millisecond, time.Second, "wait"}, {"Should clamp long wait UT-010", 5000 * time.Second, time.Hour, "wait"}, {"Should ignore async timeout UT-010", time.Hour, 0, "async"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := subagentTestRequest()
			req.Mode = tc.mode
			req.Timeout = tc.input
			got, err := normalizeSubagentRequest(req)
			if err != nil || got.Timeout != tc.want {
				t.Fatalf("%v %v", got.Timeout, err)
			}
		})
	}
	t.Run("Should hash parent and default tool identity UT-015", func(t *testing.T) {
		t.Parallel()
		sum := sha256.Sum256([]byte("parentcall"))
		req, err := normalizeSubagentRequest(subagentTestRequest())
		if err != nil || req.IdempotencyKey != "call" ||
			subagentID("parent", req.IdempotencyKey) != "sub-"+hex.EncodeToString(sum[:8]) {
			t.Fatal("unstable id", err)
		}
	})
}
func TestSubagentTargetAndPermissions(t *testing.T) {
	t.Parallel()
	t.Run("Should default omitted runtime speed to normal", func(t *testing.T) {
		t.Parallel()
		target := resolveSubagentTarget(SubagentTarget{}, SubagentTarget{}, SubagentTarget{})
		row := presentSubagent(store.SessionSubagent{Origin: store.SubagentOriginDelegated})
		if target.Speed != "normal" || row.RuntimeSpeed != "normal" {
			t.Fatal(target, row)
		}
	})
	t.Run("Should inherit only matching provider runtime UT-001 UT-002", func(t *testing.T) {
		t.Parallel()
		in := SubagentTarget{
			Agent:           "claude",
			Provider:        "claude",
			Model:           "opus",
			ReasoningEffort: "xhigh",
			Speed:           "normal",
		}
		got := resolveSubagentTarget(in, SubagentTarget{}, SubagentTarget{})
		if got.Model != "opus" || got.ReasoningEffort != "xhigh" || got.Agent != "claude" {
			t.Fatal(got)
		}
		got = resolveSubagentTarget(in, SubagentTarget{Provider: "codex"}, SubagentTarget{Model: "sol", Speed: "fast"})
		if got.Model != "sol" || got.ReasoningEffort != "" || got.Speed != "fast" {
			t.Fatal(got)
		}
	})
	t.Run("Should enforce permission mode ranks UT-011", func(t *testing.T) {
		t.Parallel()
		info := &Info{EffectivePermissions: "approve-reads"}
		req := subagentTestRequest()
		req.PermissionMode = compozyconfig.PermissionModeApproveAll
		if _, _, err := subagentPermissions(info, req); !errors.Is(err, ErrSpawnPermissionDenied) {
			t.Fatal(err)
		}
		req.PermissionMode = "inherit"
		mode, _, err := subagentPermissions(info, req)
		if err != nil || mode != "approve-reads" {
			t.Fatal(mode, err)
		}
		req.PermissionMode = "deny-all"
		if _, _, err := subagentPermissions(info, req); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should distinguish omitted and empty atoms UT-012", func(t *testing.T) {
		t.Parallel()
		info := &Info{
			EffectivePermissions: "approve-reads",
			Lineage: &store.SessionLineage{
				PermissionPolicy: store.SessionPermissionPolicy{
					Tools:          []string{"A", "B"},
					Skills:         []string{"A"},
					MCPServers:     []string{"A"},
					WorkspacePaths: []string{"A"},
				},
			},
		}
		req := subagentTestRequest()
		_, policy, err := subagentPermissions(info, req)
		if err != nil || len(policy.Tools) != 2 {
			t.Fatal(policy, err)
		}
		req.Narrowing = &SubagentPermissionNarrowing{
			Tools:          []string{},
			Skills:         []string{},
			MCPServers:     []string{},
			WorkspacePaths: []string{},
		}
		_, policy, err = subagentPermissions(info, req)
		if err != nil || len(policy.Tools) != 0 || len(policy.Skills) != 0 || len(policy.MCPServers) != 0 ||
			len(policy.WorkspacePaths) != 0 {
			t.Fatal(policy, err)
		}
		for _, n := range []*SubagentPermissionNarrowing{{Tools: []string{"C"}}, {Skills: []string{"C"}}, {MCPServers: []string{"C"}}, {WorkspacePaths: []string{"C"}}} {
			req.Narrowing = n
			if _, _, err := subagentPermissions(info, req); err == nil || !strings.Contains(err.Error(), "C") {
				t.Fatal(err)
			}
		}
	})
}

// Invariant: one semantic request owns one child, failure cleanup, and one terminal event; owner: service.
func TestSubagentDelegate(t *testing.T) {
	t.Parallel()
	t.Run("Should replay without another child and reject changed task UT-016 UT-059", func(t *testing.T) {
		t.Parallel()
		s, _, runtime := newSubagentTestService(t)
		req := subagentTestRequest()
		first := requireSubagent(t, s, req)
		second := requireSubagent(t, s, req)
		if first.ID != second.ID || *first.ChildSessionID != *second.ChildSessionID || len(runtime.spawned) != 1 {
			t.Fatal(first, second)
		}
		req.Task = "different"
		if _, err := s.Delegate(t.Context(), req); !errors.Is(err, ErrSubagentInvalidRequest) {
			t.Fatal(err)
		}
	})
	t.Run("Should retain failed reservation on spawn error UT-059", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		runtime.spawnErr = testSubagentError()
		row := requireSubagent(t, s, subagentTestRequest())
		if row.Status != "failed" || row.Error == nil || len(db.rows) != 1 {
			t.Fatal(row)
		}
	})
	t.Run("Should stop child after failed first admission UT-059", func(t *testing.T) {
		t.Parallel()
		s, _, runtime := newSubagentTestService(t)
		runtime.errorAdmit = testSubagentError()
		row := requireSubagent(t, s, subagentTestRequest())
		if row.Status != "failed" || len(runtime.stopped) != 1 || len(runtime.queues) != 0 {
			t.Fatal(row, runtime.stopped)
		}
	})
	t.Run("Should delete reservation on hook denial IT-022 service", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		runtime.spawnErr = ErrSubagentCapabilityDenied
		if _, err := s.Delegate(
			t.Context(),
			subagentTestRequest(),
		); !errors.Is(err, ErrSubagentCapabilityDenied) ||
			len(db.rows) != 0 {
			t.Fatal(err, db.rows)
		}
	})
	t.Run("Should support recursive depth and no live cap UT-013 UT-014", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		db.rows["ancestor"] = store.SessionSubagent{
			ID:              "ancestor",
			ChildSessionID:  new("parent"),
			Depth:           5,
			ParentSessionID: "root",
			Status:          "running",
		}
		for i := range 21 {
			req := subagentTestRequest()
			req.IdempotencyKey = string(rune('a' + i))
			row := requireSubagent(t, s, req)
			if row.Depth != 6 {
				t.Fatal(row.Depth)
			}
		}
		for _, opts := range runtime.spawned {
			if opts.TTL != 0 || opts.NotifyCreator || !opts.AutoStopOnParent || opts.SpawnRole != "subagent" {
				t.Fatal(opts)
			}
		}
	})
	t.Run("Should bound queued replay by completion channel UT-059", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, _, runtime := newSubagentTestService(t)
			runtime.spawnBlock = make(chan struct{})
			req := subagentTestRequest()
			done := make(chan error, 1)
			go func() { _, err := s.Delegate(t.Context(), req); done <- err }()
			synctest.Wait()
			replayed := make(chan Subagent, 1)
			go func() {
				row, err := s.Delegate(t.Context(), req)
				if err != nil {
					done <- err
				}
				replayed <- row
			}()
			synctest.Wait()
			select {
			case <-replayed:
				t.Fatal("replay returned while queued")
			default:
			}
			close(runtime.spawnBlock)
			synctest.Wait()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			row := <-replayed
			if row.Status != "running" || len(runtime.spawned) != 1 {
				t.Fatal(row)
			}
		})
	})
	t.Run("Should return wait timeout and upgrade policy UT-030", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, _ := newSubagentTestService(t)
			req := subagentTestRequest()
			req.Mode = "wait"
			req.Timeout = time.Second
			row := requireSubagent(t, s, req)
			if !row.WaitTimedOut || db.rows[row.ID].WakePolicy != "always" {
				t.Fatal(row)
			}
		})
	})
}
func TestSubagentDeliveryPlanner(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		active       bool
		queued, live int
		want         string
	}{{"Should keep active work UT-018", true, 0, 0, "working"}, {"Should keep queued work UT-018", false, 1, 0, "working"}, {"Should wait for descendants UT-018", false, 0, 1, "waiting_for_children"}, {"Should expose settled result UT-018", false, 0, 0, "result_available"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := deriveSubagentWorkState(tc.active, tc.queued, tc.live); got != tc.want {
				t.Fatal(got)
			}
		})
	}
	t.Run("Should apply wake policy and disposition UT-022 UT-024 UT-025 UT-026", func(t *testing.T) {
		t.Parallel()
		row := store.SessionSubagent{WakePolicy: "always", ParentTurnID: "turn", Delivery: "none"}
		parent := subagentSnapshot{Info: &Info{State: StateActive}, TurnID: "turn", Active: true}
		if planSubagentDelivery(row, parent, false) != "claim" || planSubagentDelivery(row, parent, true) != "pending" {
			t.Fatal("async route")
		}
		row.WakePolicy = "settled_only"
		if planSubagentDelivery(row, parent, false) != "none" {
			t.Fatal("wait route")
		}
		parent.Active = false
		if planSubagentDelivery(row, parent, false) != "claim" {
			t.Fatal("settled wait")
		}
		parent.Info.State = StateStopped
		if planSubagentDelivery(row, parent, false) != "dispose" {
			t.Fatal("stop route")
		}
		row.Delivery = "disposed"
		if planSubagentDelivery(row, parent, false) != "none" {
			t.Fatal("final delivery changed")
		}
	})
	t.Run("Should steer only eligible batches UT-027 UT-057", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionSubagent{{WakePolicy: "always"}}
		parent := subagentSnapshot{Active: true, CanSteer: true}
		if !canSteerSubagentWake(parent, rows) {
			t.Fatal("missing steer")
		}
		parent.UserSteer = true
		if canSteerSubagentWake(parent, rows) {
			t.Fatal("displaced user")
		}
		parent.UserSteer = false
		rows[0].WakePolicy = "settled_only"
		if canSteerSubagentWake(parent, rows) {
			t.Fatal("wait steer")
		}
		rows[0].WakePolicy = "always"
		parent.CanSteer = false
		if canSteerSubagentWake(parent, rows) {
			t.Fatal("unsupported steer")
		}
	})
	t.Run("Should format safe mailbox pointers UT-028", func(t *testing.T) {
		t.Parallel()
		rows := []store.SessionSubagent{{ID: "sub-x", Title: "T", Status: "completed"}}
		if got := subagentWakeText(
			rows,
		); got != "Subagent \"T\" (sub-x) finished: completed.\nCall compozy__subagent_status to read its result." {
			t.Fatal(got)
		}
		rows = append(rows, store.SessionSubagent{ID: "sub-y", Title: "a\"\nb", Status: "failed"})
		got := subagentWakeText(rows)
		if !strings.Contains(got, "\"a' b\"") || !strings.HasSuffix(got, "read each result.") {
			t.Fatal(got)
		}
	})
}
func TestSubagentLifecycle(t *testing.T) {
	t.Parallel()
	t.Run("Should finalize once and retain result limit UT-020 UT-032 UT-043", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, runtime := newSubagentTestService(t)
			s.resultLimit = func(context.Context, string) (int, error) { return 3, nil }
			hook := &subagentTestHook{}
			s.settled = hook
			row := requireSubagent(t, s, subagentTestRequest())
			runtime.results[*row.ChildSessionID] = "界界界界"
			settleTestChild(t, s, runtime, &row)
			if err := s.OnChildSettled(t.Context(), *row.ChildSessionID); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			got, err := s.Get(t.Context(), "ws", row.ID)
			if err != nil || !got.ResultTruncated || *got.Result != "界界界" ||
				got.Hint != "Read the full answer with compozy__session_history on child_session_id." ||
				hook.calls != 1 ||
				db.rows[row.ID].Delivery != "claimed" {
				t.Fatal(got, err, hook.calls)
			}
		})
	})
	t.Run("Should join queued wakes then claim successor UT-023 UT-024", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		first := requireSubagent(t, s, subagentTestRequest())
		req := subagentTestRequest()
		req.IdempotencyKey = "two"
		second := requireSubagent(t, s, req)
		settleTestChild(t, s, runtime, &first)
		settleTestChild(t, s, runtime, &second)
		wake := *db.rows[first.ID].WakeMessageID
		if *db.rows[second.ID].WakeMessageID != wake || len(db.wakes) != 1 ||
			!strings.Contains(db.rewrites[wake], second.ID) {
			t.Fatal(db.wakes, db.rewrites)
		}
		if err := s.OnWakeDispatched(t.Context(), "parent", wake); err != nil {
			t.Fatal(err)
		}
		req.IdempotencyKey = "three"
		third := requireSubagent(t, s, req)
		settleTestChild(t, s, runtime, &third)
		if db.rows[third.ID].Delivery != "pending" {
			t.Fatal(db.rows[third.ID])
		}
		if err := s.OnWakeTurnSettled(t.Context(), "parent", wake, false); err != nil {
			t.Fatal(err)
		}
		if db.rows[third.ID].Delivery != "claimed" || *db.rows[third.ID].WakeMessageID == wake {
			t.Fatal(db.rows[third.ID])
		}
	})
	t.Run("Should acknowledge owned result and cancel empty wake UT-029 UT-035", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		req := subagentTestRequest()
		row := requireSubagent(t, s, req)
		settleTestChild(t, s, runtime, &row)
		foreign := req.Caller
		foreign.SessionID = "foreign"
		runtime.snapshots["foreign"] = runtime.snapshots["parent"]
		if _, err := s.Status(t.Context(), foreign, row.ID); !errors.Is(err, ErrSubagentNotFound) {
			t.Fatal(err)
		}
		got, err := s.Status(t.Context(), req.Caller, row.ID)
		if err != nil || got.Delivery != "acknowledged" || len(runtime.queues) != 0 {
			t.Fatal(got, err)
		}
		if _, err := s.Status(t.Context(), req.Caller, row.ID); err != nil {
			t.Fatal(err)
		}
		if db.rows[row.ID].AcknowledgedTurnID != "turn" {
			t.Fatal(db.rows[row.ID])
		}
	})
	t.Run("Should dispose interrupted deliveries while child continues UT-026 IT-026 service", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		if err := s.OnParentTurnInterrupted(t.Context(), "parent", "turn"); err != nil {
			t.Fatal(err)
		}
		settleTestChild(t, s, runtime, &row)
		if db.rows[row.ID].Delivery != "disposed" || len(runtime.stopped) != 0 || len(runtime.queues) != 0 {
			t.Fatal(db.rows[row.ID])
		}
	})
	t.Run("Should queue failed steer exactly once UT-032", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		snap := runtime.snapshots["parent"]
		snap.CanSteer = true
		runtime.snapshots["parent"] = snap
		runtime.steer = acp.SteerResult{Attempt: acp.SteerAttemptUnsupported}
		row := requireSubagent(t, s, subagentTestRequest())
		settleTestChild(t, s, runtime, &row)
		wake := *db.rows[row.ID].WakeMessageID
		if !db.wakes[wake].SteerRequeued || len(runtime.queues) != 1 || len(runtime.stopped) != 0 {
			t.Fatal(db.wakes)
		}
		if err := s.OnSteerOutcome(t.Context(), "parent", wake, false); err != nil {
			t.Fatal(err)
		}
		if len(runtime.queues) != 1 {
			t.Fatal(runtime.queues)
		}
	})
	t.Run("Should reject native cancellation UT-034", func(t *testing.T) {
		t.Parallel()
		s, db, _ := newSubagentTestService(t)
		db.rows["native"] = store.SessionSubagent{ID: "native", Origin: "provider_native"}
		if _, err := s.Cancel(
			t.Context(),
			SubagentActor{Kind: "operator"},
			"native",
			"",
		); !errors.Is(
			err,
			ErrSubagentNotCancelable,
		) {
			t.Fatal(err)
		}
	})
	t.Run("Should coalesce progress to latest value UT-060", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, runtime := newSubagentTestService(t)
			row := requireSubagent(t, s, subagentTestRequest())
			before := len(runtime.publishes)
			updates, cancel, err := s.SubscribeSubagentUpdates(t.Context(), "parent")
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			for range 9 {
				s.OnChildActivity(t.Context(), *row.ChildSessionID, "earlier")
			}
			s.OnChildActivity(t.Context(), *row.ChildSessionID, strings.Repeat("x", 400)+"\nignored")
			synctest.Wait()
			time.Sleep(time.Second)
			synctest.Wait()
			update := <-updates
			if update.Subagent.Progress != strings.Repeat("x", 280) {
				t.Fatalf("progress stream=%#v", update)
			}
			if len(runtime.publishes) != before {
				t.Fatal("progress published a parent catalog upsert")
			}
			if db.writes != 1 || db.rows[row.ID].Progress != strings.Repeat("x", 280) {
				t.Fatal(db.writes, db.rows[row.ID].Progress)
			}
		})
	})
}

type subagentTestHook struct{ calls int }

func (h *subagentTestHook) DispatchSubagentSettled(context.Context, store.SessionSubagent) error {
	h.calls++
	return nil
}

// Invariant: boot repairs every interrupted boundary from durable inputs; owner: service recovery (IT-031).
func TestSubagentRecovery(t *testing.T) {
	t.Parallel()
	t.Run("Should recover reservation admission wake and orphan boundaries IT-031 service", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		db.rows["stale"] = store.SessionSubagent{
			ID:              "stale",
			WorkspaceID:     "ws",
			ParentSessionID: "parent",
			Status:          "queued",
			Origin:          "delegated",
			CreatedAt:       s.now().Add(-5 * time.Minute),
		}
		db.rows["running"] = store.SessionSubagent{
			ID:              "running",
			WorkspaceID:     "ws",
			ParentSessionID: "parent",
			Status:          "running",
			Origin:          "delegated",
			ChildSessionID:  new("child"),
			PendingTask:     new("task"),
			Role:            "general",
			Delivery:        "none",
		}
		runtime.snapshots["child"] = subagentSnapshot{
			Info:   &Info{ID: "child", State: StateActive},
			Active: true,
			TurnID: "child-turn",
		}
		db.rows["missing-task"] = store.SessionSubagent{
			ID:              "missing-task",
			WorkspaceID:     "ws",
			ParentSessionID: "parent",
			Status:          "running",
			Origin:          "delegated",
			ChildSessionID:  new("other"),
		}
		runtime.snapshots["other"] = subagentSnapshot{Info: &Info{ID: "other", State: StateActive}, Active: true}
		db.rows["complete"] = store.SessionSubagent{
			ID:              "complete",
			WorkspaceID:     "ws",
			ParentSessionID: "parent",
			Status:          "completed",
			Origin:          "delegated",
			Delivery:        "claimed",
			WakeMessageID:   new("wake-existing"),
		}
		db.wakes["wake-existing"] = store.SessionSubagentWake{
			WakeMessageID:   "wake-existing",
			ParentSessionID: "parent",
			State:           "open",
			Route:           "queue",
		}
		db.orphans = []string{"orphan"}
		if err := s.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		if db.rows["stale"].Status != "failed" || db.rows["missing-task"].Status != "failed" ||
			runtime.admitted["running"] != "task" ||
			db.rows["running"].PendingTask != nil ||
			len(runtime.queues) != 1 {
			t.Fatal(db.rows, runtime.admitted, runtime.queues)
		}
		if len(runtime.stopped) != 2 || runtime.stopped[1] != "orphan" {
			t.Fatal(runtime.stopped)
		}
	})
	t.Run("Should fail fresh reservations from the previous run and wake the parent", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		snap := runtime.snapshots["parent"]
		snap.Active, snap.TurnID = false, ""
		runtime.snapshots["parent"] = snap
		db.rows["fresh"] = store.SessionSubagent{
			ID:              "fresh",
			WorkspaceID:     "ws",
			ParentSessionID: "parent",
			ParentTurnID:    "old-turn",
			Status:          "queued",
			Origin:          "delegated",
			WakePolicy:      store.SubagentWakePolicySettledOnly,
			Delivery:        "none",
			CreatedAt:       s.now().Add(-10 * time.Second),
		}
		if err := s.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		row := db.rows["fresh"]
		if row.Status != "failed" || row.WakePolicy != store.SubagentWakePolicyAlways || row.Delivery != "claimed" ||
			len(runtime.queues) != 1 {
			t.Fatal(row, runtime.queues)
		}
	})
	t.Run(
		"Should publish a committed terminal row and preserve newest subscriber state IT-019 IT-032",
		func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				s, db, runtime := newSubagentTestService(t)
				updates, cancel, err := s.SubscribeSubagentUpdates(t.Context(), "parent")
				if err != nil {
					t.Fatal(err)
				}
				defer cancel()
				row := requireSubagent(t, s, subagentTestRequest())
				settleTestChild(t, s, runtime, &row)
				synctest.Wait()
				observed := false
				for !observed {
					update := <-updates
					persisted, err := db.GetSubagent(t.Context(), "ws", update.Subagent.ID)
					if err != nil {
						t.Fatal(err)
					}
					if update.Subagent.Status == "completed" {
						if persisted.Status != "completed" {
							t.Fatal("published before commit")
						}
						observed = true
					}
				}
				if len(runtime.publishes) == 0 {
					t.Fatal("parent catalog never published")
				}
			})
		},
	)
}

// Invariant: provider errors, waiting states, cancellation and wake outcomes preserve durable meaning.
// Owner: session service; canonical suite: subagent_service_test.go.
func TestSubagentLifecycleBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, status string
		info         Info
		active       bool
	}{
		{"running", "running", Info{State: StateActive}, true},
		{"auth", "waiting", Info{State: StateActive, PendingPermission: true}, true},
		{"input", "waiting", Info{State: StateActive, PendingClarifyCount: 1}, true},
		{"attention", "waiting", Info{State: StateActive, StopVerificationFailed: true}, true},
		{"done", "completed", Info{State: StateActive}, false},
		{"failed", "failed", Info{State: StateStopped, Failure: &store.SessionFailure{Kind: store.FailurePrompt, Summary: "rate_limited: retry after 30s"}}, false},
		{"canceled", "canceled", Info{State: StateStopped, StopReason: store.StopUserCanceled}, false},
		{"lost", "interrupted", Info{State: StateStopped}, false},
	} {
		t.Run("Should map "+tc.name+" UT-021 UT-033", func(t *testing.T) {
			t.Parallel()
			s, db, runtime := newSubagentTestService(t)
			row := requireSubagent(t, s, subagentTestRequest())
			if len(runtime.publishes) != 1 {
				t.Fatalf("creation catalog events=%v", runtime.publishes)
			}
			runtime.snapshots[*row.ChildSessionID] = subagentSnapshot{Info: &tc.info, Active: tc.active}
			runtime.results[*row.ChildSessionID] = "last answer"
			if err := s.OnChildSettled(t.Context(), *row.ChildSessionID); err != nil {
				t.Fatal(err)
			}
			wantPublishes := 2
			if tc.status == store.SubagentStatusRunning {
				wantPublishes = 1
			}
			if len(runtime.publishes) != wantPublishes {
				t.Fatalf("catalog events=%v want=%d", runtime.publishes, wantPublishes)
			}
			got := db.rows[row.ID]
			if got.Status != tc.status {
				t.Fatalf("got %s want %s", got.Status, tc.status)
			}
			if tc.status == "failed" &&
				(got.Error == nil || *got.Error != tc.info.Failure.Summary || got.Result == nil || *got.Result != "last answer") {
				t.Fatal(got)
			}
		})
	}
	t.Run("Should fail a subagent whose settling turn ended in a provider error UT-021", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		// The provider error ended the turn but left the child session alive.
		runtime.snapshots[*row.ChildSessionID] = subagentSnapshot{Info: &Info{State: StateActive}}
		runtime.results[*row.ChildSessionID] = "partial answer"
		runtime.turnErrors[*row.ChildSessionID] = "provider rate limited"
		if err := s.OnChildSettled(t.Context(), *row.ChildSessionID); err != nil {
			t.Fatal(err)
		}
		got := db.rows[row.ID]
		if got.Status != "failed" || got.Error == nil || *got.Error != "provider rate limited" ||
			got.Result == nil || *got.Result != "partial answer" {
			t.Fatal(got)
		}
	})
	t.Run("Should leave canceled queued wake pending until parent settles UT-031", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		settleTestChild(t, s, runtime, &row)
		wake := *db.rows[row.ID].WakeMessageID
		if err := s.OnWakeCanceled(t.Context(), "parent", wake); err != nil {
			t.Fatal(err)
		}
		if db.rows[row.ID].Delivery != "pending" || db.wakes[wake].State != "canceled" {
			t.Fatal(db.rows[row.ID], db.wakes[wake])
		}
		if err := s.OnParentTurnSettled(t.Context(), "parent", "turn"); err != nil {
			t.Fatal(err)
		}
		if db.rows[row.ID].Delivery != "claimed" || *db.rows[row.ID].WakeMessageID == wake {
			t.Fatal(db.rows[row.ID])
		}
	})
	t.Run("Should cancel descendants first and continue after stop errors UT-034 IT-028 service", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, runtime := newSubagentTestService(t)
			row := requireSubagent(t, s, subagentTestRequest())
			db.rows["grandchild"] = store.SessionSubagent{
				ID:              "grandchild",
				ParentSessionID: *row.ChildSessionID,
				ChildSessionID:  new("grandchild-session"),
				Origin:          "delegated",
				Status:          "running",
				Delivery:        "none",
			}
			runtime.errorStop = testSubagentError()
			got, err := s.Cancel(t.Context(), SubagentActor{Kind: "operator"}, row.ID, "")
			synctest.Wait()
			if err != nil || got.Status != "cancel_requested" || db.rows[row.ID].Delivery != "disposed" {
				t.Fatal(got, err)
			}
			if db.rows[row.ID].Status != store.SubagentStatusCanceled ||
				db.rows["grandchild"].Status != store.SubagentStatusCanceled {
				t.Fatal(db.rows)
			}
			if len(runtime.stopped) != 2 || runtime.stopped[0] != "grandchild-session" ||
				runtime.stopped[1] != *row.ChildSessionID {
				t.Fatal(runtime.stopped)
			}
			row.Status = "completed"
			db.rows[row.ID] = row.SessionSubagent
			got, err = s.Cancel(t.Context(), SubagentActor{Kind: "operator"}, row.ID, "")
			if err != nil || got.Status != "completed" || len(runtime.stopped) != 2 {
				t.Fatal(got, err)
			}
		})
	})
	t.Run("Should let an owning agent cancel without an active turn", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, runtime := newSubagentTestService(t)
			row := requireSubagent(t, s, subagentTestRequest())
			snap := runtime.snapshots["parent"]
			snap.Active, snap.TurnID = false, ""
			runtime.snapshots["parent"] = snap
			foreign := SubagentActor{
				Kind:   "agent",
				ID:     "other",
				Caller: &SubagentCaller{WorkspaceID: "ws", SessionID: "other"},
			}
			if _, err := s.Cancel(t.Context(), foreign, row.ID, ""); err == nil {
				t.Fatal("foreign caller canceled the row")
			}
			owner := SubagentActor{
				Kind:   "agent",
				ID:     "parent",
				Caller: &SubagentCaller{WorkspaceID: "ws", SessionID: "parent"},
			}
			got, err := s.Cancel(t.Context(), owner, row.ID, "")
			synctest.Wait()
			if err != nil || got.Status != "cancel_requested" ||
				db.rows[row.ID].Status != store.SubagentStatusCanceled {
				t.Fatal(got, err, db.rows[row.ID])
			}
		})
	})
	t.Run("Should mark injected steer delivered without queue UT-057", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		snap := runtime.snapshots["parent"]
		snap.CanSteer = true
		runtime.snapshots["parent"] = snap
		runtime.steer = acp.SteerResult{Attempt: acp.SteerAttemptInjected}
		row := requireSubagent(t, s, subagentTestRequest())
		settleTestChild(t, s, runtime, &row)
		if db.rows[row.ID].Delivery != "delivered" || len(runtime.queues) != 0 || len(runtime.stopped) != 0 {
			t.Fatal(db.rows[row.ID])
		}
	})
}

func TestSubagentModelValidation(t *testing.T) {
	t.Parallel()
	t.Run("Should bound unavailable models to ten UT-004", func(t *testing.T) {
		t.Parallel()
		options := make([]SubagentModelOption, 12)
		for i := range options {
			options[i].ID = string(rune('a' + i))
		}
		err := validateSubagentModel("gpt-9", "codex", options)
		typed, ok := errors.AsType[*SubagentError](err)
		if !ok || typed.Code != "model_unavailable" ||
			typed.Message != "Model gpt-9 is not available on codex. Available: a, b, c, d, e, f, g, h, i, j." {
			t.Fatal(err)
		}
	})
	t.Run("Should permit default with unavailable catalog UT-005", func(t *testing.T) {
		t.Parallel()
		if err := validateSubagentModel("default", "codex", nil); err != nil {
			t.Fatal(err)
		}
	})
}

// UT-036/UT-037 lifecycle half: native tool identity owns one row and never owns a wake.
func TestSubagentNativeLifecycle(t *testing.T) {
	t.Run("Should settle native work once without a wake", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, runtime := newSubagentTestService(t)
			hook := &subagentTestHook{}
			s.settled = hook
			ev := NativeSubagentEvent{
				WorkspaceID:        "ws",
				ParentTurnID:       "turn",
				ProviderToolCallID: "native-tool",
				ToolName:           "Agent",
				Title:              "Review the diff (high effort)",
				Model:              "sonnet-5.5",
				Status:             "in_progress",
			}
			for range 2 {
				if err := s.OnNativeToolEvent(t.Context(), "parent", ev); err != nil {
					t.Fatal(err)
				}
			}
			id := subagentID("parent", ev.ProviderToolCallID)
			row := db.rows[id]
			if len(db.rows) != 1 || row.Origin != "provider_native" || row.ChildSessionID != nil ||
				row.Title != ev.Title ||
				row.RuntimeModel != ev.Model ||
				row.ProviderToolCallID != ev.ProviderToolCallID {
				t.Fatal(row)
			}
			ev.Title = "Survey BRIEF.md risks"
			if err := s.OnNativeToolEvent(t.Context(), "parent", ev); err != nil {
				t.Fatal(err)
			}
			if db.rows[id].Title != ev.Title {
				t.Fatalf("updated title = %q", db.rows[id].Title)
			}
			ev.Status = "completed"
			ev.Result = "native answer"
			for range 2 {
				if err := s.OnNativeToolEvent(t.Context(), "parent", ev); err != nil {
					t.Fatal(err)
				}
			}
			synctest.Wait()
			row = db.rows[id]
			if row.Status != "completed" || row.Result == nil || *row.Result != ev.Result || row.Delivery != "none" ||
				len(runtime.queues) != 0 ||
				hook.calls != 1 {
				t.Fatal(row, hook.calls)
			}
			ev.ProviderToolCallID = "missing"
			ev.ToolName = ""
			for range 2 {
				if err := s.OnNativeToolEvent(t.Context(), "parent", ev); err != nil {
					t.Fatal(err)
				}
			}
			if len(db.rows) != 1 || len(s.nativeMisses) != 1 {
				t.Fatal(db.rows, s.nativeMisses)
			}
		})
	})
}

// UT-032/UT-057 and IT-025 service half: asynchronous injection completion owns the single fallback.
func TestSubagentSteerCompletion(t *testing.T) {
	t.Parallel()
	for _, injected := range []bool{true, false} {
		name := "Should deliver successful pending injection"
		if !injected {
			name = "Should queue failed pending injection once"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				s, db, runtime := newSubagentTestService(t)
				snap := runtime.snapshots["parent"]
				snap.CanSteer = true
				runtime.snapshots["parent"] = snap
				completion := make(chan error, 1)
				runtime.steer = acp.SteerResult{Attempt: acp.SteerAttemptPendingInjection, Completion: completion}
				row := requireSubagent(t, s, subagentTestRequest())
				settleTestChild(t, s, runtime, &row)
				if injected {
					completion <- nil
				} else {
					completion <- testSubagentError()
				}
				close(completion)
				synctest.Wait()
				current := db.rows[row.ID]
				wake := db.wakes[*current.WakeMessageID]
				if injected {
					if current.Delivery != "delivered" || len(runtime.queues) != 0 {
						t.Fatal(current)
					}
				} else {
					if !wake.SteerRequeued || wake.Route != "queue" || len(runtime.queues) != 1 {
						t.Fatal(wake)
					}
					if err := s.OnSteerOutcome(t.Context(), "parent", wake.WakeMessageID, false); err != nil {
						t.Fatal(err)
					}
					if len(runtime.queues) != 1 {
						t.Fatal(runtime.queues)
					}
				}
			})
		})
	}
}

// IT-031 database boundary: recovery consumes persisted tasks and stale reservations against real SQLite.
// The runtime fixture is the I/O boundary; the port, constraints and transactions are production code.
func TestSubagentDatabaseRecovery(t *testing.T) {
	t.Run("Should reconcile durable admissions and stale reservations", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		parent := createSession(t, h.harness)
		child := createSession(t, h.harness)
		s, _, runtime := newSubagentTestService(t)
		s.store = h.db
		runtime.snapshots[parent.ID] = runtime.snapshots["parent"]
		snapshot := runtime.snapshots[parent.ID]
		info := *snapshot.Info
		info.ID = parent.ID
		info.WorkspaceID = h.workspaceID
		snapshot.Info = &info
		runtime.snapshots[parent.ID] = snapshot
		runtime.snapshots[child.ID] = subagentSnapshot{
			Info:   &Info{ID: child.ID, WorkspaceID: h.workspaceID, State: StateActive},
			Active: true,
		}
		stale := store.SessionSubagent{
			ID:                 "sub-stale",
			WorkspaceID:        h.workspaceID,
			ParentSessionID:    parent.ID,
			ParentTurnID:       "turn",
			Origin:             "delegated",
			IdempotencyKey:     "stale",
			RequestFingerprint: "stale",
			Title:              "Stale",
			Role:               "general",
			Depth:              1,
			Status:             "queued",
			WorkState:          "working",
			WakePolicy:         "always",
			Delivery:           "none",
			PendingTask:        new("task"),
			CreatedAt:          s.now().Add(-5 * time.Minute),
		}
		if _, _, err := h.db.ReserveSubagent(t.Context(), stale); err != nil {
			t.Fatal(err)
		}
		running := stale
		running.ID = "sub-running"
		running.IdempotencyKey = "running"
		running.RequestFingerprint = "running"
		running.CreatedAt = s.now()
		running.PendingTask = new("recover task")
		if _, _, err := h.db.ReserveSubagent(t.Context(), running); err != nil {
			t.Fatal(err)
		}
		if _, err := h.db.LinkChild(t.Context(), running.ID, child.ID, s.now()); err != nil {
			t.Fatal(err)
		}
		if err := s.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		got, err := h.db.GetSubagent(t.Context(), h.workspaceID, stale.ID)
		if err != nil || got.Status != "failed" || got.Error == nil || *got.Error != "delegation interrupted" {
			t.Fatal(got, err)
		}
		got, err = h.db.GetSubagent(t.Context(), h.workspaceID, running.ID)
		if err != nil || got.PendingTask != nil || runtime.admitted[running.ID] != "recover task" ||
			got.Status != "running" {
			t.Fatal(got, err)
		}
		if err := s.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		if len(runtime.admitted) != 1 {
			t.Fatal(runtime.admitted)
		}
	})
}

// UT-003/UT-005/UT-017: provider availability reports exact constraints, separate from model advisories.
// Owner: session target resolution; the service suite owns these caller-visible errors.
func TestSubagentProviderAvailability(t *testing.T) {
	t.Run("Should bound capability models without restricting delegate targets UT-017", func(t *testing.T) {
		t.Parallel()
		models := make([]modelcatalog.Model, 0, 1222)
		for _, provider := range []string{"available", "absent"} {
			for i := range 611 {
				models = append(models, modelcatalog.Model{ProviderID: provider, ModelID: fmt.Sprintf("model-%03d", i)})
			}
		}
		h := newHarness(t, WithModelCatalog(modelCatalogStub{models: models}))
		h.cfg.Providers["available"] = compozyconfig.ProviderConfig{
			Command: "go", AuthMode: compozyconfig.ProviderAuthModeNone,
			Models: compozyconfig.ProviderModelsConfig{Default: "model-610"},
		}
		h.cfg.Providers["absent"] = compozyconfig.ProviderConfig{
			Command: "compozy-test-nonexistent-provider", AuthMode: compozyconfig.ProviderAuthModeNativeCLI,
		}
		runtime := managerSubagentRuntime{h.manager}
		parent := &Info{WorkspaceID: h.workspaceID, AgentName: "coder", Provider: "available", Model: "model-609"}
		_, options, err := runtime.Capabilities(t.Context(), parent)
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, option := range options {
			if option.Provider != "available" && option.Provider != "absent" {
				continue
			}
			found++
			if option.ModelsTotal != 611 || !option.ModelsTruncated {
				t.Fatal(option)
			}
			if option.Provider == "absent" {
				if option.CanDelegate || len(option.Models) != 0 {
					t.Fatal(option)
				}
			} else if !option.CanDelegate || len(option.Models) != 40 || option.Models[0].ID != "model-609" || option.Models[1].ID != "model-610" || option.Models[2].ID != "model-000" {
				t.Fatal(option)
			}
		}
		if found != 2 {
			t.Fatalf("found %d providers", found)
		}
		target, err := runtime.Resolve(t.Context(), parent, SubagentTarget{Model: "model-600"})
		if err != nil || target.Model != "model-600" {
			t.Fatal(target, err)
		}
		for _, total := range []int{0, 1, 40, 41} {
			full := SubagentProviderOption{CanDelegate: true}
			for i := range total {
				full.Models = append(full.Models, SubagentModelOption{ID: fmt.Sprint(i)})
			}
			preview := subagentCapabilityModels(full, "0", "0")
			if len(preview.Models) != min(total, 40) || preview.ModelsTotal != total ||
				preview.ModelsTruncated != (total > 40) {
				t.Fatal(preview)
			}
		}
	})

	t.Parallel()
	h := newHarness(t)
	runtime := managerSubagentRuntime{h.manager}
	cfg := compozyconfig.DefaultWithHome(h.homePaths)
	cfg.Providers["absent"] = compozyconfig.ProviderConfig{
		Command:  "compozy-test-nonexistent-provider",
		AuthMode: compozyconfig.ProviderAuthModeNativeCLI,
	}
	cfg.Providers["unauthenticated"] = compozyconfig.ProviderConfig{
		Command:  "go",
		AuthMode: compozyconfig.ProviderAuthModeBoundSecret,
		CredentialSlots: []compozyconfig.ProviderCredentialSlot{
			{
				Name:      "api_key",
				TargetEnv: "TEST_API_KEY",
				SecretRef: "env:COMPOZY_SUBAGENT_TEST_MISSING_CREDENTIAL",
				Required:  true,
			},
		},
	}
	cfg.Providers["available"] = compozyconfig.ProviderConfig{
		Command:  "go",
		AuthMode: compozyconfig.ProviderAuthModeNone,
	}
	for _, tc := range []struct {
		name       string
		available  bool
		constraint string
	}{
		{"absent", false, "Provider is not installed."},
		{"unauthenticated", false, "Provider is not authenticated."},
		{"available", true, "Model catalog unavailable; the agent default model will be used."},
	} {
		t.Run("Should report "+tc.name, func(t *testing.T) {
			option, err := runtime.providerOption(t.Context(), &cfg, tc.name)
			if err != nil || option.CanDelegate != tc.available || len(option.Constraints) != 1 ||
				option.Constraints[0] != tc.constraint {
				t.Fatal(option, err)
			}
		})
	}
	_, err := runtime.providerOption(t.Context(), &cfg, "nope")
	typed, ok := errors.AsType[*SubagentError](err)
	if !ok || typed.Code != "provider_unavailable" {
		t.Fatal(err)
	}
}

// IT-031: explicitly stopped parents cannot receive a stale open wake on restart.
// Owner: service recovery, with the runtime boundary supplied by this suite's fixture.
func TestSubagentRecoveryStoppedParent(t *testing.T) {
	t.Run("Should dispose wakes after an explicit parent stop", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		settleTestChild(t, s, runtime, &row)
		wake := *db.rows[row.ID].WakeMessageID
		snap := runtime.snapshots["parent"]
		snap.Info.State = StateStopped
		snap.Info.StopReason = store.StopUserCanceled
		snap.Active = false
		runtime.snapshots["parent"] = snap
		if err := s.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		if db.rows[row.ID].Delivery != "disposed" || db.wakes[wake].State != "canceled" {
			t.Fatal(db.rows[row.ID], db.wakes[wake])
		}
	})
}

// Invariant: concurrent denial replays preserve the same capability error and release serialization state.
// Owner: session delegation lifecycle; canonical service suite.
func TestSubagentConcurrentDenial(t *testing.T) {
	t.Run("Should return capability denial to every waiting replay", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, runtime := newSubagentTestService(t)
			runtime.spawnBlock = make(chan struct{})
			runtime.spawnErr = ErrSubagentCapabilityDenied
			results := make(chan error, 2)
			go func() { _, err := s.Delegate(t.Context(), subagentTestRequest()); results <- err }()
			synctest.Wait()
			go func() { _, err := s.Delegate(t.Context(), subagentTestRequest()); results <- err }()
			synctest.Wait()
			close(runtime.spawnBlock)
			synctest.Wait()
			for range 2 {
				if err := <-results; !errors.Is(err, ErrSubagentCapabilityDenied) {
					t.Fatal(err)
				}
			}
			if len(db.rows) != 0 || len(s.parents) != 0 || len(s.flights) != 0 {
				t.Fatal("denied delegation retained state")
			}
		})
	})
}

type blockingSubagentSettledHook struct{ entered, release chan struct{} }

var _ SubagentSettledDispatcher = (*blockingSubagentSettledHook)(nil)

func (h *blockingSubagentSettledHook) DispatchSubagentSettled(ctx context.Context, _ store.SessionSubagent) error {
	close(h.entered)
	select {
	case <-h.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Invariant: observe-only hooks cannot serialize parent delivery or status reads.
// Owner: session publication; canonical service suite.
func TestSubagentObserveHook(t *testing.T) {
	t.Run("Should allow status while the settled hook is blocked", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, _, runtime := newSubagentTestService(t)
			hook := &blockingSubagentSettledHook{entered: make(chan struct{}), release: make(chan struct{})}
			s.settled = hook
			row := requireSubagent(t, s, subagentTestRequest())
			settleTestChild(t, s, runtime, &row)
			<-hook.entered
			status, err := s.Status(t.Context(), subagentTestRequest().Caller, row.ID)
			if err != nil || status.Delivery != store.SubagentDeliveryAcknowledged {
				t.Fatal(status, err)
			}
			close(hook.release)
			synctest.Wait()
		})
	})
}

// Invariant: an in-flight steer owns an immutable batch, and channel-less acceptance completes at turn settlement.
// Owner: service delivery; canonical UT-057 and IT-025 service suite.
func TestSubagentPendingSteerBatch(t *testing.T) {
	for _, withCompletion := range []bool{true, false} {
		name := "Should isolate a later result until completion"
		if !withCompletion {
			name = "Should settle channel-less injection at turn completion without a duplicate"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				s, db, r := newSubagentTestService(t)
				snap := r.snapshots["parent"]
				snap.CanSteer = true
				r.snapshots["parent"] = snap
				completion := make(chan error, 1)
				r.steer = acp.SteerResult{Attempt: acp.SteerAttemptPendingInjection}
				if withCompletion {
					r.steer.Completion = completion
				}
				first := requireSubagent(t, s, subagentTestRequest())
				req := subagentTestRequest()
				req.IdempotencyKey = "second"
				second := requireSubagent(t, s, req)
				settleTestChild(t, s, r, &first)
				firstWake := *db.rows[first.ID].WakeMessageID
				settleTestChild(t, s, r, &second)
				if db.rows[second.ID].Delivery != store.SubagentDeliveryPending || len(r.queues) != 0 {
					t.Fatal(db.rows, r.queues)
				}
				if db.wakes[firstWake].State != store.SubagentWakeStateDispatched {
					t.Fatal(db.wakes)
				}
				snap.Active = false
				r.snapshots["parent"] = snap
				if withCompletion {
					completion <- nil
					close(completion)
					synctest.Wait()
				} else if err := s.OnParentTurnSettled(t.Context(), "parent", "turn"); err != nil {
					t.Fatal(err)
				}
				if db.rows[first.ID].Delivery != store.SubagentDeliveryDelivered {
					t.Fatal(db.rows[first.ID])
				}
				if db.rows[second.ID].WakeMessageID == nil || *db.rows[second.ID].WakeMessageID == firstWake ||
					len(r.queues) != 1 {
					t.Fatal(db.rows[second.ID], r.queues)
				}
			})
		})
	}
}

// Invariant: provider errors are bounded across batches; cancellation waits for the next settled turn.
// Owner: session wake state machine.
func TestSubagentWakeFailureLimit(t *testing.T) {
	t.Run("Should keep a wake retryable while the parent's input queue is full", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, r := newSubagentTestService(t)
			r.queueFull = 1
			row := requireSubagent(t, s, subagentTestRequest())
			snap := r.snapshots["parent"]
			snap.Active, snap.TurnID = false, ""
			r.snapshots["parent"] = snap
			settleTestChild(t, s, r, &row)
			unlock := s.lock("parent")
			deferred, queued := db.rows[row.ID], len(r.queues)
			unlock()
			if deferred.Delivery != store.SubagentDeliveryPending || queued != 0 {
				t.Fatal(deferred, queued)
			}
			time.Sleep(subagentWakeRetryDelay * subagentQueueFullRetrySteps)
			synctest.Wait()
			unlock = s.lock("parent")
			current, queued := db.rows[row.ID], len(r.queues)
			attempts := db.wakes[*current.WakeMessageID].Attempts
			unlock()
			if current.Delivery != store.SubagentDeliveryClaimed || queued != 1 || attempts != 0 {
				t.Fatal(current, queued, attempts)
			}
		})
	})
	t.Run("Should abandon after three failures without immediately retrying", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, r := newSubagentTestService(t)
			var logs strings.Builder
			s.logger = slog.New(slog.NewTextHandler(&logs, nil))
			row := requireSubagent(t, s, subagentTestRequest())
			settleTestChild(t, s, r, &row)
			for attempt := 1; attempt <= 3; attempt++ {
				wake := *db.rows[row.ID].WakeMessageID
				if err := s.OnWakeDispatched(t.Context(), "parent", wake); err != nil {
					t.Fatal(err)
				}
				// Both lifecycle orderings must produce the same delayed successor.
				if err := s.OnParentTurnSettled(t.Context(), "parent", "turn"); err != nil {
					t.Fatal(err)
				}
				if err := s.OnWakeFailed(t.Context(), "parent", wake); err != nil {
					t.Fatal(err)
				}
				want := store.SubagentDeliveryPending
				if attempt == 3 {
					want = store.SubagentDeliveryDisposed
				}
				unlock := s.lock("parent")
				current, queueCount := db.rows[row.ID], len(r.queues)
				unlock()
				if current.Delivery != want || queueCount != attempt {
					t.Fatal(current, queueCount)
				}
				if err := s.OnParentTurnSettled(t.Context(), "parent", "turn"); err != nil {
					t.Fatal(err)
				}
				unlock = s.lock("parent")
				queueCount = len(r.queues)
				unlock()
				if queueCount != attempt {
					t.Fatal("turn settlement bypassed retry backoff", queueCount)
				}
				synctest.Wait()
				time.Sleep(subagentWakeRetryDelay*time.Duration(attempt) - time.Nanosecond)
				synctest.Wait()
				unlock = s.lock("parent")
				queueCount = len(r.queues)
				unlock()
				if queueCount != attempt {
					t.Fatal("retry ran before its attempt-scaled backoff", queueCount, attempt)
				}
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				if len(r.queues) != min(attempt+1, 3) {
					t.Fatal("retry did not run at its deadline", len(r.queues), attempt)
				}
			}
			if len(r.queues) != 3 || strings.Count(logs.String(), "subagent.wake_abandoned") != 1 {
				t.Fatal(r.queues, logs.String())
			}
		})
	})
}

// Invariant: interrupted native work stops contributing live children, and terminal-first calls produce results.
// Owner: session native lifecycle and recovery.
func TestSubagentNativeRecoveryEdges(t *testing.T) {
	for _, edge := range []string{"settled", "interrupted", "stopped", "recovery", "terminal-first"} {
		t.Run("Should finalize native work at "+edge, func(t *testing.T) {
			t.Parallel()
			s, db, r := newSubagentTestService(t)
			ev := NativeSubagentEvent{
				WorkspaceID:        "ws",
				ParentTurnID:       "turn",
				ProviderToolCallID: "native",
				ToolName:           "Agent",
				Status:             "in_progress",
			}
			if edge == "terminal-first" {
				ev.Status = "completed"
				ev.Result = "answer"
			}
			if err := s.OnNativeToolEvent(t.Context(), "parent", ev); err != nil {
				t.Fatal(err)
			}
			snap := r.snapshots["parent"]
			snap.Active = false
			r.snapshots["parent"] = snap
			var err error
			switch edge {
			case "settled":
				err = s.OnParentTurnSettled(t.Context(), "parent", "turn")
			case "interrupted":
				err = s.OnParentTurnInterrupted(t.Context(), "parent", "turn")
			case "stopped":
				err = s.OnParentStopped(t.Context(), "parent")
			case "recovery":
				err = s.Recover(t.Context())
			}
			row := db.rows[subagentID("parent", "native")]
			want := store.SubagentStatusInterrupted
			if edge == "terminal-first" {
				want = store.SubagentStatusCompleted
			}
			if err != nil || row.Status != want || row.Delivery != store.SubagentDeliveryNone || len(r.queues) != 0 {
				t.Fatal(row, err, r.queues)
			}
		})
	}
}

func TestSubagentRecoveryIsolation(t *testing.T) {
	t.Run("Should keep sent wake inputs and continue after orphan failures", func(t *testing.T) {
		t.Parallel()
		s, db, r := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		settleTestChild(t, s, r, &row)
		wake := db.wakes[*db.rows[row.ID].WakeMessageID]
		r.inputStatuses = map[string]string{wake.InputEntryID: store.SessionInputQueueStatusSent}
		db.orphans = []string{"bad-orphan", "another-orphan"}
		r.errorStop = testSubagentError()
		snap := r.snapshots["parent"]
		snap.Active = false
		r.snapshots["parent"] = snap
		if err := s.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		if len(r.stopped) != 2 || len(r.queues) != 1 || db.rows[row.ID].Delivery != store.SubagentDeliveryDelivered {
			t.Fatal(r.stopped, r.queues, db.rows[row.ID])
		}
	})
	for _, status := range []string{store.SessionInputQueueStatusFailed, store.SessionInputQueueStatusCanceled} {
		t.Run("Should release a recovered "+status+" wake", func(t *testing.T) {
			t.Parallel()
			s, db, r := newSubagentTestService(t)
			row := requireSubagent(t, s, subagentTestRequest())
			settleTestChild(t, s, r, &row)
			wake := db.wakes[*db.rows[row.ID].WakeMessageID]
			if err := s.OnWakeDispatched(t.Context(), "parent", wake.WakeMessageID); err != nil {
				t.Fatal(err)
			}
			r.inputStatuses = map[string]string{wake.InputEntryID: status}
			if err := s.recoverWake(t.Context(), wake); err != nil {
				t.Fatal(err)
			}
			if status == store.SessionInputQueueStatusFailed {
				if db.rows[row.ID].Delivery != store.SubagentDeliveryPending ||
					db.wakes[wake.WakeMessageID].Attempts != 1 ||
					len(r.queues) != 1 {
					t.Fatal(db.rows, db.wakes, r.queues)
				}
			} else if db.rows[row.ID].WakeMessageID == nil || *db.rows[row.ID].WakeMessageID == wake.WakeMessageID || len(r.queues) != 2 {
				t.Fatal(db.rows, r.queues)
			}
		})
	}
}

func TestSubagentUserSteerSlot(t *testing.T) {
	t.Run("Should queue without displacing user steer UT-057", func(t *testing.T) {
		t.Parallel()
		s, db, r := newSubagentTestService(t)
		var logs strings.Builder
		s.logger = slog.New(slog.NewTextHandler(&logs, nil))
		snap := r.snapshots["parent"]
		snap.CanSteer = true
		snap.UserSteer = true
		r.snapshots["parent"] = snap
		r.steer = acp.SteerResult{Attempt: acp.SteerAttemptInjected}
		row := requireSubagent(t, s, subagentTestRequest())
		settleTestChild(t, s, r, &row)
		wake := db.wakes[*db.rows[row.ID].WakeMessageID]
		if wake.Route != store.SubagentWakeRouteQueue || len(r.queues) != 1 ||
			!strings.Contains(logs.String(), "steer_slot_taken") {
			t.Fatal(wake, r.queues, logs.String())
		}
	})
}

type countingNativeSubagentStore struct {
	store.SubagentStore
	lookups int
}

func (s *countingNativeSubagentStore) GetSubagent(ctx context.Context, ws, id string) (store.SessionSubagent, error) {
	s.lookups++
	return s.SubagentStore.GetSubagent(ctx, ws, id)
}

func (s *countingNativeSubagentStore) GetSubagentByChild(
	ctx context.Context,
	id string,
) (store.SessionSubagent, error) {
	s.lookups++
	return s.SubagentStore.GetSubagentByChild(ctx, id)
}

// Invariant: inner native chunks do not hit the store or recursively finalize, and diagnostic deduplication is bounded.
// Owner: native ingest hot path.
func TestSubagentNativeInnerEvents(t *testing.T) {
	t.Run("Should process inner chunks without database reads and bound stitch misses", func(t *testing.T) {
		t.Parallel()
		s, db, _ := newSubagentTestService(t)
		counted := &countingNativeSubagentStore{SubagentStore: db}
		s.store = counted
		if err := s.OnNativeToolEvent(
			t.Context(),
			"parent",
			NativeSubagentEvent{
				WorkspaceID:        "ws",
				ParentTurnID:       "turn",
				ToolName:           "Agent",
				ProviderToolCallID: "known",
				Status:             "in_progress",
			},
		); err != nil {
			t.Fatal(err)
		}
		before := counted.lookups
		for range 2048 {
			if err := s.OnNativeToolEvent(
				t.Context(),
				"parent",
				NativeSubagentEvent{WorkspaceID: "ws", ProviderToolCallID: "known"},
			); err != nil {
				t.Fatal(err)
			}
		}
		for i := range 2048 {
			if err := s.OnNativeToolEvent(
				t.Context(),
				"parent",
				NativeSubagentEvent{WorkspaceID: "ws", ProviderToolCallID: fmt.Sprintf("unknown-%d", i)},
			); err != nil {
				t.Fatal(err)
			}
		}
		if counted.lookups != before || len(s.nativeMisses) > 1024 {
			t.Fatal(counted.lookups, before, len(s.nativeMisses))
		}
	})
}

type faultySubagentAdmission struct {
	*subagentTestRuntime
	bad string
}

func (r faultySubagentAdmission) HasAdmission(ctx context.Context, row store.SessionSubagent) (bool, error) {
	if row.ID == r.bad {
		return false, errors.New("corrupt child transcript")
	}
	return r.subagentTestRuntime.HasAdmission(ctx, row)
}

func TestSubagentRecoveryCorruptChild(t *testing.T) {
	t.Run("Should recover healthy children despite one corrupt transcript", func(t *testing.T) {
		t.Parallel()
		s, db, r := newSubagentTestService(t)
		bad := requireSubagent(t, s, subagentTestRequest())
		req := subagentTestRequest()
		req.IdempotencyKey = "healthy"
		healthy := requireSubagent(t, s, req)
		for _, row := range []Subagent{bad, healthy} {
			snap := r.snapshots[*row.ChildSessionID]
			snap.Active = false
			r.snapshots[*row.ChildSessionID] = snap
		}
		s.runtime = faultySubagentAdmission{subagentTestRuntime: r, bad: bad.ID}
		if err := s.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		if db.rows[healthy.ID].Status != store.SubagentStatusCompleted ||
			db.rows[bad.ID].Status != store.SubagentStatusRunning {
			t.Fatal(db.rows)
		}
	})
	t.Run("Should interrupt an unresumable shutdown child without disposing its result", func(t *testing.T) {
		t.Parallel()
		s, db, r := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		r.snapshots[*row.ChildSessionID] = subagentSnapshot{
			Info: &Info{ID: *row.ChildSessionID, State: StateStopped, StopReason: store.StopShutdown},
		}
		r.errorResume = errors.New("provider cannot resume")
		if err := s.OnChildSettled(t.Context(), *row.ChildSessionID); err != nil {
			t.Fatal(err)
		}
		if db.rows[row.ID].Status != store.SubagentStatusRunning {
			t.Fatal(db.rows[row.ID])
		}
		if err := s.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		if db.rows[row.ID].Status != store.SubagentStatusInterrupted ||
			db.rows[row.ID].Delivery != store.SubagentDeliveryClaimed {
			t.Fatal(db.rows[row.ID])
		}
	})
}

// N2: accepting cancellation finalizes the row before an independent lifecycle-owned stop.
// Owner: service cancellation; canonical subagent service suite.
type blockedSubagentStop struct {
	*subagentTestRuntime
	entered chan context.Context
	release chan struct{}
}

func (r blockedSubagentStop) Stop(ctx context.Context, id string) error {
	r.entered <- ctx
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.release:
		return r.subagentTestRuntime.Stop(ctx, id)
	}
}

func TestSubagentCancelLifetime(t *testing.T) {
	t.Run("Should finalize synchronously and keep stopping after the request is canceled", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s, db, runtime := newSubagentTestService(t)
			row := requireSubagent(t, s, subagentTestRequest())
			blocked := blockedSubagentStop{runtime, make(chan context.Context), make(chan struct{})}
			s.runtime = blocked
			start := make(chan struct{})
			launch := s.launch
			s.launch = func(f func()) {
				launch(func() {
					select {
					case <-start:
						f()
					case <-s.ctx.Done():
					}
				})
			}
			ctx, cancel := context.WithCancel(t.Context())
			out, err := s.Cancel(ctx, SubagentActor{Kind: "operator"}, row.ID, "")
			cancel()
			if err != nil || out.Status != "cancel_requested" ||
				db.rows[row.ID].Status != store.SubagentStatusCanceled ||
				db.rows[row.ID].Delivery != store.SubagentDeliveryDisposed {
				t.Fatal(out, db.rows[row.ID], err)
			}
			close(start)
			stopCtx := <-blocked.entered
			if stopCtx.Err() != nil {
				t.Fatal("stop inherited request cancellation", stopCtx.Err())
			}
			close(blocked.release)
			synctest.Wait()
			if len(runtime.stopped) != 1 {
				t.Fatal(runtime.stopped)
			}
		})
	})
}

// N3/N6: pending steering is retriable after interruption and tracking ends with the wake.
// Owner: service delivery; extends the canonical steer-batch invariant.
func TestSubagentPendingSteerInterruption(t *testing.T) {
	t.Run("Should retain steering until the last batch member is disposed", func(t *testing.T) {
		t.Parallel()
		s, db, runtime := newSubagentTestService(t)
		row := requireSubagent(t, s, subagentTestRequest())
		settleTestChild(t, s, runtime, &row)
		wake := *db.rows[row.ID].WakeMessageID
		other := db.rows[row.ID]
		other.ID = "other-member"
		db.rows[other.ID] = other
		s.steerTurns = map[string]string{wake: "turn"}
		for _, id := range []string{row.ID, other.ID} {
			if err := s.dispose(
				t.Context(),
				store.SubagentDisposeFilter{ParentSessionID: "parent", IDs: []string{id}},
			); err != nil {
				t.Fatal(err)
			}
			_, tracked := s.steerTurns[wake]
			if tracked != (id == row.ID) {
				t.Fatal("steering tracking must follow remaining claimed members", id, tracked)
			}
		}
	})
	for _, edge := range []string{"interrupt", "stop", "cancel-wake", "injected", "fallback"} {
		t.Run("Should release pending steer tracking on "+edge, func(t *testing.T) {
			t.Parallel()
			s, db, runtime := newSubagentTestService(t)
			snap := runtime.snapshots["parent"]
			snap.CanSteer = true
			runtime.snapshots["parent"] = snap
			runtime.steer = acp.SteerResult{Attempt: acp.SteerAttemptPendingInjection}
			row := requireSubagent(t, s, subagentTestRequest())
			settleTestChild(t, s, runtime, &row)
			wake := *db.rows[row.ID].WakeMessageID
			if len(s.steerTurns) != 1 {
				t.Fatal(s.steerTurns)
			}
			var err error
			switch edge {
			case "interrupt":
				err = s.OnParentTurnInterrupted(t.Context(), "parent", "turn")
			case "stop":
				err = s.OnParentStopped(t.Context(), "parent")
			case "cancel-wake":
				err = s.OnWakeCanceled(t.Context(), "parent", wake)
			case "injected":
				err = s.OnSteerOutcome(t.Context(), "parent", wake, true)
			case "fallback":
				err = s.OnSteerOutcome(t.Context(), "parent", wake, false)
			}
			if err != nil || len(s.steerTurns) != 0 {
				t.Fatal(s.steerTurns, err)
			}
			if edge == "interrupt" {
				if db.rows[row.ID].Delivery != store.SubagentDeliveryPending || len(runtime.queues) != 0 {
					t.Fatal(db.rows, runtime.queues)
				}
				snap.Active = false
				runtime.snapshots["parent"] = snap
				if err := s.OnParentTurnSettled(t.Context(), "parent", "turn"); err != nil {
					t.Fatal(err)
				}
				current := db.rows[row.ID]
				if current.Delivery != store.SubagentDeliveryClaimed || current.WakeMessageID == nil ||
					*current.WakeMessageID == wake ||
					len(runtime.queues) != 1 {
					t.Fatal(current, runtime.queues)
				}
			}
		})
	}
}

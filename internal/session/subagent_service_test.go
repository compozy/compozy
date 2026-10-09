package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
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
		s, db, runtime := newSubagentTestService(t)
		s.resultLimit = func() int { return 3 }
		hook := &subagentTestHook{}
		s.settled = hook
		row := requireSubagent(t, s, subagentTestRequest())
		runtime.results[*row.ChildSessionID] = "界界界界"
		settleTestChild(t, s, runtime, &row)
		if err := s.OnChildSettled(t.Context(), *row.ChildSessionID); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(t.Context(), "ws", row.ID)
		if err != nil || !got.ResultTruncated || *got.Result != "界界界" || got.Hint == "" || hook.calls != 1 ||
			db.rows[row.ID].Delivery != "claimed" {
			t.Fatal(got, err, hook.calls)
		}
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
			s, db, _ := newSubagentTestService(t)
			row := requireSubagent(t, s, subagentTestRequest())
			for range 9 {
				s.OnChildActivity(t.Context(), *row.ChildSessionID, "earlier")
			}
			s.OnChildActivity(t.Context(), *row.ChildSessionID, strings.Repeat("x", 400)+"\nignored")
			synctest.Wait()
			time.Sleep(time.Second)
			synctest.Wait()
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
			runtime.snapshots[*row.ChildSessionID] = subagentSnapshot{Info: &tc.info, Active: tc.active}
			runtime.results[*row.ChildSessionID] = "last answer"
			if err := s.OnChildSettled(t.Context(), *row.ChildSessionID); err != nil {
				t.Fatal(err)
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
		if err != nil || got.Status != "cancel_requested" || db.rows[row.ID].Delivery != "disposed" {
			t.Fatal(got, err)
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
		err := validateSubagentModel("gpt-9", options)
		typed, ok := errors.AsType[*SubagentError](err)
		if !ok || typed.Code != "model_unavailable" ||
			typed.Message != "Model gpt-9 is unavailable. Available models: a, b, c, d, e, f, g, h, i, j" {
			t.Fatal(err)
		}
	})
	t.Run("Should permit default with unavailable catalog UT-005", func(t *testing.T) {
		t.Parallel()
		if err := validateSubagentModel("default", nil); err != nil {
			t.Fatal(err)
		}
	})
}

package globaldb

import (
	"encoding/json"
	"testing"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/testutil"
)

func TestLoopRunTerminalCausesShouldProjectSettledCause(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t, "ws-1")
	ctx := testutil.Context(t)
	now := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)

	blocked := testLoopRun("looprun-terminal-blocked", now, looppkg.StatusBlocked)
	blocked.WorkspaceID = "ws-1"
	insertLoopCatalogRunForTest(t, globalDB, blocked)
	active := testLoopRun("looprun-terminal-active", now, looppkg.StatusRunning)
	active.WorkspaceID = "ws-1"
	insertLoopCatalogRunForTest(t, globalDB, active)

	terminalPayload, err := json.Marshal(map[string]any{
		"from": "running", "to": string(looppkg.StatusBlocked), "status": string(looppkg.StatusBlocked),
		"cause": string(looppkg.TransitionCauseGateRejected), "reason_code": "gate_blocked",
		"gate_id": "contract", "node_id": "phase_gate", "message": "external blocker",
	})
	if err != nil {
		t.Fatalf("json.Marshal(terminal payload) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO loop_run_events (id, loop_run_id, workspace_id, seq, kind, payload_json, at)
		 VALUES (?, ?, ?, 1, 'status_changed', ?, ?)`,
		"event-terminal-blocked",
		string(blocked.ID),
		"ws-1",
		string(terminalPayload),
		now.Add(time.Minute).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert terminal status event error = %v", err)
	}
	runningPayload, err := json.Marshal(map[string]any{
		"from": "queued", "to": string(looppkg.StatusRunning), "status": string(looppkg.StatusRunning),
		"cause": string(looppkg.TransitionCauseStart),
	})
	if err != nil {
		t.Fatalf("json.Marshal(running payload) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO loop_run_events (id, loop_run_id, workspace_id, seq, kind, payload_json, at)
		 VALUES (?, ?, ?, 1, 'status_changed', ?, ?)`,
		"event-terminal-active",
		string(active.ID),
		"ws-1",
		string(runningPayload),
		now.Add(time.Minute).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert running status event error = %v", err)
	}

	causes, err := globalDB.ListLoopRunTerminalCauses(
		ctx,
		"ws-1",
		[]looppkg.RunID{blocked.ID, active.ID},
	)
	if err != nil {
		t.Fatalf("ListLoopRunTerminalCauses() error = %v", err)
	}
	if _, ok := causes[active.ID]; ok {
		t.Fatalf("terminal causes included the non-terminal run: %#v", causes)
	}
	cause, ok := causes[blocked.ID]
	if !ok {
		t.Fatalf("terminal causes = %#v, want %q", causes, blocked.ID)
	}
	if cause.Status != looppkg.StatusBlocked || cause.ReasonCode != "gate_blocked" ||
		cause.GateID != "contract" || cause.NodeID != "phase_gate" || cause.Message != "external blocker" {
		t.Fatalf("terminal cause = %#v", cause)
	}
	if cause.Cause != looppkg.TransitionCauseGateRejected {
		t.Fatalf("terminal cause.Cause = %q, want gate_rejected", cause.Cause)
	}
	if want := now.Add(time.Minute); !cause.At.Equal(want) {
		t.Fatalf("terminal cause.At = %s, want %s", cause.At, want)
	}

	foreign, err := globalDB.ListLoopRunTerminalCauses(
		ctx,
		"ws-foreign",
		[]looppkg.RunID{blocked.ID},
	)
	if err != nil {
		t.Fatalf("ListLoopRunTerminalCauses(foreign) error = %v", err)
	}
	if len(foreign) != 0 {
		t.Fatalf("foreign terminal causes = %#v, want none", foreign)
	}
}

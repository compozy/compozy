import { describe, expect, it } from "vitest";

import { applyLoopEventFrame, emptyLoopRunLiveState } from "../loop-events";
import type { LoopRunEventKind } from "../../types";
import { loopEventFrame as frame } from "./loop-event-frame";

describe("applyLoopEventFrame", () => {
  it("Should retain structural frames in seq order, bounded to the newest history", () => {
    let state = emptyLoopRunLiveState();
    for (let i = 1; i <= 520; i++) {
      state = applyLoopEventFrame(state, frame("status_changed", { status: "running" }, i));
    }
    expect(state.frames).toHaveLength(500);
    expect(state.frames[0].seq).toBe(21);
    expect(state.frames[state.frames.length - 1].seq).toBe(520);
  });

  it("Should never retain token_tick or channel_msg frames (they aggregate elsewhere)", () => {
    let state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("token_tick", { tokens_used: 1_000 }, 1)
    );
    state = applyLoopEventFrame(state, frame("channel_msg", { id: "m1", text: "hello" }, 2));
    state = applyLoopEventFrame(state, frame("node_running", { node_id: "fix" }, 3));
    expect(state.frames.map(f => f.kind)).toEqual(["node_running"]);
    expect(state.tokensUsed).toBe(1_000);
  });

  it("Should bound a goal-turn flood to the newest slice while never evicting node_running", () => {
    let state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("node_running", { node_id: "goal" }, 1)
    );
    // A goal node emits an unbounded stream of turns after it starts. They
    // aggregate into goalTurns and must never consume the frame retention
    // window — and the goalTurns slice itself must stay bounded (newest 500).
    for (let seq = 2; seq <= 700; seq++) {
      const kind: LoopRunEventKind = seq % 2 === 0 ? "goal_turn_started" : "goal_turn_completed";
      state = applyLoopEventFrame(
        state,
        frame(kind, { node_id: "goal", prompt_id: `prompt_${seq}`, turn: seq }, seq)
      );
    }
    expect(state.frames.map(f => f.kind)).toEqual(["node_running"]);
    // 699 unique turns collapse to the newest 500; the oldest are dropped.
    expect(state.goalTurns).toHaveLength(500);
    expect(state.goalTurns.at(-1)?.promptId).toBe("prompt_700");
    expect(state.goalTurns[0].promptId).toBe("prompt_201");
    expect(state.goalTurns.some(turn => turn.promptId === "prompt_2")).toBe(false);
  });

  it("Should skip reconnect-replay duplicates by seq", () => {
    let state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("node_running", { node_id: "fix" }, 5)
    );
    state = applyLoopEventFrame(state, frame("node_running", { node_id: "fix" }, 5));
    state = applyLoopEventFrame(state, frame("node_succeeded", { node_id: "fix" }, 4));
    expect(state.frames).toHaveLength(1);
  });

  it("Should fold a gate_verdict into the per-node verdict map", () => {
    const state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("gate_verdict", {
        node_id: "review",
        generation: 1,
        verdict: "revise",
        confidence: 0.91,
        criteria: [{ id: "all_handled", type: "agent-judge", status: "revise" }],
        blocking_issues: [{ id: "issue_022", note: "no triage decision" }],
        route: "revise",
      })
    );
    const verdict = state.gateVerdicts.review;
    expect(verdict.verdict).toBe("revise");
    expect(verdict.confidence).toBe(0.91);
    expect(verdict.criteria).toHaveLength(1);
    expect(verdict.blockingIssues[0].id).toBe("issue_022");
    expect(verdict.route).toBe("revise");
  });

  it("Should capture a needs_approval payload and the latest token tick", () => {
    let state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("needs_approval", {
        gate_id: "approve",
        title: "Approve merge to main?",
        facts: [{ label: "Branch", value: "main" }],
      })
    );
    state = applyLoopEventFrame(state, frame("token_tick", { tokens_used: 268_000 }, 2));
    expect(state.needsApproval?.gateId).toBe("approve");
    expect(state.needsApproval?.facts[0].value).toBe("main");
    expect(state.tokensUsed).toBe(268_000);
  });

  it("Should merge Goal turn start and completion frames by prompt identity", () => {
    const started = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("goal_turn_started", {
        seq: 7,
        generation: 2,
        node_id: "goal",
        item_index: 1,
        turn: 3,
        prompt_attempt: 1,
        prompt_id: "prompt_7",
        session_id: "session_1",
        binding_handle: "goal:abc",
        binding_epoch: 2,
        actor_kind: "agent",
        actor_id: "implementer",
      })
    );

    const completed = applyLoopEventFrame(
      started,
      frame(
        "goal_turn_completed",
        {
          seq: 7,
          generation: 2,
          node_id: "goal",
          item_index: 1,
          turn: 3,
          prompt_attempt: 1,
          prompt_id: "prompt_7",
          session_id: "session_1",
          result_status: "completed",
          stop_reason: "end_turn",
          verdict_outcome: "rejected",
          blocking_issues: [{ id: "issue_1", note: "Missing evidence" }],
          evidence_ref: "blob_1",
          tokens_used: 420,
        },
        2
      )
    );

    expect(completed.goalTurns).toHaveLength(1);
    expect(completed.goalTurns[0]).toMatchObject({
      promptId: "prompt_7",
      resultStatus: "completed",
      stopReason: "end_turn",
      verdictOutcome: "rejected",
      evidenceRef: "blob_1",
      tokensUsed: 420,
    });
    expect(completed.goalTurns[0].blockingIssues).toEqual([
      { id: "issue_1", note: "Missing evidence" },
    ]);
  });

  it("Should retain a goal_status_changed frame without inventing a turn", () => {
    const state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("goal_status_changed", { from: "active", to: "complete" })
    );
    expect(state.frames[0].kind).toBe("goal_status_changed");
    expect(state.goalTurns).toEqual([]);
  });

  it("Should project a generation-zero coordinator failure from the terminal status event", () => {
    const state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("status_changed", {
        status: "failed",
        generation: 0,
        failure: {
          kind: "coordinator_failure",
          code: "watch_poll_failed",
          cause: "The watch source failed before it could produce a generation.",
          recovery:
            "Verify the Loop watch provider and workspace prerequisites, then start a new run.",
        },
      })
    );

    expect(state.failure).toEqual({
      kind: "coordinator_failure",
      code: "watch_poll_failed",
      cause: "The watch source failed before it could produce a generation.",
      recovery: "Verify the Loop watch provider and workspace prerequisites, then start a new run.",
    });
    expect(state.frames[0].kind).toBe("status_changed");
  });

  it("Should degrade a malformed frame to a retained frame without throwing", () => {
    const state = applyLoopEventFrame(emptyLoopRunLiveState(), frame("gate_verdict", null));
    expect(state.frames).toHaveLength(1);
    expect(state.gateVerdicts).toEqual({});
  });
});

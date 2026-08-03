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
        gate_id: "quality",
        generation: 1,
        verdict: "revise",
        score: 0.91,
        best_generation: 1,
        confidence: 0.99,
        criteria: [{ id: "all_handled", type: "agent-judge", status: "revise", score: 0.91 }],
        blocking_issues: [{ id: "issue_022", note: "no triage decision" }],
        route: "revise",
      })
    );
    const verdict = state.gateVerdicts.review;
    expect(verdict.verdict).toBe("revise");
    expect(verdict.gateId).toBe("quality");
    expect(verdict.score).toBe(0.91);
    expect(verdict.bestGeneration).toBe(1);
    expect(verdict).not.toHaveProperty("confidence");
    expect(verdict.criteria).toHaveLength(1);
    expect(verdict.criteria[0].score).toBe(0.91);
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

  // WT-001: every one of the 15 lifecycle kinds is a story beat an operator must
  // be able to read back after a reconnect, so all of them retain.
  it("Should retain every one of the 15 new lifecycle kinds as a structural frame", () => {
    const kinds: LoopRunEventKind[] = [
      "node_retry_scheduled",
      "node_paused",
      "node_resumed",
      "node_canceled",
      "node_killed",
      "node_quarantined",
      "node_requeued",
      "node_wait_started",
      "node_wait_resumed",
      "node_attention_flagged",
      "node_attention_cleared",
      "effect_results",
      "custom_event",
      "duplicate_suppressed",
      "target_breaker_transition",
    ];
    expect(kinds).toHaveLength(15);
    let state = emptyLoopRunLiveState();
    kinds.forEach((kind, index) => {
      state = applyLoopEventFrame(state, frame(kind, { node_id: "fix_batch" }, index + 1));
    });
    expect(state.frames.map(retained => retained.kind)).toEqual(kinds);
  });

  it("Should fold a retry schedule and drop it once the node moves on", () => {
    let state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame(
        "node_retry_scheduled",
        {
          node_id: "fix_batch",
          item_index: 3,
          generation: 2,
          attempt: 2,
          next_attempt_at: "2026-08-03T14:33:41Z",
          failure_class: "transport",
        },
        1
      )
    );
    expect(state.retrySchedules.fix_batch).toEqual({
      nodeId: "fix_batch",
      itemIndex: 3,
      generation: 2,
      attempt: 2,
      nextAttemptAt: "2026-08-03T14:33:41Z",
      failureClass: "transport",
    });
    // A resumed node is no longer counting down to that attempt.
    state = applyLoopEventFrame(state, frame("node_resumed", { node_id: "fix_batch" }, 2));
    expect(state.retrySchedules.fix_batch).toBeUndefined();
    // Both frames still stand as history.
    expect(state.frames).toHaveLength(2);
  });

  it("Should keep one retry schedule per node rather than merging them", () => {
    let state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("node_retry_scheduled", { node_id: "fix_batch", attempt: 1 }, 1)
    );
    state = applyLoopEventFrame(
      state,
      frame("node_retry_scheduled", { node_id: "publish", attempt: 3 }, 2)
    );
    state = applyLoopEventFrame(
      state,
      frame("node_retry_scheduled", { node_id: "fix_batch", attempt: 2 }, 3)
    );
    expect(state.retrySchedules.fix_batch.attempt).toBe(2);
    expect(state.retrySchedules.publish.attempt).toBe(3);
  });

  it("Should accumulate effect deliveries without dropping structural history", () => {
    let state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("node_running", { node_id: "fix_batch" }, 1)
    );
    state = applyLoopEventFrame(
      state,
      frame(
        "effect_results",
        {
          delivery_id: "d-1",
          trigger: "on_failed",
          node_id: "fix_batch",
          outcome: "error",
          cause: "channel #ops rejected the message",
          duration_ms: 412,
        },
        2
      )
    );
    expect(state.effectResults).toEqual([
      {
        deliveryId: "d-1",
        trigger: "on_failed",
        nodeId: "fix_batch",
        outcome: "error",
        code: "",
        cause: "channel #ops rejected the message",
        durationMs: 412,
        at: state.frames[1].at,
      },
    ]);
    expect(state.frames.map(retained => retained.kind)).toEqual(["node_running", "effect_results"]);
  });

  it("Should ignore a lifecycle frame with no node identity instead of corrupting state", () => {
    const state = applyLoopEventFrame(
      emptyLoopRunLiveState(),
      frame("node_retry_scheduled", { attempt: 2 }, 1)
    );
    expect(state.retrySchedules).toEqual({});
    // The frame still retains — the operator can see it arrived.
    expect(state.frames).toHaveLength(1);
  });
});

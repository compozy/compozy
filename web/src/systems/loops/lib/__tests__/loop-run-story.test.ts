import { describe, expect, it } from "vitest";

import type { LoopDefinition } from "../../types";
import { readLoopGraph } from "../loop-graph";
import { buildNextNote, buildRunStory, humanizeLoopNodeId } from "../loop-run-story";
import { loopEventFrame as frame } from "./loop-event-frame";

function watchGraph() {
  return readLoopGraph({
    graph: {
      nodes: [
        { id: "watch_pr", class: "source", kind: "watch-events", watch: { poll: "30s" } },
        { id: "inspect_events", class: "action", kind: "run-agent" },
        {
          id: "fix_batches",
          class: "control",
          kind: "fan-out",
          batch_size: 10,
          max_parallel: 1,
          max_fan_out: 64,
        },
        { id: "check_all", class: "control", kind: "gate", verdict_policy: "revise_until_clean" },
        { id: "archive_event", class: "action", kind: "run-agent" },
      ],
      edges: [
        { from: "watch_pr", to: "inspect_events" },
        { from: "inspect_events", to: "fix_batches" },
        { from: "fix_batches", to: "check_all" },
        { from: "check_all", to: "archive_event" },
      ],
    },
  } as unknown as Pick<LoopDefinition, "graph">);
}

describe("humanizeLoopNodeId", () => {
  it("Should collapse mixed separators, trim edges, and space snake and kebab ids", () => {
    expect(humanizeLoopNodeId("fix_batches")).toBe("fix batches");
    expect(humanizeLoopNodeId("archive-event")).toBe("archive event");
    // Consecutive and mixed separators collapse to a single space.
    expect(humanizeLoopNodeId("fix__batches")).toBe("fix batches");
    expect(humanizeLoopNodeId("resolve-_review__threads")).toBe("resolve review threads");
    // Leading and trailing separators are trimmed, not rendered as gaps.
    expect(humanizeLoopNodeId("_watch_pr_")).toBe("watch pr");
  });
});

describe("buildRunStory", () => {
  it("Should tell the wake → round → work story newest-first without operator vocabulary", () => {
    const frames = [
      frame(
        "status_changed",
        { from: "watching", to: "running", status: "running", cause: "watch_events" },
        1
      ),
      frame("generation_started", { generation: 1, reattempt_strategy: "failed_only" }, 2),
      frame("node_succeeded", { node_id: "inspect_events", generation: 1 }, 3),
    ];
    const story = buildRunStory(frames, {
      status: "running",
      reattemptStrategy: "failed_only",
      graph: watchGraph(),
    });
    expect(story.rows.map(row => row.title)).toEqual([
      "Finished inspect events",
      "Round 1 started",
      "A new event woke the run",
    ]);
    expect(story.rows[2].micro).toBe("status_changed · watching → running");
    expect(story.rows[1].micro).toBe("generation_started · gen 1");
  });

  it("Should collapse consecutive branch successes into one `k of n clean` row", () => {
    const frames = [
      frame("node_running", { node_id: "fix_batches", generation: 1, item_index: 1 }, 1),
      frame("node_succeeded", { node_id: "fix_batches", generation: 1, item_index: 1 }, 2),
      frame("node_succeeded", { node_id: "fix_batches", generation: 1, item_index: 2 }, 3),
      frame(
        "node_failed",
        {
          node_id: "fix_batches",
          generation: 1,
          item_index: 3,
          output_ref: JSON.stringify({
            kind: "action_failure",
            code: "incomplete_batch",
            cause: "2 of its 4 comments weren't fully handled.",
            recovery: "Re-run the failed group.",
          }),
        },
        4
      ),
    ];
    const story = buildRunStory(frames, {
      status: "running",
      reattemptStrategy: "failed_only",
      graph: watchGraph(),
    });
    const [failedRow, successRow] = story.rows;
    expect(successRow.title).toBe("Fix batches — 2 of 3 clean");
    expect(successRow.micro).toBe("node_succeeded · fix_batches[1–2]");
    expect(failedRow.title).toBe("Fix batches came up short");
    expect(failedRow.tone).toBe("danger");
    expect(failedRow.sub).toContain("2 of its 4 comments weren't fully handled.");
    expect(failedRow.sub).toContain("queued to be redone");
    expect(failedRow.micro).toBe("node_failed · fix_batches[3]");
  });

  it("Should accent only the latest round while running and never once parked", () => {
    const frames = [
      frame("generation_started", { generation: 1, reattempt_strategy: "failed_only" }, 1),
      frame("generation_started", { generation: 2, reattempt_strategy: "failed_only" }, 2),
    ];
    const running = buildRunStory(frames, { status: "running", graph: null });
    expect(running.rows[0].tone).toBe("accent");
    expect(running.rows[1].tone).toBe("neutral");
    const parked = buildRunStory(frames, { status: "needs-approval", graph: null });
    expect(parked.rows.map(row => row.tone)).toEqual(["neutral", "neutral"]);
  });

  it("Should render one round marker when the runtime re-emits a generation start", () => {
    const frames = [
      frame("generation_started", { generation: 1, parent_generation: 0, origin: "initial" }, 1),
      frame("generation_started", { generation: 1, parent_generation: 0, origin: "initial" }, 2),
    ];

    const story = buildRunStory(frames, { status: "running", graph: null });
    expect(story.rows.filter(row => row.kind === "generation_started")).toHaveLength(1);
  });

  it("Should render verdict rows with scores, best state, and mono blocking issues", () => {
    const frames = [
      frame(
        "gate_verdict",
        {
          node_id: "check_all",
          generation: 2,
          verdict: "revise",
          score: 0.91,
          best_generation: 2,
          blocking_issues: [
            { id: "issue_022", note: "no decision" },
            { id: "issue_024", note: "fix missing" },
          ],
        },
        1
      ),
      frame(
        "gate_verdict",
        {
          node_id: "check_all",
          generation: 3,
          verdict: "pass",
          score: 0.94,
          best_generation: 3,
        },
        2
      ),
    ];
    const story = buildRunStory(frames, { status: "running", graph: watchGraph() });
    const [pass, revise] = story.rows;
    expect(revise.title).toBe("Check: not clean yet");
    expect(revise.sub).toBe("Verdict revise.");
    expect(revise.score).toBe(0.91);
    expect(revise.isBest).toBe(true);
    expect(revise.issues).toEqual([
      { id: "issue_022", note: "no decision" },
      { id: "issue_024", note: "fix missing" },
    ]);
    expect(pass.title).toBe("Check passed — everything handled");
    expect(pass.sub).toBe("Verdict pass");
    expect(pass.score).toBe(0.94);
    expect(pass.isBest).toBe(true);
  });

  it("Should project persisted score, best, and restore provenance onto a generation", () => {
    const story = buildRunStory(
      [
        frame(
          "generation_started",
          {
            generation: 3,
            parent_generation: 1,
            origin: "ratchet_restore",
            reattempt_strategy: "all",
          },
          1
        ),
        frame(
          "gate_verdict",
          {
            node_id: "quality",
            gate_id: "quality",
            generation: 3,
            verdict: "revise",
            score: 0.7,
            best_generation: 3,
          },
          2
        ),
      ],
      {
        status: "exhausted",
        graph: null,
        bestGeneration: 3,
        generations: [
          {
            generation: 3,
            parent_generation: 1,
            origin: "ratchet_restore",
            verdicts: [
              {
                gate_id: "quality",
                item_index: 0,
                outcome: "rejected",
                score: 0.7,
                criteria: [],
                blocking_issues: [],
              },
            ],
            outputs: [],
          },
        ],
      }
    );

    expect(story.rows[0]).toMatchObject({
      kind: "gate_verdict",
      score: undefined,
      isBest: false,
    });
    expect(story.rows[1]).toMatchObject({
      generation: 3,
      score: 0.7,
      isBest: true,
      originLabel: "Restored from gen 1",
    });
  });

  it("Should keep durable generation markers when their start events were pruned", () => {
    const story = buildRunStory([], {
      status: "exhausted",
      graph: null,
      bestGeneration: 2,
      generations: [
        {
          generation: 1,
          parent_generation: 0,
          origin: "initial",
          verdicts: [
            {
              gate_id: "quality",
              item_index: 0,
              outcome: "rejected",
              score: 0.7,
              criteria: [],
              blocking_issues: [],
            },
          ],
          outputs: [],
        },
        {
          generation: 2,
          parent_generation: 1,
          origin: "ratchet_restore",
          verdicts: [
            {
              gate_id: "quality",
              item_index: 0,
              outcome: "rejected",
              score: 0.8,
              criteria: [],
              blocking_issues: [],
            },
          ],
          outputs: [],
        },
      ],
    });

    expect(story.rows).toMatchObject([
      {
        kind: "generation_started",
        generation: 2,
        score: 0.8,
        isBest: true,
        originLabel: "Restored from gen 1",
      },
      {
        kind: "generation_started",
        generation: 1,
        score: 0.7,
        isBest: false,
        originLabel: "Initial generation",
      },
    ]);
  });

  it("Should prefer a streamed best generation until the durable detail catches up", () => {
    const story = buildRunStory(
      [
        frame(
          "gate_verdict",
          {
            node_id: "quality",
            gate_id: "quality",
            generation: 2,
            verdict: "pass",
            score: 0.9,
            best_generation: 2,
          },
          10
        ),
      ],
      {
        status: "running",
        graph: null,
        bestGeneration: 1,
        generations: [
          {
            generation: 1,
            parent_generation: 0,
            origin: "initial",
            verdicts: [
              {
                gate_id: "quality",
                item_index: 0,
                outcome: "rejected",
                score: 0.7,
                criteria: [],
                blocking_issues: [],
              },
            ],
            outputs: [],
          },
        ],
      }
    );

    expect(story.rows[0]).toMatchObject({
      kind: "gate_verdict",
      generation: 2,
      isBest: true,
    });
    expect(story.rows[1]).toMatchObject({
      kind: "generation_started",
      generation: 1,
      isBest: false,
    });
  });

  it("Should feed Happening now from node_running with the task link, never a story row", () => {
    const frames = [
      frame(
        "node_running",
        {
          node_id: "fix_batches",
          generation: 2,
          item_index: 3,
          task_id: "task_9",
          task_run_id: "tr_9",
        },
        1
      ),
    ];
    const story = buildRunStory(frames, { status: "running", graph: watchGraph() });
    expect(story.rows).toHaveLength(0);
    expect(story.now).toMatchObject({
      nodeId: "fix_batches",
      label: "fix batches",
      generation: 2,
      taskLink: { taskId: "task_9", taskRunId: "tr_9" },
    });
  });

  it("Should clear Happening now on the node's terminal event and on any park", () => {
    const running = [
      frame("node_running", { node_id: "fix_batches", generation: 1, item_index: 0 }, 1),
    ];
    const done = [
      ...running,
      frame("node_succeeded", { node_id: "fix_batches", generation: 1, item_index: 0 }, 2),
    ];
    const paused = [
      ...running,
      frame(
        "status_changed",
        { from: "running", to: "paused", status: "paused", cause: "pause_boundary" },
        2
      ),
    ];
    expect(buildRunStory(done, { status: "running", graph: null }).now).toBeNull();
    expect(buildRunStory(paused, { status: "paused", graph: null }).now).toBeNull();
  });

  it("Should link an awaiting_child node's Happening now to the child run", () => {
    const frames = [frame("node_running", { node_id: "child_delivery", generation: 2 }, 1)];
    const story = buildRunStory(frames, {
      status: "running",
      graph: null,
      generations: [
        {
          generation: 2,
          parent_generation: 1,
          origin: "gate_revise",
          verdicts: [],
          outputs: [
            {
              node_id: "child_delivery",
              status: "awaiting_child",
              child_loop_run_id: "looprun_child",
            },
          ],
        },
      ],
    });
    expect(story.now?.childRunId).toBe("looprun_child");
  });

  it("Should narrate terminal transitions from the status payload", () => {
    const frames = [
      frame(
        "status_changed",
        {
          from: "running",
          to: "failed",
          status: "failed",
          cause: "coordinator_failure",
          failure: {
            kind: "coordinator_failure",
            code: "coordinator_failed",
            cause: "The Loop coordinator failed before it could settle the run.",
            recovery:
              "Inspect daemon logs for the correlated coordinator run, then start a new run.",
          },
        },
        1
      ),
    ];
    const story = buildRunStory(frames, { status: "failed", graph: null });
    expect(story.rows[0]).toMatchObject({
      title: "Run failed",
      tone: "danger",
      sub: "The Loop coordinator failed before it could settle the run.",
      micro: "status_changed · running → failed",
    });
    expect(story.now).toBeNull();
  });

  it("Should narrate a canceled run and a killed run from the terminal cause", () => {
    const canceled = buildRunStory(
      [
        frame(
          "status_changed",
          { from: "running", to: "canceled", status: "canceled", cause: "operator_cancel" },
          1
        ),
      ],
      { status: "canceled", graph: null }
    );
    expect(canceled.rows[0]).toMatchObject({
      title: "Run canceled",
      tone: "neutral",
      icon: "circle-slash",
      micro: "status_changed · running → canceled",
    });
    const killed = buildRunStory(
      [
        frame(
          "status_changed",
          { from: "running", to: "canceled", status: "canceled", cause: "operator_kill" },
          1
        ),
      ],
      { status: "canceled", graph: null }
    );
    expect(killed.rows[0]).toMatchObject({
      title: "Run killed",
      tone: "danger",
      icon: "killed",
    });
  });
});

describe("buildNextNote", () => {
  it("Should phrase the remaining downstream nodes plus the watch clause", () => {
    const note = buildNextNote(watchGraph(), [
      {
        generation: 2,
        parent_generation: 1,
        origin: "gate_revise",
        verdicts: [],
        outputs: [
          { node_id: "inspect_events", status: "succeeded" },
          { node_id: "fix_batches", status: "running", item_index: 1 },
          { node_id: "check_all", status: "pending" },
          { node_id: "archive_event", status: "pending" },
        ],
      },
    ]);
    expect(note).toBe(
      "Still ahead this round: check all, then archive event. After that, the run goes back to " +
        "watching. If the next check comes back clean, it finishes as Done; if new work arrives, " +
        "a new round starts automatically."
    );
  });

  it("Should fall back to the final-check clause when the graph has no watch source", () => {
    const graph = readLoopGraph({
      graph: {
        nodes: [
          { id: "implement", class: "action", kind: "run-agent" },
          { id: "review", class: "control", kind: "gate", verdict_policy: "revise_until_clean" },
        ],
        edges: [{ from: "implement", to: "review" }],
      },
    } as unknown as Pick<LoopDefinition, "graph">);
    const note = buildNextNote(graph, [
      {
        generation: 1,
        parent_generation: 0,
        origin: "initial",
        verdicts: [],
        outputs: [{ node_id: "implement", status: "succeeded" }],
      },
    ]);
    expect(note).toBe(
      "Still ahead this round: review. When every step is done, a final check decides: clean " +
        "finishes the run as Done; anything left starts another round."
    );
  });

  it("Should return null without a graph", () => {
    expect(buildNextNote(null, [])).toBeNull();
  });
});

// WT-002: every lifecycle kind becomes a readable beat, and every clause in it
// traces to a payload field. The failure this guards against is copy that
// asserts something the payload never said.
describe("lifecycle story rows", () => {
  function storyFor(frames: Parameters<typeof buildRunStory>[0]) {
    return buildRunStory(frames, { status: "running", graph: null });
  }

  function rowFor(kind: Parameters<typeof frame>[0], payload: unknown) {
    const story = storyFor([frame(kind, payload, 1)]);
    expect(story.rows).toHaveLength(1);
    return story.rows[0];
  }

  it("Should phrase a scheduled retry with its attempt and failure class", () => {
    const row = rowFor("node_retry_scheduled", {
      node_id: "fix_batches",
      generation: 2,
      item_index: 3,
      attempt: 2,
      failure_class: "transport",
      next_attempt_at: "2026-08-03T14:33:41Z",
    });
    expect(row.title).toBe("Retry scheduled for fix batches");
    expect(row.sub).toContain("Attempt 2 is queued.");
    expect(row.sub).toContain("transport-class");
    expect(row.micro).toBe(
      "node_retry_scheduled · fix_batches[3] · gen 2 · next 2026-08-03T14:33:41Z"
    );
    expect(row.tone).toBe("warning");
  });

  it("Should attribute a pause to its actor and reason, and a rule when one drove it", () => {
    const manual = rowFor("node_paused", {
      node_id: "fix_batches",
      generation: 2,
      actor_kind: "user",
      actor_id: "pedro",
      reason: "hold this one",
      mode: "drain",
    });
    expect(manual.title).toBe("Fix batches paused");
    expect(manual.sub).toContain("Parked by you (pedro)");
    expect(manual.sub).toContain("“hold this one”");
    expect(manual.sub).toContain("finished cleanly before parking");

    const byRule = rowFor("node_paused", {
      node_id: "fix_batches",
      rule_id: "retry_storm",
      actor_kind: "system",
    });
    expect(byRule.sub).toContain("Parked by rule retry_storm");
  });

  it("Should name the resume semantics that were actually applied", () => {
    expect(rowFor("node_resumed", { node_id: "fix_batches", mode: "reset_attempts" }).sub).toBe(
      "Attempt counter restarted; the retry delay was cleared."
    );
    expect(rowFor("node_resumed", { node_id: "fix_batches", mode: "immediate" }).sub).toContain(
      "retry delay was skipped"
    );
  });

  it("Should separate a cooperative cancel from a kill", () => {
    const canceled = rowFor("node_canceled", { node_id: "fix_batches", actor_id: "pedro" });
    expect(canceled.title).toBe("Fix batches canceled");
    expect(canceled.sub).toContain("requested → delivering → draining → canceled");
    expect(canceled.tone).toBe("neutral");

    const killed = rowFor("node_killed", { node_id: "fix_batches", actor_id: "pedro" });
    expect(killed.title).toBe("Fix batches killed");
    expect(killed.sub).toContain("reactions were skipped");
    expect(killed.tone).toBe("danger");
  });

  it("Should report a quarantine with its attempt count and keep the run's continuation clear", () => {
    const row = rowFor("node_quarantined", {
      node_id: "fix_batches",
      generation: 2,
      attempt: 4,
      failure: { cause: "GitHub rejected the credential." },
    });
    expect(row.title).toBe("Fix batches was set aside after 4 attempts");
    expect(row.sub).toContain("GitHub rejected the credential. — Its clock is suspended");
    expect(row.sub).toContain("the rest of the run keeps going");
  });

  it("Should record a requeue with its actor and the caps that still apply", () => {
    const row = rowFor("node_requeued", { node_id: "fix_batches", actor_id: "pedro" });
    expect(row.title).toBe("Fix batches requeued");
    expect(row.sub).toContain("All caps and budgets still apply.");
  });

  it("Should phrase each wait kind from its own wait_kind", () => {
    expect(
      rowFor("node_wait_started", { node_id: "publish", wait_kind: "timer", resume_at: "15:10" })
        .sub
    ).toContain("Sleeping until its timer is due — resumes 15:10");
    expect(
      rowFor("node_wait_started", { node_id: "publish", wait_kind: "approval_escalation" }).sub
    ).toContain("Waiting for your decision");
    expect(rowFor("node_wait_resumed", { node_id: "publish", actor_id: "ana" }).sub).toContain(
      "Resolved by ana"
    );
  });

  it("Should read an attention flag from either emitter's field and say it self-clears", () => {
    // The liveness path writes the flag into `reason`, the escalation path into
    // `attention_flag`; both must produce the same beat.
    const fromReason = rowFor("node_attention_flagged", { node_id: "task_04", reason: "silence" });
    const fromFlag = rowFor("node_attention_flagged", {
      node_id: "task_04",
      attention_flag: "silence",
    });
    expect(fromReason.title).toBe("Flagged: task 04 is silent for a while");
    expect(fromFlag.title).toBe(fromReason.title);
    expect(fromReason.icon).toBe("attention-silence");
    expect(fromFlag.icon).toBe("attention-silence");
    expect(
      rowFor("node_attention_flagged", {
        node_id: "task_04",
        attention_flag: "dependency_quarantined",
      }).icon
    ).toBe("attention");
    expect(fromFlag.sub).toContain("clears itself on any evidence of life");
    expect(rowFor("node_attention_cleared", { node_id: "task_04" }).tone).toBe("success");
  });

  it("Should keep an effect delivery clearly separate from the node's own outcome", () => {
    const failed = rowFor("effect_results", {
      trigger: "on_failed",
      outcome: "error",
      cause: "channel #ops rejected the message.",
    });
    expect(failed.title).toBe("The on_failed reaction couldn't be delivered");
    expect(failed.sub).toContain("reactions never touch the step itself");
    expect(rowFor("effect_results", { trigger: "on_failed", outcome: "ok" }).tone).toBe("neutral");
  });

  it("Should report suppressed duplicates and breaker transitions", () => {
    const suppressed = rowFor("duplicate_suppressed", {
      event_key: "pr-4182",
      suppressed_count: 2,
    });
    expect(suppressed.title).toBe("2 duplicate deliveries were suppressed");
    expect(suppressed.sub).toContain("no second run started");

    const opened = rowFor("target_breaker_transition", { target: "github-mcp", state: "open" });
    expect(opened.title).toBe("Paused calls to github-mcp");
    expect(opened.tone).toBe("warning");
    const closed = rowFor("target_breaker_transition", { target: "github-mcp", state: "closed" });
    expect(closed.title).toBe("Calls to github-mcp resumed");
    expect(closed.tone).toBe("success");
  });

  it("Should clear a node from Happening now once a lifecycle beat ends its attempt", () => {
    const story = storyFor([
      frame("node_running", { node_id: "fix_batches", item_index: 3, generation: 2 }, 1),
      frame("node_paused", { node_id: "fix_batches", item_index: 3, generation: 2 }, 2),
    ]);
    expect(story.now).toBeNull();
  });

  it("Should omit a clause the payload never carried instead of inventing one", () => {
    const row = rowFor("node_retry_scheduled", { node_id: "fix_batches" });
    expect(row.sub).toBeUndefined();
    expect(row.micro).toBe("node_retry_scheduled · fix_batches");
  });
});

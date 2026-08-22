import {
  createFrameFactory,
  generationsFor,
  genericWatchDefinition,
  genericWatchRun,
  minutesAgo,
  nodePayload,
  reviewAndFixDefinition,
  reviewAndFixRun,
  roundOneFrames,
} from "./loop-run-page-fixture-world";
import { briefingFor } from "./loop-run-read-builders";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";

/**
 * Production-derived run-page scenarios. Review states use the bundled
 * agent-authored Loop; watch-only states use a separate generic watch Loop.
 * The scored `quality-ratchet` states live in `loop-run-metric-fixtures`.
 * The scenario shape lives in `loop-run-scenario-types`, and the projection into
 * page props in `loop-run-scenario-props`.
 *
 * Every scenario states its own served verdict through `briefingFor`, which
 * copies the run's server-owned status, progress and usage so the two reads
 * cannot disagree. Nothing fills in a missing one.
 */

export type { LoopRunStoryScenario } from "./loop-run-scenario-types";

export function runningScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame),
    frame("generation_started", 4, {
      generation: 2,
      parent_generation: 1,
      origin: "gate_revise",
      reattempt_strategy: "failed_only",
    }),
    frame(
      "node_running",
      4,
      nodePayload("fix_batch", 2, {
        item_index: 3,
        task_id: "task_fix",
        task_run_id: "tr_204",
      })
    ),
    frame("token_tick", 3, { tokens_used: 68_000 }),
  ];
  const run = reviewAndFixRun();
  return {
    run,
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "Fixing the third finding of round 2",
      detail: "Nothing needs you. One of two steps is done in round 2.",
    }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("running"),
  };
}

export function needsApprovalScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 20),
    frame("generation_started", 6, {
      generation: 3,
      parent_generation: 2,
      origin: "gate_revise",
      reattempt_strategy: "failed_only",
    }),
    frame("needs_approval", 2, {
      gate_id: "tool_policy",
      title: "Approve writing review artifacts?",
      generation: 3,
      facts: [
        { label: "Tool", value: "write_review_artifacts" },
        { label: "Task", value: "billing-webhooks" },
        { label: "Round", value: "2" },
      ],
    }),
    frame("status_changed", 2, {
      from: "running",
      to: "needs-approval",
      status: "needs-approval",
      cause: "tool_policy",
    }),
  ];
  const run = reviewAndFixRun({
    status: "needs-approval",
    generation: 3,
    progress: { round: 3, steps_done: 1, steps_total: 2 },
    tokens_used: 92_000,
    created_at: minutesAgo(46),
    started_at: minutesAgo(46),
    last_progress_at: minutesAgo(1),
    active_gate_id: "tool_policy",
  });
  return {
    run,
    // An open gate is a blocker the daemon always emits alongside the status,
    // so the strip leads on it and points down at the card that owns the answer.
    briefing: briefingFor(run, {
      tone: "needs_you",
      headline: 'The gate "tool_policy" is waiting for your decision',
      detail: "Nothing else moves until you approve or reject writing the review artifacts.",
      blockers: [
        {
          kind: "approval",
          gate_id: "tool_policy",
          waiting_since: minutesAgo(2),
          unblocker: `compozy loop approve ${run.id} --gate tool_policy`,
        },
      ],
    }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("pending"),
  };
}

export function watchingScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory("r-watch-01");
  const frames = [
    frame("generation_started", 20, {
      generation: 4,
      parent_generation: 3,
      origin: "reattempt",
      reattempt_strategy: "failed_only",
    }),
    frame("node_succeeded", 19, nodePayload("watch_inbox", 4)),
    frame("node_succeeded", 18, nodePayload("handle_event", 4)),
    frame("status_changed", 16, {
      from: "running",
      to: "watching",
      status: "watching",
      cause: "contract",
    }),
  ];
  const run = genericWatchRun({
    created_at: minutesAgo(38),
    started_at: minutesAgo(38),
    last_progress_at: minutesAgo(16),
    // `watch_inbox` is a source node, so the round holds exactly one action step.
    progress: { round: 4, steps_done: 1, steps_total: 1 },
  });
  return {
    run,
    // Dormant is the resting state of a watch loop, not a stall: calm tone, and
    // the sentence says which of the two it is (US-005.EC-2).
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "Watching for the next inbox event",
      detail:
        "Nothing has arrived for 16m. This is the resting state, not a stall — the run wakes when the next event lands.",
    }),
    definition: genericWatchDefinition,
    frames,
    generations: [
      {
        generation: 4,
        parent_generation: 3,
        origin: "reattempt",
        route_causes: [],
        verdicts: [],
        outputs: [
          { node_id: "watch_inbox", status: "succeeded", generation: 4 },
          { node_id: "handle_event", status: "succeeded", generation: 4 },
        ],
      },
    ],
  };
}

export function pausedScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 5),
    frame("generation_started", 8, {
      generation: 2,
      parent_generation: 1,
      origin: "gate_revise",
      reattempt_strategy: "failed_only",
    }),
    frame("status_changed", 3, {
      from: "running",
      to: "paused",
      status: "paused",
      cause: "pause_boundary",
    }),
  ];
  const run = reviewAndFixRun({
    status: "paused",
    tokens_used: 74_000,
    created_at: minutesAgo(29),
    started_at: minutesAgo(29),
    last_progress_at: minutesAgo(3),
  });
  return {
    run,
    // A pause someone asked for is calm. Nothing is wrong, and nothing is owed.
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "Paused at a round boundary",
      detail: "Round 2 is held where it stands and resumes exactly there.",
    }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("pending"),
  };
}

export function failedScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 61),
    frame("generation_started", 62, {
      generation: 2,
      parent_generation: 1,
      origin: "gate_revise",
      reattempt_strategy: "failed_only",
    }),
    frame(
      "node_running",
      61,
      nodePayload("fix_batch", 2, { item_index: 3, task_id: "task_fix", task_run_id: "tr_204" })
    ),
    frame("status_changed", 60, {
      from: "running",
      to: "failed",
      status: "failed",
      cause: "coordinator_failure",
      failure: {
        kind: "action_failure",
        code: "invalid_output",
        cause: "The fixer result did not include one entry per issue file.",
        recovery: "Correct the fixer output and start a new run.",
      },
    }),
  ];
  const run = reviewAndFixRun({
    status: "failed",
    tokens_used: 81_000,
    created_at: minutesAgo(87),
    started_at: minutesAgo(87),
    last_progress_at: minutesAgo(60),
  });
  return {
    run,
    briefing: briefingFor(run, {
      tone: "failed",
      headline: "The fixer's result did not cover every issue file",
      detail: "Round 2 stopped there, and nothing downstream started.",
      outcome: { status: "failed", cause: "invalid_output", at: minutesAgo(60) },
    }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("failed"),
  };
}

export function noOpScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory("r-watch-01");
  const frames = [
    frame("status_changed", 50, {
      from: "watching",
      to: "running",
      status: "running",
      cause: "watch_poll",
    }),
    frame("node_succeeded", 50, nodePayload("watch_inbox", 5)),
    frame("status_changed", 49, {
      from: "running",
      to: "no-op",
      status: "no-op",
      cause: "contract",
    }),
  ];
  const run = genericWatchRun({
    status: "no-op",
    generation: 5,
    tokens_used: 21_000,
    created_at: minutesAgo(51),
    started_at: minutesAgo(51),
    last_progress_at: minutesAgo(49),
    // It woke, found nothing to act on, and settled without executing a step.
    progress: { round: 5, steps_done: 0, steps_total: 0 },
  });
  return {
    run,
    // No artifacts, and none implied: the empty list is the truthful statement
    // that this run produced nothing (US-008.EC-1).
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "Nothing to do — no matching event arrived",
      detail: "The run woke on the poll, found no ready inbox event and settled.",
      outcome: { status: "no-op", cause: "no_work", at: minutesAgo(49) },
      artifacts: [],
    }),
    definition: genericWatchDefinition,
    frames,
    generations: [],
  };
}

export { buildScenarioProps } from "./loop-run-scenario-props";

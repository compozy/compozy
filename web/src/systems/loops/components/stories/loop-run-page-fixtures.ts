import {
  createFrameFactory,
  generationsFor,
  genericWatchDefinition,
  genericWatchRun,
  metricRatchetDefinition,
  metricRatchetRun,
  minutesAgo,
  nodePayload,
  reviewAndFixDefinition,
  reviewAndFixRun,
  roundOneFrames,
} from "./loop-run-page-fixture-world";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";
import type { LoopRunGeneration } from "../../types";

/**
 * Production-derived run-page scenarios. Review states use the bundled
 * agent-authored Loop; watch-only states use a separate generic watch Loop.
 * The scenario shape lives in `loop-run-scenario-types`, and the projection into
 * page props in `loop-run-scenario-props`.
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
  return {
    run: reviewAndFixRun(),
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
  return {
    run: reviewAndFixRun({
      status: "needs-approval",
      generation: 3,
      tokens_used: 92_000,
      created_at: minutesAgo(46),
      started_at: minutesAgo(46),
      last_progress_at: minutesAgo(1),
      active_gate_id: "tool_policy",
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
  return {
    run: genericWatchRun({
      created_at: minutesAgo(38),
      started_at: minutesAgo(38),
      last_progress_at: minutesAgo(16),
    }),
    definition: genericWatchDefinition,
    frames,
    generations: [
      {
        generation: 4,
        parent_generation: 3,
        origin: "reattempt",
        verdicts: [],
        outputs: [
          { node_id: "watch_inbox", status: "succeeded", generation: 4 },
          { node_id: "handle_event", status: "succeeded", generation: 4 },
        ],
      },
    ],
    watchEvents: {
      subscriptions: [{ kind: "event.post_record", filter: "payload.inbox == input.inbox" }],
      cursors: { workspace_events: 4_182 },
      last_wake_at: minutesAgo(16),
    },
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
  return {
    run: reviewAndFixRun({
      status: "paused",
      tokens_used: 74_000,
      created_at: minutesAgo(29),
      started_at: minutesAgo(29),
      last_progress_at: minutesAgo(3),
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
        code: "action_schema_invalid",
        cause: "The fixer result did not include one entry per issue file.",
        recovery: "Correct the fixer output and start a new run.",
      },
    }),
  ];
  return {
    run: reviewAndFixRun({
      status: "failed",
      tokens_used: 81_000,
      created_at: minutesAgo(87),
      started_at: minutesAgo(87),
      last_progress_at: minutesAgo(60),
    }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("failed"),
  };
}

function metricGeneration(
  generation: number,
  parentGeneration: number,
  origin: LoopRunGeneration["origin"],
  score?: number
): LoopRunGeneration {
  return {
    generation,
    parent_generation: parentGeneration,
    origin,
    verdicts:
      score === undefined
        ? []
        : [
            {
              gate_id: "quality",
              item_index: 0,
              outcome: "approved",
              score,
              route_cause_rank: 0,
              criteria: [],
              blocking_issues: [],
            },
          ],
    outputs: [{ node_id: "draft", status: "succeeded", generation }],
  };
}

function metricVerdictPayload(generation: number, score: number, bestGeneration: number) {
  return {
    node_id: "quality",
    gate_id: "quality",
    generation,
    verdict: "pass",
    score,
    best_generation: bestGeneration,
    criteria: [{ id: "quality_score", type: "agent-judge", status: "pass", score }],
    blocking_issues: [],
    route: "next_generation",
  };
}

export function scoredBestScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory("r-score-best");
  const frames = [
    frame("generation_started", 12, {
      generation: 1,
      parent_generation: 0,
      origin: "initial",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 10, metricVerdictPayload(1, 0.72, 1)),
    frame("generation_started", 8, {
      generation: 2,
      parent_generation: 1,
      origin: "gate_next_generation",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 5, metricVerdictPayload(2, 0.96, 2)),
    frame("status_changed", 4, {
      from: "running",
      to: "done",
      status: "done",
      cause: "stop_when",
    }),
  ];
  return {
    run: metricRatchetRun({
      id: "r-score-best",
      status: "done",
      generation: 2,
      best_generation: 2,
      best_score: 0.96,
    }),
    definition: metricRatchetDefinition,
    frames,
    generations: [
      metricGeneration(1, 0, "initial", 0.72),
      metricGeneration(2, 1, "gate_next_generation", 0.96),
    ],
  };
}

export function ratchetRestoreScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory("r-ratchet-restore");
  const frames = [
    frame("generation_started", 18, {
      generation: 1,
      parent_generation: 0,
      origin: "initial",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 16, metricVerdictPayload(1, 0.8, 1)),
    frame("generation_started", 13, {
      generation: 2,
      parent_generation: 1,
      origin: "gate_next_generation",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 10, metricVerdictPayload(2, 0.6, 1)),
    frame("generation_started", 7, {
      generation: 3,
      parent_generation: 1,
      origin: "ratchet_restore",
      reattempt_strategy: "full_body",
    }),
    frame("node_running", 6, nodePayload("draft", 3)),
  ];
  return {
    run: metricRatchetRun({
      id: "r-ratchet-restore",
      status: "running",
      generation: 3,
      best_generation: 1,
      best_score: 0.8,
    }),
    definition: metricRatchetDefinition,
    frames,
    generations: [
      metricGeneration(1, 0, "initial", 0.8),
      metricGeneration(2, 1, "gate_next_generation", 0.6),
      metricGeneration(3, 1, "ratchet_restore"),
    ],
  };
}

export function exhaustedScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    frame("generation_started", 50, {
      generation: 1,
      parent_generation: 0,
      origin: "initial",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 44, metricVerdictPayload(1, 0.7, 1)),
    frame("generation_started", 40, {
      generation: 2,
      parent_generation: 1,
      origin: "gate_next_generation",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 36, metricVerdictPayload(2, 0.6, 1)),
    frame("generation_started", 34, {
      generation: 3,
      parent_generation: 1,
      origin: "ratchet_restore",
      reattempt_strategy: "full_body",
    }),
    frame("gate_verdict", 31, metricVerdictPayload(3, 0.5, 1)),
    frame("status_changed", 30, {
      from: "running",
      to: "exhausted",
      status: "exhausted",
      cause: "iteration_cap",
    }),
  ];
  return {
    run: metricRatchetRun({
      id: "r-best-exhausted",
      status: "exhausted",
      generation: 3,
      best_generation: 1,
      best_score: 0.7,
      tokens_used: 144_000,
      created_at: minutesAgo(80),
      started_at: minutesAgo(80),
      last_progress_at: minutesAgo(30),
    }),
    definition: metricRatchetDefinition,
    frames,
    generations: [
      metricGeneration(1, 0, "initial", 0.7),
      metricGeneration(2, 1, "gate_next_generation", 0.6),
      metricGeneration(3, 1, "ratchet_restore", 0.5),
    ],
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
  return {
    run: genericWatchRun({
      status: "no-op",
      generation: 5,
      tokens_used: 21_000,
      created_at: minutesAgo(51),
      started_at: minutesAgo(51),
      last_progress_at: minutesAgo(49),
    }),
    definition: genericWatchDefinition,
    frames,
    generations: [],
  };
}

export { buildScenarioProps } from "./loop-run-scenario-props";

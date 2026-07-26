import type { GoalTurnTimelineItem } from "../../hooks/use-goal-turns";
import {
  applyLoopEventFrame,
  emptyLoopRunLiveState,
  type LoopRunLiveState,
} from "../../lib/loop-events";
import { projectLoopRunPageView } from "../../lib/loop-run-page-view";
import { deriveCostEstimate } from "../../lib/loop-run-usage";
import type { LoopRunPageBodyProps } from "../run-page/loop-run-page-body";
import type {
  LoopRunEventFrame,
  LoopRunGeneration,
  LoopRunRecord,
  LoopWatchEventsState,
} from "../../types";
import {
  createFrameFactory,
  generationsFor,
  minutesAgo,
  nodePayload,
  REVISE_ISSUES,
  reviewsWatchDefinition,
  reviewsWatchRun,
  roundOneFrames,
  STORY_NOW,
} from "./loop-run-page-fixture-world";

/**
 * The `reviews-watch` story world mirroring the canonical prototypes
 * (`loop-run-detail.html` + states): a watch loop fixing review comments on PR
 * #128 in three fan-out groups. The data is fixture flavor; every derivation
 * below runs through the production libs, exactly like the page hook.
 */

export interface LoopRunStoryScenario {
  run: LoopRunRecord;
  frames: LoopRunEventFrame[];
  generations: LoopRunGeneration[];
  watchEvents?: LoopWatchEventsState;
  goalTurns?: GoalTurnTimelineItem[];
}

export function runningScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame),
    frame("generation_started", 4, { generation: 2, reattempt_strategy: "failed_only" }),
    frame(
      "node_running",
      4,
      nodePayload("fix_batches", 2, {
        item_index: 3,
        task_id: "task_fix",
        task_run_id: "tr_204",
      })
    ),
    frame("token_tick", 3, { tokens_used: 268_000 }),
  ];
  return { run: reviewsWatchRun(), frames, generations: generationsFor("running") };
}

export function needsApprovalScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 20),
    frame("gate_verdict", 7, {
      node_id: "check_all",
      generation: 2,
      verdict: "revise",
      confidence: 0.88,
      criteria: [
        {
          id: "all_issues_handled",
          type: "agent-judge",
          status: "revise",
          note: "still two open points",
        },
      ],
      blocking_issues: REVISE_ISSUES,
    }),
    frame("generation_started", 6, { generation: 3, reattempt_strategy: "failed_only" }),
    frame("needs_approval", 2, {
      gate_id: "budget",
      title: "Time limit reached — continue this run?",
      generation: 3,
      facts: [
        { label: "Time used", value: "45m of 45m" },
        { label: "Tokens", value: "412K / 1.5M" },
        { label: "Cost", value: `${deriveCostEstimate(412_000)} est.` },
        { label: "Round", value: "3" },
      ],
    }),
    frame("status_changed", 2, {
      from: "running",
      to: "needs-approval",
      status: "needs-approval",
      cause: "budget",
    }),
  ];
  return {
    run: reviewsWatchRun({
      status: "needs-approval",
      generation: 3,
      tokens_used: 412_000,
      created_at: minutesAgo(46),
      started_at: minutesAgo(46),
      last_progress_at: minutesAgo(1),
      active_gate_id: "budget",
    }),
    frames,
    generations: generationsFor("pending"),
  };
}

export function watchingScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 16),
    frame("generation_started", 20, { generation: 2, reattempt_strategy: "failed_only" }),
    frame(
      "node_succeeded",
      19,
      nodePayload("fix_batches", 2, { item_index: 3, task_id: "task_fix", task_run_id: "tr_204" })
    ),
    frame("gate_verdict", 18, {
      node_id: "check_all",
      generation: 2,
      verdict: "pass",
      confidence: 0.94,
      criteria: [
        {
          id: "all_issues_handled",
          type: "agent-judge",
          status: "pass",
          note: "every comment handled",
        },
      ],
      blocking_issues: [],
    }),
    frame(
      "node_succeeded",
      17,
      nodePayload("resolve_threads", 2, { task_id: "task_resolve", task_run_id: "tr_205" })
    ),
    frame(
      "node_succeeded",
      17,
      nodePayload("push_changes", 2, { task_id: "task_push", task_run_id: "tr_206" })
    ),
    frame("status_changed", 16, {
      from: "running",
      to: "watching",
      status: "watching",
      cause: "contract",
    }),
  ];
  return {
    run: reviewsWatchRun({
      status: "watching",
      tokens_used: 341_000,
      created_at: minutesAgo(38),
      started_at: minutesAgo(38),
      last_progress_at: minutesAgo(16),
    }),
    frames,
    generations: generationsFor("succeeded", "succeeded"),
    watchEvents: {
      subscriptions: [{ kind: "event.post_record", filter: "payload.pr == input.pr" }],
      cursors: { workspace_events: 4_182 },
      last_wake_at: minutesAgo(16),
    },
  };
}

export function pausedScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 5),
    frame("generation_started", 8, { generation: 2, reattempt_strategy: "failed_only" }),
    frame("status_changed", 3, {
      from: "running",
      to: "paused",
      status: "paused",
      cause: "pause_boundary",
    }),
  ];
  return {
    run: reviewsWatchRun({
      status: "paused",
      tokens_used: 296_000,
      created_at: minutesAgo(29),
      started_at: minutesAgo(29),
      last_progress_at: minutesAgo(3),
    }),
    frames,
    generations: generationsFor("pending"),
  };
}

export function failedScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 61),
    frame("generation_started", 62, { generation: 2, reattempt_strategy: "failed_only" }),
    frame(
      "node_running",
      61,
      nodePayload("fix_batches", 2, { item_index: 3, task_id: "task_fix", task_run_id: "tr_204" })
    ),
    frame("status_changed", 60, {
      from: "running",
      to: "failed",
      status: "failed",
      cause: "coordinator_failure",
      failure: {
        kind: "coordinator_failure",
        code: "watch_poll_failed",
        cause: "The watch source failed before it could produce a generation.",
        recovery:
          "Verify the Loop watch provider and workspace prerequisites, then start a new run.",
      },
    }),
  ];
  return {
    run: reviewsWatchRun({
      status: "failed",
      tokens_used: 310_000,
      created_at: minutesAgo(87),
      started_at: minutesAgo(87),
      last_progress_at: minutesAgo(60),
    }),
    frames,
    generations: generationsFor("running"),
  };
}

export function exhaustedScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 26),
    frame("status_changed", 30, {
      from: "running",
      to: "exhausted",
      status: "exhausted",
      cause: "budget",
    }),
  ];
  return {
    run: reviewsWatchRun({
      status: "exhausted",
      tokens_used: 1_500_000,
      budget_on_exceeded: "halt",
      created_at: minutesAgo(80),
      started_at: minutesAgo(80),
      last_progress_at: minutesAgo(30),
    }),
    frames,
    generations: generationsFor("pending"),
  };
}

export function noOpScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    frame("status_changed", 50, {
      from: "watching",
      to: "running",
      status: "running",
      cause: "watch_poll",
    }),
    frame("node_succeeded", 50, nodePayload("fetch_issues", 1, {})),
    frame("status_changed", 49, {
      from: "running",
      to: "no-op",
      status: "no-op",
      cause: "contract",
    }),
  ];
  return {
    run: reviewsWatchRun({
      status: "no-op",
      generation: 1,
      tokens_used: 12_000,
      created_at: minutesAgo(51),
      started_at: minutesAgo(51),
      last_progress_at: minutesAgo(49),
    }),
    frames,
    generations: [],
  };
}

/** Reduces the scenario's frames through the production reducer, like the page. */
function reduceLiveState(frames: readonly LoopRunEventFrame[]): LoopRunLiveState {
  return frames.reduce(applyLoopEventFrame, emptyLoopRunLiveState());
}

type ScenarioBodyProps = Omit<LoopRunPageBodyProps, "inspect">;

/**
 * Derives the full page-body prop set from a scenario through the same
 * `projectLoopRunPageView` the live page hook uses, then adds the fixture-only
 * chrome (workspace label, pinned version, no-op handlers). One derivation path
 * means the stories cannot drift from production.
 */
export function buildScenarioProps(scenario: LoopRunStoryScenario): ScenarioBodyProps {
  const { run, generations, watchEvents } = scenario;
  const live = reduceLiveState(scenario.frames);
  const { effectiveRun, ...view } = projectLoopRunPageView({
    run,
    generations,
    live,
    definition: reviewsWatchDefinition,
    nowMs: STORY_NOW,
  });
  return {
    ...view,
    run: effectiveRun,
    definition: reviewsWatchDefinition,
    goalTurns: scenario.goalTurns ?? [],
    watchEvents,
    frames: live.frames,
    workspaceLabel: "Home",
    versionLabel: `v${run.definition_version} · pinned`,
    onDecision: () => undefined,
    onStartNewRun: () => undefined,
  };
}

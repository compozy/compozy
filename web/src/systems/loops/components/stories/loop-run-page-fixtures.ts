import type { GoalTurnTimelineItem } from "../../hooks/use-goal-turns";
import {
  applyLoopEventFrame,
  emptyLoopRunLiveState,
  type LoopRunLiveState,
} from "../../lib/loop-events";
import { projectLoopRunPageView } from "../../lib/loop-run-page-view";
import type { LoopRunPageBodyProps } from "../run-page/loop-run-page-body";
import type {
  LoopDefinition,
  LoopRunEventFrame,
  LoopRunGeneration,
  LoopRunRecord,
  LoopWatchEventsState,
} from "../../types";
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
  STORY_NOW,
} from "./loop-run-page-fixture-world";

/**
 * Production-derived run-page scenarios. Review states use the bundled
 * agent-authored Loop; watch-only states use a separate generic watch Loop.
 */

export interface LoopRunStoryScenario {
  run: LoopRunRecord;
  definition: LoopDefinition;
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
    frame("generation_started", 6, { generation: 3, reattempt_strategy: "failed_only" }),
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
    frame("generation_started", 20, { generation: 4, reattempt_strategy: "failed_only" }),
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
    frame("generation_started", 8, { generation: 2, reattempt_strategy: "failed_only" }),
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
    frame("generation_started", 62, { generation: 2, reattempt_strategy: "failed_only" }),
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

export function exhaustedScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory();
  const frames = [
    ...roundOneFrames(frame, 26),
    frame("status_changed", 30, {
      from: "running",
      to: "exhausted",
      status: "exhausted",
      cause: "iteration_cap",
    }),
  ];
  return {
    run: reviewAndFixRun({
      status: "exhausted",
      generation: 3,
      tokens_used: 144_000,
      created_at: minutesAgo(80),
      started_at: minutesAgo(80),
      last_progress_at: minutesAgo(30),
    }),
    definition: reviewAndFixDefinition,
    frames,
    generations: generationsFor("pending"),
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

/** Reduces the scenario's frames through the production reducer, like the page. */
function reduceLiveState(frames: readonly LoopRunEventFrame[]): LoopRunLiveState {
  return frames.reduce(applyLoopEventFrame, emptyLoopRunLiveState());
}

type ScenarioBodyProps = Omit<LoopRunPageBodyProps, "inspect">;

/** Projects fixture events through the same view model used by the live page. */
export function buildScenarioProps(scenario: LoopRunStoryScenario): ScenarioBodyProps {
  const { run, definition, generations, watchEvents } = scenario;
  const live = reduceLiveState(scenario.frames);
  const { effectiveRun, ...view } = projectLoopRunPageView({
    run,
    generations,
    live,
    definition,
    nowMs: STORY_NOW,
  });
  return {
    ...view,
    run: effectiveRun,
    definition,
    goalTurns: scenario.goalTurns ?? [],
    generations,
    watchEvents,
    frames: live.frames,
    workspaceLabel: "Home",
    versionLabel: `v${run.definition_version} · pinned`,
    onDecision: () => undefined,
    onStartNewRun: () => undefined,
  };
}

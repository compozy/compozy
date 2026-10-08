import { HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import { loopEffectiveConfigFixture } from "../../mocks/fixtures-config";
import { materializeContractFixture } from "../../mocks/materialize-contract-fixture";
import type {
  LoopDefinition,
  LoopDefinitionGraph,
  LoopFanoutRollup,
  LoopRosterNode,
  LoopRunDetail,
  LoopRunRecord,
  LoopRunRosterPage,
  LoopStepProgress,
} from "../../types";
import { briefingFor, makeRosterNode as node, storyAt } from "./loop-run-read-builders";
import { reviewAndFixDefinition, reviewAndFixRun } from "./loop-run-page-fixture-world";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";

/**
 * Loops that run loops (issue #705).
 *
 * The child reads are served over MSW rather than staged as props, because the
 * page reads them the way production does — lazily, per child, from the child's
 * own detail, briefing and roster routes. Each child is somewhere different so
 * one capture shows every reading a child row has to make: moving, stuck on a
 * person, finished, and waiting on a loop of its own.
 */

export const NESTED_STORY_WORKSPACE_ID = "ws_default";

interface StoryChild {
  runId: string;
  loopName: string;
  inputs: Record<string, unknown>;
  run: Partial<LoopRunRecord>;
  /** Served by the briefing; the detail route leaves `progress` zeroed. */
  progress: LoopStepProgress;
  nodes: LoopRosterNode[];
}

const ZERO_PROGRESS: LoopStepProgress = { round: 0, steps_done: 0, steps_total: 0 };

function live(minutesAgo: number, lastProgress = 1): Partial<LoopRunRecord> {
  return {
    status: "running",
    created_at: storyAt(minutesAgo),
    started_at: storyAt(minutesAgo),
    last_progress_at: storyAt(lastProgress),
  };
}

const r1 = { generation: 1 };

const CHILDREN: StoryChild[] = [
  {
    runId: "r-8f21a0",
    loopName: "fix-one-batch",
    inputs: { batch: "api", files: 4 },
    run: live(6),
    progress: { round: 1, steps_done: 1, steps_total: 4 },
    nodes: [
      node("plan_fix", "succeeded", r1),
      node("run_tests", "running", { ...r1, started_at: storyAt(2) }),
      node("apply_fix", "pending", r1),
      node("finalize_round", "pending", r1),
    ],
  },
  {
    runId: "r-3b9c55",
    loopName: "fix-one-batch",
    inputs: { batch: "web", files: 11 },
    run: { ...live(52, 47), status: "needs-approval" },
    progress: { round: 1, steps_done: 3, steps_total: 4 },
    nodes: [
      node("plan_fix", "succeeded", r1),
      node("run_tests", "succeeded", r1),
      node("apply_fix", "succeeded", r1),
      node("finalize_round", "control_pending", { ...r1, started_at: storyAt(47) }),
    ],
  },
  {
    runId: "r-5c71e2",
    loopName: "fix-one-batch",
    inputs: { batch: "billing", files: 2 },
    run: live(9),
    progress: { round: 1, steps_done: 1, steps_total: 4 },
    nodes: [
      node("plan_fix", "succeeded", r1),
      node("deep_review", "awaiting_child", {
        ...r1,
        started_at: storyAt(8),
        child_loop_run_id: "r-9e04aa",
      }),
      node("apply_fix", "pending", r1),
      node("finalize_round", "pending", r1),
    ],
  },
  {
    runId: "r-9e04aa",
    loopName: "review-one-file",
    inputs: { file: "internal/billing/webhooks.go" },
    run: live(8),
    progress: { round: 1, steps_done: 2, steps_total: 3 },
    nodes: [
      node("read_file", "succeeded", r1),
      node("lint", "succeeded", r1),
      node("judge", "running", { ...r1, started_at: storyAt(3) }),
    ],
  },
  {
    runId: "r-d40e17",
    loopName: "fix-one-batch",
    inputs: { batch: "docs", files: 1 },
    run: {
      status: "done",
      created_at: storyAt(14),
      started_at: storyAt(14),
      last_progress_at: storyAt(5),
      completed_at: storyAt(5),
    },
    progress: { round: 1, steps_done: 4, steps_total: 4 },
    nodes: ["plan_fix", "run_tests", "apply_fix", "finalize_round"].map(id =>
      node(id, "succeeded", r1)
    ),
  },
  {
    runId: "r-wave02",
    loopName: "run-one-wave",
    inputs: { wave: 2, batches: ["api", "billing"] },
    run: live(18, 6),
    progress: { round: 1, steps_done: 1, steps_total: 3 },
    nodes: [
      node("plan_wave", "succeeded", r1),
      node("fix_batch", "awaiting_child", {
        ...r1,
        started_at: storyAt(9),
        child_loop_run_id: "r-5c71e2",
      }),
      node("close_wave", "pending", r1),
    ],
  },
];

const childById = new Map(CHILDREN.map(child => [child.runId, child]));

/** The run as the briefing describes it. */
function childRecord(child: StoryChild): LoopRunRecord {
  return reviewAndFixRun({
    id: child.runId,
    loop_name: child.loopName,
    generation: 1,
    started_by_kind: "loop",
    tokens_used: 9_400,
    inputs: child.inputs,
    progress: child.progress,
    ...child.run,
  });
}

function childDetail(child: StoryChild): LoopRunDetail {
  return {
    // The detail route does not populate `progress`; the briefing serves it. A
    // fixture that filled it here would hide a page reading the wrong route.
    run: { ...childRecord(child), progress: ZERO_PROGRESS },
    executed_definition: reviewAndFixDefinition,
    materialized_contract: materializeContractFixture(reviewAndFixDefinition.contract, {}),
    effective_config: loopEffectiveConfigFixture,
    generations: [],
    amendments: [],
    node_controls: [],
    requests: [],
    waits: [],
  };
}

function childRoster(child: StoryChild): LoopRunRosterPage {
  return {
    run_id: child.runId,
    loop_name: child.loopName,
    run_status: childRecord(child).status,
    nodes: child.nodes,
    fanout_rollups: [],
    next_cursor: "",
  };
}

const NOT_FOUND = { error: "Loop run not found" };

/** The three child reads a child row makes, answered for every staged child. */
export const nestedChildRunHandlers = [
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}/briefing", ({ params }) => {
    const child = childById.get(String(params.run_id));
    return child
      ? HttpResponse.json(
          briefingFor(childRecord(child), { tone: "ok", headline: "Working through its steps" })
        )
      : HttpResponse.json(NOT_FOUND, { status: 404 });
  }),
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}", ({ params }) => {
    const child = childById.get(String(params.run_id));
    return child
      ? HttpResponse.json(childDetail(child))
      : HttpResponse.json(NOT_FOUND, { status: 404 });
  }),
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes", ({ params }) => {
    const child = childById.get(String(params.run_id));
    return child
      ? HttpResponse.json(childRoster(child))
      : HttpResponse.json(NOT_FOUND, { status: 404 });
  }),
];

/** The parent: `fix_batch` fanned into four child runs, three still owed. */
export function nestedLoopsScenario(): LoopRunStoryScenario {
  const progress = { round: 2, steps_done: 2, steps_total: 7 };
  const run = reviewAndFixRun({ status: "running", generation: 2, progress });
  const rollup: LoopFanoutRollup = {
    generation: 2,
    node_id: "fix_batch",
    done: 1,
    total: 4,
    failed: 0,
  };
  const branches: [string, number, LoopRosterNode["state"], number][] = [
    ["r-8f21a0", 0, "awaiting_child", 6],
    ["r-3b9c55", 1, "awaiting_child", 52],
    ["r-5c71e2", 2, "awaiting_child", 9],
    ["r-d40e17", 3, "succeeded", 14],
  ];
  return {
    run,
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "Four fix batches are running as their own loops",
      detail: "One batch has been waiting 47m for a decision inside its own run.",
      usage: { cost_usd: 0.42, duration: "14m02s" },
    }),
    definition: reviewAndFixDefinition,
    frames: [],
    generations: [],
    rosterNodes: [
      node("review", "succeeded", { session_id: "ses-77120a3f" }),
      ...branches.map(([runId, itemIndex, state, startedMinutesAgo]) =>
        node("fix_batch", state, {
          item_index: itemIndex,
          started_at: storyAt(startedMinutesAgo),
          child_loop_run_id: runId,
        })
      ),
      node("collect_fixes", "pending"),
      node("write_artifacts", "pending"),
    ],
    rosterRollups: [rollup],
    timeline: [],
  };
}

/**
 * The graph from the issue: one `wave` step running a child loop, whose own
 * step runs another — three levels deep — beside steps nothing has reached.
 */
const waveDefinition: LoopDefinition = {
  ...reviewAndFixDefinition,
  meta: { ...reviewAndFixDefinition.meta, name: "nightly-waves" },
  graph: {
    nodes: [
      { id: "prepare", class: "action", kind: "run-agent" },
      { id: "wave", class: "action", kind: "run-loop", params: { loop: "run-one-wave" } },
      { id: "wave_start_failed", class: "action", kind: "run-agent" },
      { id: "wave_ok", class: "action", kind: "run-agent" },
      { id: "wave_failed", class: "action", kind: "run-agent" },
    ],
    edges: [
      { from: "prepare", to: "wave" },
      { from: "prepare", to: "wave_start_failed" },
      { from: "wave", to: "wave_ok" },
      { from: "wave", to: "wave_failed" },
    ],
  } as unknown as LoopDefinitionGraph,
};

export function nestedWaveScenario(): LoopRunStoryScenario {
  const progress = { round: 1, steps_done: 1, steps_total: 3 };
  const run = reviewAndFixRun({
    loop_name: "nightly-waves",
    status: "running",
    generation: 1,
    progress,
  });
  return {
    run,
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "Wave 2 is running as its own loop",
      detail: "Nothing needs you here.",
    }),
    definition: waveDefinition,
    frames: [],
    generations: [],
    rosterNodes: [
      node("prepare", "succeeded", { generation: 1 }),
      node("wave", "awaiting_child", {
        generation: 1,
        started_at: storyAt(18),
        child_loop_run_id: "r-wave02",
      }),
      node("wave_start_failed", "not_taken", { generation: 1 }),
      node("wave_ok", "pending", { generation: 1 }),
      node("wave_failed", "pending", { generation: 1 }),
    ],
    rosterRollups: [],
    timeline: [],
  };
}

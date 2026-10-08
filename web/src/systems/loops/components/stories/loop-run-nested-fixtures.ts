import { HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";

import { loopEffectiveConfigFixture } from "../../mocks/fixtures-config";
import { materializeContractFixture } from "../../mocks/materialize-contract-fixture";
import type {
  LoopFanoutRollup,
  LoopRosterNode,
  LoopRunDetail,
  LoopRunRecord,
  LoopRunRosterPage,
} from "../../types";
import { briefingFor, makeRosterNode as node, storyAt } from "./loop-run-read-builders";
import { reviewAndFixDefinition, reviewAndFixRun } from "./loop-run-page-fixture-world";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";

/**
 * A loop that runs loops: the parent fans `fix_batch` out into three child runs.
 *
 * The child reads are served over MSW rather than staged as props, because the
 * page reads them the way production does — lazily, per child, from the child's
 * own detail and roster routes. Each child is in a different place so one
 * capture shows every reading the disclosure has to make: one still working,
 * one parked on a person, one finished.
 */

export const NESTED_STORY_WORKSPACE_ID = "ws_default";

const CHILD_LOOP_NAME = "fix-one-batch";

interface StoryChild {
  runId: string;
  run: Partial<LoopRunRecord>;
  nodes: LoopRosterNode[];
}

const CHILDREN: StoryChild[] = [
  {
    runId: "r-8f21a0",
    run: {
      status: "running",
      created_at: storyAt(6),
      started_at: storyAt(6),
      last_progress_at: storyAt(1),
      progress: { round: 1, steps_done: 1, steps_total: 4 },
    },
    nodes: [
      node("plan_fix", "succeeded", { generation: 1 }),
      node("run_tests", "running", { generation: 1, started_at: storyAt(2) }),
      node("apply_fix", "pending", { generation: 1 }),
      node("finalize_round", "pending", { generation: 1 }),
    ],
  },
  {
    runId: "r-3b9c55",
    run: {
      status: "needs-approval",
      created_at: storyAt(11),
      started_at: storyAt(11),
      last_progress_at: storyAt(3),
      progress: { round: 1, steps_done: 3, steps_total: 4 },
    },
    nodes: [
      node("plan_fix", "succeeded", { generation: 1 }),
      node("run_tests", "succeeded", { generation: 1 }),
      node("apply_fix", "succeeded", { generation: 1 }),
      node("finalize_round", "control_pending", { generation: 1, started_at: storyAt(3) }),
    ],
  },
  {
    runId: "r-d40e17",
    run: {
      status: "done",
      created_at: storyAt(14),
      started_at: storyAt(14),
      last_progress_at: storyAt(5),
      completed_at: storyAt(5),
      progress: { round: 1, steps_done: 4, steps_total: 4 },
    },
    nodes: [
      node("plan_fix", "succeeded", { generation: 1 }),
      node("run_tests", "succeeded", { generation: 1 }),
      node("apply_fix", "succeeded", { generation: 1 }),
      node("finalize_round", "succeeded", { generation: 1 }),
    ],
  },
];

/** The run as the briefing describes it: served step counts included. */
function childRecord(child: StoryChild): LoopRunRecord {
  return reviewAndFixRun({
    id: child.runId,
    loop_name: CHILD_LOOP_NAME,
    generation: 1,
    parent_loop_run_id: reviewAndFixRun().id,
    started_by_kind: "loop",
    tokens_used: 9_400,
    ...child.run,
  });
}

function childDetail(child: StoryChild): LoopRunDetail {
  return {
    // The detail route does not populate `progress`; the briefing serves it. A
    // fixture that filled it here would hide a page reading the wrong route.
    run: { ...childRecord(child), progress: { round: 0, steps_done: 0, steps_total: 0 } },
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
    loop_name: CHILD_LOOP_NAME,
    run_status: childRecord(child).status,
    nodes: child.nodes,
    fanout_rollups: [],
    next_cursor: "",
  };
}

const childById = new Map(CHILDREN.map(child => [child.runId, child]));

/** The three child reads the disclosure makes, answered for every staged child. */
export const nestedChildRunHandlers = [
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}/briefing", ({ params }) => {
    const child = childById.get(String(params.run_id));
    return child
      ? HttpResponse.json(
          briefingFor(childRecord(child), { tone: "ok", headline: "Fixing one batch" })
        )
      : HttpResponse.json({ error: "Loop run not found" }, { status: 404 });
  }),
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}", ({ params }) => {
    const child = childById.get(String(params.run_id));
    return child
      ? HttpResponse.json(childDetail(child))
      : HttpResponse.json({ error: "Loop run not found" }, { status: 404 });
  }),
  compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes", ({ params }) => {
    const child = childById.get(String(params.run_id));
    return child
      ? HttpResponse.json(childRoster(child))
      : HttpResponse.json({ error: "Loop run not found" }, { status: 404 });
  }),
];

/** The parent: `fix_batch` fanned into three child runs, two still owed. */
export function nestedLoopsScenario(): LoopRunStoryScenario {
  const progress = { round: 2, steps_done: 2, steps_total: 6 };
  const run = reviewAndFixRun({ status: "running", generation: 2, progress });
  const rollup: LoopFanoutRollup = {
    generation: 2,
    node_id: "fix_batch",
    done: 1,
    total: 3,
    failed: 0,
  };
  const [working, parked, finished] = CHILDREN;
  return {
    run,
    briefing: briefingFor(run, {
      tone: "ok",
      headline: "Three fix batches are running as their own loops",
      detail: "Nothing needs you here. One batch is waiting for a decision inside its own run.",
      usage: { cost_usd: 0.42, duration: "14m02s" },
    }),
    definition: reviewAndFixDefinition,
    frames: [],
    generations: [],
    rosterNodes: [
      node("review", "succeeded", { session_id: "ses-77120a3f" }),
      node("fix_batch", "awaiting_child", {
        item_index: 0,
        started_at: storyAt(6),
        child_loop_run_id: working.runId,
      }),
      node("fix_batch", "awaiting_child", {
        item_index: 1,
        started_at: storyAt(11),
        child_loop_run_id: parked.runId,
      }),
      node("fix_batch", "succeeded", {
        item_index: 2,
        started_at: storyAt(14),
        ended_at: storyAt(5),
        child_loop_run_id: finished.runId,
      }),
      node("collect_fixes", "pending"),
      node("write_artifacts", "pending"),
    ],
    rosterRollups: [rollup],
    timeline: [],
  };
}

import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";

import {
  GRAPH_ENG_FORK_RUN_ID,
  GRAPH_ENG_RUN_ID,
  GRAPH_ENG_TERMINAL_RUN_ID,
  graphEngPendingRequests,
  graphEngResolvedRequests,
} from "./fixture-graph-eng-requests";
import { RELEASE_TRAIN_LOOP_NAME, releaseTrainDetail } from "./fixture-release-train";
import { materializeContractFixture } from "./materialize-contract-fixture";
import type {
  LoopAmendment,
  LoopRun,
  LoopRunDetail,
  LoopRunGeneration,
  LoopNodeWait,
} from "../types";

const WORKSPACE_ID = "ws_default";

function buildReleaseTrainRun(
  overrides: Partial<LoopRun> & Pick<LoopRun, "id" | "status">
): LoopRun {
  return {
    workspace_id: WORKSPACE_ID,
    loop_name: RELEASE_TRAIN_LOOP_NAME,
    completion_state: "complete",
    historical: false,
    generation: 3,
    iteration_cap: 20,
    tokens_used: 214_000,
    pause_requested: false,
    budget_tokens: 900_000,
    budget_wall_sec: 7_200,
    budget_on_exceeded: "escalate",
    reattempt_strategy: "failed_only",
    resolved_network_participation: buildLocalNetworkParticipationFixture(),
    created_at: "2026-08-17T08:40:00Z",
    started_at: "2026-08-17T08:40:00Z",
    last_progress_at: "2026-08-17T09:00:00Z",
    definition_version: 3,
    definition_digest: "sha256:release-train-v3",
    start_metadata: {},
    inputs: { services: "api,web,worker", severity: "p1" },
    ...overrides,
    // Server-owned step/round progress (B-001): the roster reads it, never
    // derives it, so the round has to follow the generation the caller set. The
    // fork runs at generation 2 and must say round 2.
    progress: overrides.progress ?? {
      round: overrides.generation ?? 3,
      steps_done: 4,
      steps_total: 6,
    },
    forks: overrides.forks ?? [],
  };
}

export const releaseTrainRun: LoopRun = buildReleaseTrainRun({
  id: GRAPH_ENG_RUN_ID,
  status: "running",
  forks: [{ run_id: GRAPH_ENG_FORK_RUN_ID, generation: 2 }],
});

export const releaseTrainPartialRun: LoopRun = buildReleaseTrainRun({
  id: GRAPH_ENG_TERMINAL_RUN_ID,
  status: "done",
  completion_state: "partial",
  generation: 3,
  last_progress_at: "2026-08-17T09:40:00Z",
});

export const releaseTrainForkRun: LoopRun = buildReleaseTrainRun({
  id: GRAPH_ENG_FORK_RUN_ID,
  status: "running",
  generation: 2,
  inputs: { services: "api,web,worker", severity: "p0" },
  forked_from: { run_id: GRAPH_ENG_RUN_ID, generation: 2 },
});

const routeCauses: LoopRunGeneration["route_causes"] = [
  {
    at: "2026-08-17T08:52:00Z",
    cause: "condition_matched",
    item_index: 0,
    matched_when: 'inputs.severity == "p1"',
    node_id: "triage",
    route: "standard",
  },
];

const defaultRouteCauses: LoopRunGeneration["route_causes"] = [
  {
    at: "2026-08-17T08:52:00Z",
    cause: "default_route",
    default: true,
    item_index: 0,
    node_id: "triage",
    route: "backlog",
  },
];

function rolloutGeneration(generation: number, settled: boolean): LoopRunGeneration {
  return {
    generation,
    parent_generation: Math.max(0, generation - 1),
    origin: generation === 3 ? "operator_rerun" : "initial",
    route_causes: generation === 1 ? defaultRouteCauses : routeCauses,
    verdicts: [],
    outputs: [
      { node_id: "services", status: "succeeded", generation },
      { node_id: "confirm-rollout", status: settled ? "succeeded" : "pending", generation },
      { node_id: "triage", status: "succeeded", generation },
      { node_id: "standard", status: "succeeded", generation },
      { node_id: "rollout", status: "succeeded", generation },
      { node_id: "apply-migration", status: "succeeded", generation, item_index: 0 },
      { node_id: "apply-migration", status: "failed", generation, item_index: 1 },
      {
        node_id: "apply-migration",
        status: settled ? "canceled" : "running",
        generation,
        item_index: 2,
        disposition: settled ? "canceled_by_strategy" : undefined,
      },
      {
        node_id: "collect-rollout",
        status: settled ? "partial" : "pending",
        generation,
      },
      { node_id: "render-notes", status: settled ? "succeeded" : "pending", generation },
    ],
  };
}

export const releaseTrainAmendments: LoopAmendment[] = [
  {
    loop_run_id: GRAPH_ENG_RUN_ID,
    generation: 3,
    node_id: "render-notes",
    item_index: 0,
    amendment_seq: 1,
    actor_kind: "operator",
    actor_id: "pedro",
    reason: "analyst over-rated the risk",
    created_at: "2026-08-17T09:22:00Z",
    original: { risk: "high" },
    amended: { risk: "medium" },
  },
];

const pendingRequestWaits: LoopNodeWait[] = graphEngPendingRequests.map(entry => ({
  admission_failures: 0,
  age_seconds: 420,
  claim_state: "waiting",
  created_at: entry.opened_at,
  escalation_cursor: 0,
  expect: entry.expect,
  generation: entry.generation,
  issued_epoch: 1,
  item_index: entry.item_index,
  kind: "request",
  loop_run_id: entry.loop_run_id,
  node_id: entry.node_id,
  resume_at: entry.expires_at ?? null,
}));

export const releaseTrainRunDetail: LoopRunDetail = {
  run: {
    ...releaseTrainRun,
    started_by_kind: "user",
    started_by_ref: "pedro",
    started_origin_kind: "cli",
  },
  executed_definition: releaseTrainDetail.definition,
  materialized_contract: materializeContractFixture(
    releaseTrainDetail.definition.contract,
    releaseTrainRun.inputs ?? {}
  ),
  generations: [
    rolloutGeneration(1, true),
    rolloutGeneration(2, true),
    rolloutGeneration(3, false),
  ],
  node_controls: [],
  waits: pendingRequestWaits,
  requests: graphEngPendingRequests,
  amendments: releaseTrainAmendments,
};

export const releaseTrainPartialRunDetail: LoopRunDetail = {
  run: {
    ...releaseTrainPartialRun,
    started_by_kind: "user",
    started_by_ref: "pedro",
    started_origin_kind: "cli",
  },
  executed_definition: releaseTrainDetail.definition,
  materialized_contract: materializeContractFixture(
    releaseTrainDetail.definition.contract,
    releaseTrainPartialRun.inputs ?? {}
  ),
  generations: [rolloutGeneration(1, true), rolloutGeneration(2, true), rolloutGeneration(3, true)],
  node_controls: [],
  waits: [],

  requests: graphEngResolvedRequests,
  amendments: releaseTrainAmendments,
};

export const releaseTrainForkRunDetail: LoopRunDetail = {
  run: {
    ...releaseTrainForkRun,
    started_by_kind: "user",
    started_by_ref: "pedro",
    started_origin_kind: "http",
  },
  executed_definition: releaseTrainDetail.definition,
  materialized_contract: materializeContractFixture(
    releaseTrainDetail.definition.contract,
    releaseTrainForkRun.inputs ?? {}
  ),
  generations: [
    {
      generation: 1,
      parent_generation: 0,
      origin: "fork_seed",
      route_causes: [],
      verdicts: [],
      outputs: [{ node_id: "services", status: "succeeded", generation: 1 }],
    },
    rolloutGeneration(2, false),
  ],
  node_controls: [],
  waits: [],
  requests: [],
  amendments: [],
};

export const graphEngRunFixtures: LoopRun[] = [
  releaseTrainRun,
  releaseTrainPartialRun,
  releaseTrainForkRun,
];

export const graphEngRunDetailByRunId = new Map<string, LoopRunDetail>([
  [GRAPH_ENG_RUN_ID, releaseTrainRunDetail],
  [GRAPH_ENG_TERMINAL_RUN_ID, releaseTrainPartialRunDetail],
  [GRAPH_ENG_FORK_RUN_ID, releaseTrainForkRunDetail],
]);

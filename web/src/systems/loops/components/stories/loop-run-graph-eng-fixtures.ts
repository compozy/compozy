import {
  GRAPH_ENG_FORK_RUN_ID,
  GRAPH_ENG_RUN_ID,
  answeredAskRequest,
  canceledReviewRequest,
  expiredAskRequest,
  laneAskRequests,
  nearExpiryAskRequest,
  pendingAskRequest,
  pendingEnumAskRequest,
  pendingReviewRequest,
  redactedContextRequest,
  releaseTrainDetail,
  releaseTrainForkRunDetail,
  releaseTrainPartialRunDetail,
  releaseTrainRunDetail,
} from "../../mocks";
import type { LoopRunDetail, LoopRunEventFrame, LoopRequest } from "../../types";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";
import { createFrameFactory } from "./loop-run-page-fixture-world";

function fromRunDetail(detail: LoopRunDetail, frames: LoopRunEventFrame[]): LoopRunStoryScenario {
  return {
    run: detail.run,
    definition: detail.executed_definition ?? releaseTrainDetail.definition,
    frames,
    generations: detail.generations ?? [],
    nodeControls: detail.node_controls,
    waits: detail.waits,
    requests: detail.requests,
  };
}

function requestOpenedPayload(request: LoopRequest) {
  return {
    generation: request.generation,
    node_id: request.node_id,
    item_index: request.item_index,
    kind: request.kind,
    prompt: request.prompt,
    decisions: request.decisions,
    expires_at: request.expires_at ?? "",
  };
}

function releaseTrainFrames(): LoopRunEventFrame[] {
  const frame = createFrameFactory(GRAPH_ENG_RUN_ID);
  return [
    frame("generation_started", 22, {
      generation: 3,
      parent_generation: 2,
      origin: "operator_rerun",
    }),
    frame("route_taken", 20, {
      generation: 3,
      node_id: "triage",
      item_index: 0,
      route: "standard",
      cause: "condition_matched",
      matched_when: 'inputs.severity == "p1"',
    }),
    frame("request_opened", 18, requestOpenedPayload(pendingAskRequest)),
    frame("request_opened", 17, requestOpenedPayload(pendingReviewRequest)),
    frame("branch_pruned", 12, {
      generation: 3,
      node_id: "apply-migration",
      item_indexes: [2],
      reason: "fail_fast",
    }),
    frame("node_amended", 8, {
      generation: 3,
      node_id: "render-notes",
      item_index: 0,
      amendment_seq: 1,
      actor_kind: "operator",
      actor_id: "pedro",
    }),
    frame("run_forked", 5, {
      source_run_id: GRAPH_ENG_RUN_ID,
      source_generation: 2,
      fork_run_id: GRAPH_ENG_FORK_RUN_ID,
    }),
  ];
}

function resolvedRequestFrames(): LoopRunEventFrame[] {
  const frame = createFrameFactory(GRAPH_ENG_RUN_ID);
  return [
    frame("request_opened", 40, requestOpenedPayload(answeredAskRequest)),
    frame("request_answered", 30, {
      generation: answeredAskRequest.generation,
      node_id: answeredAskRequest.node_id,
      item_index: answeredAskRequest.item_index,
      decision: "respond",
      actor_kind: "operator",
      actor_id: "pedro",
    }),
    frame("request_expired", 20, {
      generation: expiredAskRequest.generation,
      node_id: expiredAskRequest.node_id,
      item_index: expiredAskRequest.item_index,
    }),
    frame("request_canceled", 10, {
      generation: canceledReviewRequest.generation,
      node_id: canceledReviewRequest.node_id,
      item_index: canceledReviewRequest.item_index,
      actor_kind: "operator",
      actor_id: "pedro",
    }),
  ];
}

export function pendingRequestsScenario(): LoopRunStoryScenario {
  return fromRunDetail(releaseTrainRunDetail, releaseTrainFrames());
}

export function pendingEnumRequestScenario(): LoopRunStoryScenario {
  return {
    ...fromRunDetail(releaseTrainRunDetail, releaseTrainFrames()),
    requests: [pendingEnumAskRequest],
  };
}

export function resolvedRequestsScenario(): LoopRunStoryScenario {
  return fromRunDetail(releaseTrainPartialRunDetail, resolvedRequestFrames());
}

export function redactedRequestScenario(): LoopRunStoryScenario {
  return {
    ...fromRunDetail(releaseTrainRunDetail, releaseTrainFrames()),
    requests: [redactedContextRequest, nearExpiryAskRequest],
  };
}

export function laneRequestsScenario(): LoopRunStoryScenario {
  return {
    ...fromRunDetail(releaseTrainRunDetail, releaseTrainFrames()),
    requests: laneAskRequests,
  };
}

export function partialCompletionScenario(): LoopRunStoryScenario {
  return fromRunDetail(releaseTrainPartialRunDetail, releaseTrainFrames());
}

export function forkedRunScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory(GRAPH_ENG_FORK_RUN_ID);
  return fromRunDetail(releaseTrainForkRunDetail, [
    frame("generation_started", 6, { generation: 2, parent_generation: 1, origin: "fork_seed" }),
  ]);
}

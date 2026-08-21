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
import {
  type StoryVerdict,
  briefingFor,
  makeTimelineEntry as entry,
} from "./loop-run-read-builders";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";
import { createFrameFactory } from "./loop-run-page-fixture-world";

/**
 * One `getLoopRun` payload plus the verdict the daemon would serve over it.
 *
 * The verdict is a parameter rather than a default because these scenarios
 * stage very different runs — a live one with two open questions, a settled one
 * whose questions were all answered — and one sentence cannot be true of both.
 */
function fromRunDetail(
  detail: LoopRunDetail,
  frames: LoopRunEventFrame[],
  verdict: StoryVerdict
): LoopRunStoryScenario {
  return {
    run: detail.run,
    briefing: briefingFor(detail.run, verdict),
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

/** The verdict over a release-train round holding two unanswered questions. */
const PENDING_VERDICT: StoryVerdict = {
  tone: "needs_you",
  headline: "Two questions are waiting for your answer before the rollout continues",
  detail: "The migration and the region order both need a decision in round 3.",
};

export function pendingRequestsScenario(): LoopRunStoryScenario {
  return fromRunDetail(releaseTrainRunDetail, releaseTrainFrames(), PENDING_VERDICT);
}

export function pendingEnumRequestScenario(): LoopRunStoryScenario {
  return {
    ...fromRunDetail(releaseTrainRunDetail, releaseTrainFrames(), PENDING_VERDICT),
    requests: [pendingEnumAskRequest],
  };
}

export function resolvedRequestsScenario(): LoopRunStoryScenario {
  return fromRunDetail(releaseTrainPartialRunDetail, resolvedRequestFrames(), {
    tone: "ok",
    headline: "The rollout finished after every question was answered",
    detail: "One answered, one expired, one canceled — all three are recorded in the story.",
    outcome: { status: "done", cause: "stop_when", at: "2026-08-17T09:40:00Z" },
  });
}

export function redactedRequestScenario(): LoopRunStoryScenario {
  return {
    ...fromRunDetail(releaseTrainRunDetail, releaseTrainFrames(), PENDING_VERDICT),
    requests: [redactedContextRequest, nearExpiryAskRequest],
  };
}

export function laneRequestsScenario(): LoopRunStoryScenario {
  return {
    ...fromRunDetail(releaseTrainRunDetail, releaseTrainFrames(), {
      ...PENDING_VERDICT,
      headline: "Every migration lane is asking the same question",
      detail: "Each worker needs its own answer before round 3 can settle.",
    }),
    requests: laneAskRequests,
  };
}

/**
 * The fork's own history, which is not the same as its lineage.
 *
 * `run_forked` is appended to the *source* run
 * (`global_db_loop_timetravel_create.go:241` writes it against `source.ID`), so
 * a forked child never carries that beat and a fixture that staged one on this
 * run would be inventing an event. The child's story is its own beats; the
 * "forked from" side is served on the run record and rendered by
 * `LoopRunLineageSection`. Staging nothing left the pane reading "Nothing has
 * happened in this run yet." over a run that had plainly run.
 */
function forkedRunTimeline() {
  return [
    entry(14, "node_running", "Step confirm-rollout running", {
      generation: 2,
      node_id: "confirm-rollout",
    }),
    entry(11, "node_succeeded", "Step services succeeded", { generation: 2, node_id: "services" }),
    entry(8, "generation_started", "Round 2 started", { generation: 2 }),
  ];
}

export function forkedRunScenario(): LoopRunStoryScenario {
  const frame = createFrameFactory(GRAPH_ENG_FORK_RUN_ID);
  const scenario = fromRunDetail(
    releaseTrainForkRunDetail,
    [frame("generation_started", 6, { generation: 2, parent_generation: 1, origin: "fork_seed" })],
    {
      tone: "ok",
      headline: "Round 2 restarted in a fork with the severity raised to p0",
      detail: "The original run is untouched; this one carries the changed input.",
    }
  );
  return { ...scenario, timeline: forkedRunTimeline() };
}

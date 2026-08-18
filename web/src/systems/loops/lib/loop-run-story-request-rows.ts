import {
  parseBranchPruned,
  parseNodeAmended,
  parseRequestOpened,
  parseRequestResolved,
  parseRouteTaken,
  parseRunForked,
} from "./loop-graph-events";
import { humanizeLoopNodeId, type MutableStoryRow } from "./loop-run-story-rows";
import { isLoopRequestKind, LOOP_REQUEST_KIND_TITLE } from "./loop-request-vocabulary";
import type { LoopRunEventFrame } from "../types";

function lane(itemIndex: number): string {
  return itemIndex > 0 ? ` · lane ${itemIndex}` : "";
}

function requestOpenedRow(frame: LoopRunEventFrame): MutableStoryRow | null {
  const payload = parseRequestOpened(frame);
  if (!payload) return null;
  const kind = isLoopRequestKind(payload.kind) ? payload.kind : "ask";
  return {
    key: `${frame.id}`,
    kind: "request_opened",
    seq: frame.seq,
    at: frame.at,
    tone: "warning",
    icon: "request-opened",
    title: `${LOOP_REQUEST_KIND_TITLE[kind]} on ${humanizeLoopNodeId(payload.nodeId)}`,
    sub: payload.prompt || undefined,
    micro: `request_opened · ${payload.nodeId}${lane(payload.itemIndex)}`,
    nodeId: payload.nodeId,
    generation: payload.generation,
  };
}

function requestAnsweredRow(frame: LoopRunEventFrame): MutableStoryRow | null {
  const payload = parseRequestResolved(frame);
  if (!payload) return null;
  const actor = payload.actorId || payload.actorKind;
  const decision = payload.decision ? ` · ${payload.decision}` : "";
  return {
    key: `${frame.id}`,
    kind: "request_answered",
    seq: frame.seq,
    at: frame.at,
    tone: "info",
    icon: "request-answered",
    title: `Answered ${humanizeLoopNodeId(payload.nodeId)}`,
    sub: actor ? `by ${actor}` : undefined,
    micro: `request_answered · ${payload.nodeId}${lane(payload.itemIndex)}${decision}`,
    nodeId: payload.nodeId,
    generation: payload.generation,
  };
}

function requestExpiredRow(frame: LoopRunEventFrame): MutableStoryRow | null {
  const payload = parseRequestResolved(frame);
  if (!payload) return null;
  return {
    key: `${frame.id}`,
    kind: "request_expired",
    seq: frame.seq,
    at: frame.at,
    tone: "danger",
    icon: "request-expired",
    title: `Request on ${humanizeLoopNodeId(payload.nodeId)} expired`,
    micro: `request_expired · ${payload.nodeId}${lane(payload.itemIndex)}`,
    nodeId: payload.nodeId,
    generation: payload.generation,
  };
}

function requestCanceledRow(frame: LoopRunEventFrame): MutableStoryRow | null {
  const payload = parseRequestResolved(frame);
  if (!payload) return null;
  const actor = payload.actorId || payload.actorKind;
  return {
    key: `${frame.id}`,
    kind: "request_canceled",
    seq: frame.seq,
    at: frame.at,
    tone: "neutral",
    icon: "pruned",
    title: `Request on ${humanizeLoopNodeId(payload.nodeId)} canceled`,
    sub: actor ? `by ${actor}` : undefined,
    micro: `request_canceled · ${payload.nodeId}${lane(payload.itemIndex)}`,
    nodeId: payload.nodeId,
    generation: payload.generation,
  };
}

function routeTakenRow(frame: LoopRunEventFrame): MutableStoryRow | null {
  const payload = parseRouteTaken(frame);
  if (!payload) return null;
  const sub = payload.isDefault
    ? "default — no condition matched"
    : payload.matchedWhen
      ? `matched ${payload.matchedWhen}`
      : undefined;
  return {
    key: `${frame.id}`,
    kind: "route_taken",
    seq: frame.seq,
    at: frame.at,
    tone: "neutral",
    icon: payload.isDefault ? "route-default" : "route-taken",
    title: `${humanizeLoopNodeId(payload.nodeId)} routed to ${humanizeLoopNodeId(payload.route)}`,
    sub,
    micro: `route_taken · ${payload.nodeId} → ${payload.route}${lane(payload.itemIndex)}`,
    nodeId: payload.nodeId,
    generation: payload.generation,
  };
}

function branchPrunedRow(frame: LoopRunEventFrame): MutableStoryRow | null {
  const payload = parseBranchPruned(frame);
  if (!payload) return null;
  const count = payload.itemIndexes.length;

  const laneSummary = count === 0 ? "" : count === 1 ? "1 lane" : `${count} lanes`;
  return {
    key: `${frame.id}`,
    kind: "branch_pruned",
    seq: frame.seq,
    at: frame.at,
    tone: "neutral",
    icon: "pruned",
    title: laneSummary
      ? `${laneSummary} of ${humanizeLoopNodeId(payload.nodeId)} canceled by strategy`
      : `${humanizeLoopNodeId(payload.nodeId)} canceled by strategy`,
    sub: payload.reason || undefined,
    micro: `branch_pruned · ${payload.nodeId}${count > 0 ? ` · ${count}` : ""}`,
    nodeId: payload.nodeId,
    generation: payload.generation,
  };
}

function nodeAmendedRow(frame: LoopRunEventFrame): MutableStoryRow | null {
  const payload = parseNodeAmended(frame);
  if (!payload) return null;
  const actor = payload.actorId || payload.actorKind;
  return {
    key: `${frame.id}`,
    kind: "node_amended",
    seq: frame.seq,
    at: frame.at,
    tone: "info",
    icon: "amended",
    title: `Amended ${humanizeLoopNodeId(payload.nodeId)} output`,
    sub: actor ? `by ${actor} · the original stays in history` : "The original stays in history",
    micro: `node_amended · ${payload.nodeId}${lane(payload.itemIndex)} · #${payload.amendmentSeq}`,
    nodeId: payload.nodeId,
    generation: payload.generation,
  };
}

function runForkedRow(frame: LoopRunEventFrame): MutableStoryRow | null {
  const payload = parseRunForked(frame);
  if (!payload) return null;
  return {
    key: `${frame.id}`,
    kind: "run_forked",
    seq: frame.seq,
    at: frame.at,
    tone: "info",
    icon: "forked",
    title: `Forked from generation ${payload.sourceGeneration}`,
    sub: `New run ${payload.forkRunId}`,
    micro: `run_forked · ${payload.forkRunId}`,
    generation: payload.sourceGeneration,
  };
}

const GRAPH_STORY_ROW_BUILDERS: Record<
  string,
  (frame: LoopRunEventFrame) => MutableStoryRow | null
> = {
  request_opened: requestOpenedRow,
  request_answered: requestAnsweredRow,
  request_expired: requestExpiredRow,
  request_canceled: requestCanceledRow,
  route_taken: routeTakenRow,
  branch_pruned: branchPrunedRow,
  node_amended: nodeAmendedRow,
  run_forked: runForkedRow,
};

export function graphStoryRow(kind: string, frame: LoopRunEventFrame): MutableStoryRow | null {
  return GRAPH_STORY_ROW_BUILDERS[kind]?.(frame) ?? null;
}

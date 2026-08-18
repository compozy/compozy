import type { LoopRunEventFrame, LoopRunEventKind } from "../types";

export const LOOP_REQUEST_EVENT_KINDS: ReadonlySet<LoopRunEventKind> = new Set<LoopRunEventKind>([
  "request_opened",
  "request_answered",
  "request_expired",
  "request_canceled",
  "node_amended",
]);

export function isLoopRequestEventKind(kind: string): boolean {
  return LOOP_REQUEST_EVENT_KINDS.has(kind as LoopRunEventKind);
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
}

function str(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function int(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function intList(value: unknown): number[] {
  if (!Array.isArray(value)) return [];
  return value.filter(
    (entry): entry is number => typeof entry === "number" && Number.isFinite(entry)
  );
}

export interface LoopNodeLaneRef {
  generation: number;
  nodeId: string;
  itemIndex: number;
}

function laneRef(payload: Record<string, unknown>): LoopNodeLaneRef {
  return {
    generation: int(payload.generation),
    nodeId: str(payload.node_id),
    itemIndex: int(payload.item_index),
  };
}

export interface LoopRequestOpenedPayload extends LoopNodeLaneRef {
  kind: string;
  prompt: string;
  decisions: string[];
  expiresAt: string;
}

export function parseRequestOpened(frame: LoopRunEventFrame): LoopRequestOpenedPayload | null {
  const payload = asRecord(frame.payload);
  if (!payload) return null;
  const ref = laneRef(payload);
  if (!ref.nodeId) return null;
  const decisions = Array.isArray(payload.decisions)
    ? payload.decisions.filter((entry): entry is string => typeof entry === "string")
    : [];
  return {
    ...ref,
    kind: str(payload.kind),
    prompt: str(payload.prompt),
    decisions,
    expiresAt: str(payload.expires_at),
  };
}

export interface LoopRequestResolvedPayload extends LoopNodeLaneRef {
  decision: string;
  actorKind: string;
  actorId: string;
}

export function parseRequestResolved(frame: LoopRunEventFrame): LoopRequestResolvedPayload | null {
  const payload = asRecord(frame.payload);
  if (!payload) return null;
  const ref = laneRef(payload);
  if (!ref.nodeId) return null;
  return {
    ...ref,
    decision: str(payload.decision),
    actorKind: str(payload.actor_kind),
    actorId: str(payload.actor_id),
  };
}

export interface LoopRouteTakenPayload extends LoopNodeLaneRef {
  route: string;
  cause: string;

  matchedWhen: string;
  isDefault: boolean;
}

export function parseRouteTaken(frame: LoopRunEventFrame): LoopRouteTakenPayload | null {
  const payload = asRecord(frame.payload);
  if (!payload) return null;
  const ref = laneRef(payload);
  if (!ref.nodeId) return null;
  return {
    ...ref,
    route: str(payload.route),
    cause: str(payload.cause),
    matchedWhen: str(payload.matched_when),
    isDefault: payload.default === true,
  };
}

export interface LoopBranchPrunedPayload {
  generation: number;
  nodeId: string;

  itemIndexes: number[];
  reason: string;
}

export function parseBranchPruned(frame: LoopRunEventFrame): LoopBranchPrunedPayload | null {
  const payload = asRecord(frame.payload);
  if (!payload) return null;
  const nodeId = str(payload.node_id);
  if (!nodeId) return null;
  return {
    generation: int(payload.generation),
    nodeId,
    itemIndexes: intList(payload.item_indexes),
    reason: str(payload.reason),
  };
}

export interface LoopNodeAmendedPayload extends LoopNodeLaneRef {
  amendmentSeq: number;
  actorKind: string;
  actorId: string;
}

export function parseNodeAmended(frame: LoopRunEventFrame): LoopNodeAmendedPayload | null {
  const payload = asRecord(frame.payload);
  if (!payload) return null;
  const ref = laneRef(payload);
  if (!ref.nodeId) return null;
  return {
    ...ref,
    amendmentSeq: int(payload.amendment_seq),
    actorKind: str(payload.actor_kind),
    actorId: str(payload.actor_id),
  };
}

export interface LoopRunForkedPayload {
  sourceRunId: string;
  sourceGeneration: number;
  forkRunId: string;
}

export function parseRunForked(frame: LoopRunEventFrame): LoopRunForkedPayload | null {
  const payload = asRecord(frame.payload);
  if (!payload) return null;
  const forkRunId = str(payload.fork_run_id);
  if (!forkRunId) return null;
  return {
    sourceRunId: str(payload.source_run_id),
    sourceGeneration: int(payload.source_generation),
    forkRunId,
  };
}

import {
  isLoopRequestDecision,
  isLoopRequestKind,
  isLoopRequestState,
  LOOP_REQUEST_KIND_TITLE,
  LOOP_REQUEST_NEAR_EXPIRY_SIGNAL,
  LOOP_REQUEST_STATE_SIGNAL,
  LOOP_REQUEST_WAIT_SENTENCE,
  type LoopRequestDecision,
  type LoopRequestKind,
  type LoopRequestState,
  type LoopSignal,
} from "./loop-request-vocabulary";
import type { LoopRequest, LoopRunStatus } from "../types";
import { isTerminalLoopStatus } from "./loop-formatters";

const NEAR_EXPIRY_WINDOW_MS = 60 * 60 * 1000;

export interface LoopRequestView {
  request: LoopRequest;
  kind: LoopRequestKind;
  state: LoopRequestState;
  title: string;

  waitSentence: string;
  signal: LoopSignal;

  isAnswerable: boolean;

  decisions: LoopRequestDecision[];

  laneLabel: string;
  expiry: LoopRequestExpiry | null;

  resolution: LoopRequestResolution | null;
}

/** Stable identity for a request across refreshes: generation, node, and lane. */
export function loopRequestKey(
  request: Pick<LoopRequest, "generation" | "node_id" | "item_index">
): string {
  return `${request.generation}:${request.node_id}:${request.item_index}`;
}

export interface LoopRequestExpiry {
  at: string;
  remainingMs: number;

  label: string;
  isNearExpiry: boolean;
  isPast: boolean;
}

export interface LoopRequestResolution {
  decision: string;
  actorKind: string;
  actorId: string;
  at: string;
}

function narrowKind(value: string): LoopRequestKind {
  return isLoopRequestKind(value) ? value : "ask";
}

function narrowState(value: string): LoopRequestState {
  return isLoopRequestState(value) ? value : "pending";
}

function remainingLabel(remainingMs: number): string {
  if (remainingMs <= 0) return "expired";
  const minutes = Math.floor(remainingMs / 60_000);
  if (minutes < 60) return `expires ${Math.max(1, minutes)}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 48) return `expires ${hours}h`;
  return `expires ${Math.floor(hours / 24)}d`;
}

export function loopRequestExpiry(expiresAt: string | null | undefined, nowMs: number) {
  if (!expiresAt) return null;
  const at = Date.parse(expiresAt);
  if (Number.isNaN(at)) return null;
  const remainingMs = at - nowMs;
  return {
    at: expiresAt,
    remainingMs,
    label: remainingLabel(remainingMs),
    isNearExpiry: remainingMs > 0 && remainingMs <= NEAR_EXPIRY_WINDOW_MS,
    isPast: remainingMs <= 0,
  } satisfies LoopRequestExpiry;
}

function resolutionOf(request: LoopRequest): LoopRequestResolution | null {
  const at = request.answered_at ?? request.resolved_at ?? "";
  if (!at && !request.answered_decision && !request.actor_id) return null;
  return {
    decision: request.answered_decision ?? "",
    actorKind: request.actor_kind ?? "",
    actorId: request.actor_id ?? "",
    at,
  };
}

export function projectLoopRequest(
  request: LoopRequest,
  { nowMs, runStatus }: { nowMs: number; runStatus?: LoopRunStatus }
): LoopRequestView {
  const kind = narrowKind(request.kind);
  const state = narrowState(request.state);
  const expiry = loopRequestExpiry(request.expires_at, nowMs);
  const runTerminated = runStatus !== undefined && isTerminalLoopStatus(runStatus);
  const isAnswerable = state === "pending" && !runTerminated;
  const signal =
    isAnswerable && expiry?.isNearExpiry
      ? LOOP_REQUEST_NEAR_EXPIRY_SIGNAL
      : LOOP_REQUEST_STATE_SIGNAL[state];
  return {
    request,
    kind,
    state,
    title: LOOP_REQUEST_KIND_TITLE[kind],
    waitSentence: LOOP_REQUEST_WAIT_SENTENCE[kind],
    signal,
    isAnswerable,
    decisions: request.decisions.filter(isLoopRequestDecision),
    laneLabel: request.item_index > 0 ? `lane ${request.item_index}` : "",
    expiry,
    resolution: state === "pending" ? null : resolutionOf(request),
  };
}

export function loopRequestDecisionSchema(
  request: LoopRequest,
  decision: LoopRequestDecision
): unknown {
  switch (decision) {
    case "edit":
      return request.edit_schema;
    case "respond":
      return request.kind === "ask" ? request.expect : request.respond_schema;
    default:
      return undefined;
  }
}

export function loopRequestDecisionCarriesPayload(decision: LoopRequestDecision): boolean {
  return decision === "edit" || decision === "respond";
}

export function loopRequestsForNode(
  requests: readonly LoopRequest[],
  nodeId: string
): LoopRequest[] {
  return requests.filter(entry => entry.node_id === nodeId);
}

export function pendingLoopRequestCount(requests: readonly LoopRequest[]): number {
  return requests.filter(entry => entry.state === "pending").length;
}

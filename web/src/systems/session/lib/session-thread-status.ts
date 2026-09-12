// What the status row under the scroller needs from the thread messages: whether
// the in-flight reply has content yet, and how the most recent settled turn
// ended with its durable start/end instants. Pure over the assistant-ui message
// shapes; the render never derives these itself.

import { assistantMessageHasContent } from "@/systems/session/lib/session-thinking-state";
import type {
  SessionLastTurn,
  SessionStopAttribution,
} from "@/systems/session/lib/session-working-status";
import type { SessionPayload } from "@/systems/session/types";
import { isAgentEventPayload } from "@/systems/session/lib/message-parts";
import { isSessionErrorEvent } from "@/systems/session/lib/runtime-activity-notice";
import { providerErrorView } from "@/systems/session/lib/provider-error";

import type { SessionTimelinePart } from "./session-timeline-parts";
import {
  isRecord,
  stringField,
  toTimelineParts,
} from "@/systems/session/lib/timeline-message-parts";

interface ThreadStatusMessage {
  id?: string;
  role?: string;
  content?: unknown;
  status?: unknown;
  createdAt?: unknown;
  /** The thread shape keeps the daemon's message metadata under `custom`. */
  metadata?: { custom?: unknown } | undefined;
}

const INTERRUPT_STOP_REASONS = new Set([
  "cancelled",
  "canceled",
  "interrupted",
  "aborted",
  "stopped",
  "user_canceled",
]);
const STEER_FALLBACK_MARKER = "transcript_marker.prompt_steered";

function lastAssistant(messages: readonly ThreadStatusMessage[]): ThreadStatusMessage | null {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message?.role === "assistant") return message;
    if (message?.role === "user") return null;
  }
  return null;
}

/** Whether the reply to the latest prompt has any content yet (thinking ends here). */
export function activeReplyHasContent(messages: readonly ThreadStatusMessage[]): boolean {
  const reply = lastAssistant(messages);
  return reply !== null && assistantMessageHasContent(reply.content);
}

function messageFailed(message: ThreadStatusMessage): boolean {
  const status = message.status;
  return (
    isRecord(status) &&
    stringField(status, "type") === "incomplete" &&
    stringField(status, "reason") === "error"
  );
}

function instantMs(value: unknown): number | null {
  if (value instanceof Date) return value.getTime();
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Date.parse(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

const PROVIDER_FAILURE_WORDS: Record<string, string> = {
  provider_rate_limited: "rate limited",
  provider_auth_required: "sign-in expired",
};

const SUPERVISION_STOPPED_EVENT = "session.supervision_stopped";
const SUPERVISION_WARNING_EVENT = "session.supervision_warning";
const SESSION_STOP_ESCALATED_EVENT = "session.stop_escalated";
const TRANSCRIPT_MARKER_EVENT = "transcript_marker.created";
const TERMINAL_MARKER_KINDS = new Set([
  "transcript_marker.prompt_timeout",
  "transcript_marker.prompt_cancel",
  "transcript_marker.prompt_interrupted",
]);
/** The durable turn-stop receipt: persisted after verified quiescence, never before an attempted action. */
const TURN_QUIESCED_EVENT = "session.turn_quiesced";
const SESSION_STOPPED_EVENT = "session_stopped";
const FAILED_STOP_REASONS = new Set(["error", "agent_crashed"]);

/** The session-level stop facts the status row reads beside the transcript. */
export interface SessionStopFacts {
  state?: SessionPayload["state"];
  stop_cause?: string;
  stop_reason?: string;
  stop_detail?: string;
  escalated?: boolean | null;
  verified?: boolean | null;
}

// The quiet episode a supervision event records: `raw.supervision.quiet_warning.quiet_since`.
function quietSinceMs(data: Record<string, unknown>): number | null {
  const raw = data.raw;
  if (!isRecord(raw) || !isRecord(raw.supervision)) return null;
  const warning = raw.supervision.quiet_warning;
  return isRecord(warning) ? instantMs(stringField(warning, "quiet_since")) : null;
}

function supervisionCause(data: Record<string, unknown>): string | undefined {
  const raw = data.raw;
  return isRecord(raw) ? stringField(raw, "cause") : undefined;
}

// The daemon persists the session's end as a group of its own after the
// authored turn: `session.stop_escalated`, `session_stopped`,
// `session.supervision_stopped`, the timeout/cancel marker — and the live tail
// carries a stop it saw as an `error` event with the stop reason. Such a group
// records how the preceding turn ended; it is not a turn of its own.
function isTerminalEvidencePart(part: SessionTimelinePart): boolean {
  if (part.kind !== "data" || part.name !== "data-compozy-event") return false;
  const data = part.data;
  if (!isRecord(data)) return false;
  const type = stringField(data, "type");
  if (type === "error") return stringField(data, "stop_reason") !== undefined;
  if (type === TRANSCRIPT_MARKER_EVENT) {
    const marker = data.marker;
    return isRecord(marker) && TERMINAL_MARKER_KINDS.has(stringField(marker, "kind") ?? "");
  }
  return (
    type === SESSION_STOPPED_EVENT ||
    type === SUPERVISION_STOPPED_EVENT ||
    type === SESSION_STOP_ESCALATED_EVENT
  );
}

// When the operator sent the prompt, as the daemon recorded it on the message
// (`metadata.timestamp`); never a wall clock.
function userSentAtMs(message: ThreadStatusMessage): number | null {
  const custom = message.metadata?.custom;
  return isRecord(custom) ? instantMs(stringField(custom, "timestamp")) : null;
}

// The turn-scope receipt (`session.turn_quiesced`): the daemon persisted it
// only after the stop verified and the turn quiesced, so `escalated` here is a
// verified forced close — unlike `session.stop_escalated`, which precedes an
// attempted action and never establishes it. A receipt for another turn is not
// this turn's fact.
function turnReceiptAttribution(
  data: Record<string, unknown>,
  turnId: string | undefined
): SessionStopAttribution | null {
  const raw = data.raw;
  if (!isRecord(raw) || raw.scope !== "turn" || raw.verified !== true) return null;
  const receiptTurn = stringField(raw, "turn_id") ?? stringField(data, "turn_id");
  if (turnId && receiptTurn && receiptTurn !== turnId) return null;
  if (raw.escalated === true) return { kind: "escalated" };
  const cause = stringField(raw, "stop_cause") ?? "";
  if (cause === "" || cause === "user_requested") return { kind: "user" };
  return { kind: "other", detail: cause.replaceAll("_", " ") };
}

// Session-scope attribution from the resource: the operator's request, an
// escalated close the daemon verified, supervision's inactivity stop, or any
// other daemon stop with its detail. `null` when the session did not stop.
function sessionStopAttribution(
  facts: SessionStopFacts,
  noWorkMs: number | null,
  recordedCause: string | undefined
): SessionStopAttribution | null {
  const stoppedState = facts.state === "stopped" || facts.state === "stopping";
  // The transcript's own `session.supervision_stopped` names the cause; the
  // resource may lag behind it or omit it. An active session never reads a stop.
  if (recordedCause === "inactivity" && (stoppedState || facts.state === undefined)) {
    return { kind: "inactivity", noWorkMs };
  }
  if (!stoppedState) return null;
  const cause = facts.stop_cause?.trim() ?? "";
  const reason = facts.stop_reason?.trim() ?? "";
  if (cause === "inactivity") return { kind: "inactivity", noWorkMs };
  if (facts.escalated === true && facts.verified === true) return { kind: "escalated" };
  if (cause === "user_requested" || reason === "user_canceled") return { kind: "user" };
  if (cause === "" && reason === "") return null;
  const detail = facts.stop_detail?.trim();
  return { kind: "other", detail: detail || (cause || reason).replaceAll("_", " ") };
}

/**
 * The most recent turn as the transcript and the session resource record it,
 * for the frozen sentence after the turn: an operator stop (stop reason on the
 * daemon's event, or the session's `user_requested` cause), a fallback steer
 * (steer marker with `interrupt_fallback`), a failure (session error event,
 * a crashed/errored session stop, or the message's error status), the daemon's
 * own stops (escalated and verified, inactivity with the actual quiet span,
 * or another cause with its detail), or a completed turn. Durations span the
 * earliest and latest recorded instants of the turn's parts; `null` when the
 * last message is still running or no assistant turn exists.
 */
// The daemon projects one turn as several assistant messages (its segments,
// single-event receipts) and persists the session's end as a synthetic group of
// its own. The turn is the union of every trailing assistant message after the
// operator's prompt: the authored segments plus a trailing terminal-only group,
// which records how that turn ended. The prompt's own instant bounds the span.
function lastTurnParts(messages: readonly ThreadStatusMessage[]) {
  const last = messages.at(-1);
  if (!last || last.role !== "assistant") return null;
  const segments: { turnId: string; parts: SessionTimelinePart[] }[] = [];
  let sentAtMs: number | null = null;
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (!message || message.role !== "assistant") {
      if (message?.role === "user") sentAtMs = userSentAtMs(message);
      break;
    }
    const parts = toTimelineParts(message);
    const turnId = parts.find(part => part.turnId)?.turnId ?? message.id ?? "";
    const current = segments[0];
    if (current && current.turnId === turnId) current.parts.unshift(...parts);
    else segments.unshift({ turnId, parts: [...parts] });
  }
  let authoredIndex = segments.length - 1;
  while (authoredIndex > 0) {
    const segment = segments[authoredIndex]!;
    if (segment.parts.length === 0 || !segment.parts.every(isTerminalEvidencePart)) break;
    authoredIndex -= 1;
  }
  const authored = segments[authoredIndex]!;
  return {
    message: last,
    parts: segments.slice(authoredIndex).flatMap(segment => segment.parts),
    turnId: authored.turnId,
    sentAtMs,
  };
}

export function lastSettledTurn(
  messages: readonly ThreadStatusMessage[],
  facts: SessionStopFacts = {}
): SessionLastTurn | null {
  const turn = lastTurnParts(messages);
  if (!turn) return null;
  const { message, parts } = turn;
  const status = message.status;
  if (isRecord(status) && stringField(status, "type") === "running") return null;
  let startedAtMs: number | null = turn.sentAtMs;
  let endedAtMs: number | null = turn.sentAtMs;
  let stopped = false;
  let superseded = false;
  // A failure the daemon recorded (an error event, a failed stop reason)
  // outranks every stop; the message's own error status alone does not outrank
  // a stop the daemon attributed — the live tail marks stopped replies as
  // incomplete too.
  let failedByEvidence = false;
  const messageFailedStatus = messageFailed(message);
  let failureCause: string | null = null;
  let noWorkMs: number | null = null;
  let latestQuietSinceMs: number | null = null;
  let recordedCause: string | undefined;
  let turnReceipt: SessionStopAttribution | null = null;
  let awaitingResult = false;
  const turnId = turn.turnId;
  for (const part of parts) {
    const at = part.timestamp ? instantMs(part.timestamp) : null;
    if (at !== null) {
      startedAtMs = startedAtMs === null ? at : Math.min(startedAtMs, at);
      endedAtMs = endedAtMs === null ? at : Math.max(endedAtMs, at);
    }
    if (part.kind === "tool" && part.status === "running") awaitingResult = true;
    if (part.kind !== "data" || part.name !== "data-compozy-event") continue;
    const data = part.data;
    if (!isRecord(data)) continue;
    const stopReason = stringField(data, "stop_reason")?.toLowerCase();
    if (stopReason && INTERRUPT_STOP_REASONS.has(stopReason)) stopped = true;
    const type = stringField(data, "type");
    if (type === SUPERVISION_WARNING_EVENT) {
      latestQuietSinceMs = quietSinceMs(data) ?? latestQuietSinceMs;
    }
    if (type === SUPERVISION_STOPPED_EVENT) {
      // "no work for N": from the episode the stop closed — the stop event's
      // own record, else the latest warning's — to the stop instant. Nothing
      // recorded means no span is stated.
      recordedCause = supervisionCause(data) ?? recordedCause;
      const stoppedAt = instantMs(stringField(data, "timestamp") ?? part.timestamp);
      const since = quietSinceMs(data) ?? latestQuietSinceMs;
      noWorkMs = stoppedAt !== null && since !== null ? Math.max(0, stoppedAt - since) : null;
    }
    if (type === TURN_QUIESCED_EVENT) {
      // The latest receipt for this turn wins on replay or reload.
      turnReceipt = turnReceiptAttribution(data, turnId) ?? turnReceipt;
    }
    if (type === SESSION_STOPPED_EVENT && stopReason && FAILED_STOP_REASONS.has(stopReason)) {
      failedByEvidence = true;
      failureCause = failureCause ?? stopReason.replaceAll("_", " ");
    }
    const marker = data.marker;
    if (
      isRecord(marker) &&
      stringField(marker, "kind") === STEER_FALLBACK_MARKER &&
      isRecord(marker.evidence) &&
      stringField(marker.evidence, "steer_delivery") === "interrupt_fallback"
    ) {
      superseded = true;
    }
    if (isAgentEventPayload(data) && isSessionErrorEvent(data)) {
      failedByEvidence = true;
      const provider = providerErrorView(data);
      failureCause =
        (provider ? PROVIDER_FAILURE_WORDS[provider.code] : undefined) ??
        data.failure?.kind?.replaceAll("_", " ") ??
        failureCause;
    }
  }
  // A call still awaiting its result is live work — unless the daemon already
  // recorded the turn's end (its receipt, an interrupting stop): then the call
  // was cut short and the turn is settled.
  if (awaitingResult && turnReceipt === null && !stopped) return null;
  if (startedAtMs === null) {
    const created = instantMs(message.createdAt);
    startedAtMs = created;
    endedAtMs = endedAtMs ?? created;
  }
  const sessionStop = sessionStopAttribution(facts, noWorkMs, recordedCause);
  if (failedByEvidence) {
    return { startedAtMs, endedAtMs, cause: "failed", failureCause };
  }
  if (sessionStop !== null) {
    return { startedAtMs, endedAtMs, cause: "stopped", failureCause: null, stop: sessionStop };
  }
  if (turnReceipt !== null || stopped) {
    return {
      startedAtMs,
      endedAtMs,
      cause: "stopped",
      failureCause: null,
      stop: turnReceipt ?? { kind: "user" },
    };
  }
  if (messageFailedStatus) {
    return { startedAtMs, endedAtMs, cause: "failed", failureCause };
  }
  return {
    startedAtMs,
    endedAtMs,
    cause: superseded ? "steer_fallback" : "completed",
    failureCause: null,
  };
}

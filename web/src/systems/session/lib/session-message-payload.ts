// Session messages (`_uiux.md` S1–S4): the one wire → view mapping for the
// daemon's typed facts about agent-to-agent messages. Every surface reads these
// view models, never the raw payloads, so the shapes in `_dx.md` change here
// only:
// - `metadata.custom.origin` on a user message (`PromptOriginMeta`, S1);
// - `metadata.custom.synthetic` with kind `session_reply` on a system message (S2);
// - the `data-compozy-session-message` part beside a `compozy__session_prompt` call (S3);
// - `origin` on a queued input (`SessionInputPayload.origin`, S4).
// S1/S4 read the generated `PromptOriginPayload`. The UI transcript's metadata
// and data parts are untyped in the OpenAPI document (`metadata: unknown`), so
// S2/S3 read DTOs that mirror the daemon structs (`transcript.inputUISyntheticMeta`,
// `transcript.UISessionMessagePayload`), checked here once. The reply wake's
// text is daemon-authored and never parsed: identity, outcome, links and the
// answer come only from the typed fields.

import type { PromptOriginPayload, SessionInputPayload } from "../types";
import { isRecord, stringField } from "./timeline-message-parts";

export const SESSION_MESSAGE_PART_NAME = "data-compozy-session-message";
export const SESSION_PROMPT_TOOL_NAME = "compozy__session_prompt";
const ORIGIN_KIND_SESSION = "session";
const SYNTHETIC_KIND_SESSION_REPLY = "session_reply";

/** Who sent a session message (`PromptOriginMeta`); the daemon stamps it, the client never infers it. */
export interface SessionMessageOrigin {
  sessionId: string;
  workspaceId: string;
  /** Provider mark for the avatar; `null` when the daemon recorded none. */
  agentName: string | null;
  /** The sender's title when it sent; the card shows the current title (Gap 6). */
  titleAtSend: string | null;
  hop: number;
  notifyOnComplete: boolean;
  replyWatchId: string | null;
}

/** How a reply ended (`reason` on the reply wake). Anything newer reads `unknown`. */
export type SessionReplyOutcome = "completed" | "failed" | "canceled" | "dropped" | "unknown";

/** A `session_reply` wake (S2), from `metadata.custom.synthetic`. */
export interface SessionReplyMeta {
  /** The reply watch id; matches the sent card's `reply_watch_id`. */
  watchId: string;
  targetSessionId: string;
  targetWorkspaceId: string;
  targetAgentName: string | null;
  outcome: SessionReplyOutcome;
  /** The target's answer (or the error summary when it failed); `""` when the turn left none. */
  text: string;
  truncated: boolean;
}

/**
 * How a sent message actually reached the target: `steer` and `interrupt` only
 * when the daemon reported an interrupt-then-prompt delivery, else `queue` (no
 * chip, Gap 3). A steer to an idle target started directly and reads `queue`.
 */
export type SessionMessageMode = "queue" | "steer" | "interrupt";

/** The call behind a "Sent to" card: in flight, admitted, or refused. */
export type SessionSentCallState = "sending" | "sent" | "failed";

/** The `data-compozy-session-message` part (S3). */
export interface SessionSentMessagePart {
  toolCallId: string;
  targetSessionId: string;
  targetWorkspaceId: string;
  messageId: string | null;
  mode: SessionMessageMode;
  replyWatchId: string | null;
  state: SessionSentCallState;
}

const REPLY_OUTCOMES: ReadonlySet<string> = new Set<SessionReplyOutcome>([
  "completed",
  "failed",
  "canceled",
  "dropped",
  "unknown",
]);

function customMetadata(metadata: unknown): Record<string, unknown> | null {
  if (!isRecord(metadata)) return null;
  return isRecord(metadata.custom) ? metadata.custom : metadata;
}

function trimmedOrNull(value: string | undefined): string | null {
  const text = value?.trim();
  return text ? text : null;
}

/** The generated `PromptOriginMeta` DTO, or `null` when it is absent or not a session origin. */
export function sessionMessageOriginFromPayload(
  origin: PromptOriginPayload | null | undefined
): SessionMessageOrigin | null {
  if (!origin || origin.kind !== ORIGIN_KIND_SESSION) return null;
  const sessionId = origin.session_id.trim();
  if (!sessionId) return null;
  return {
    sessionId,
    workspaceId: origin.workspace_id.trim(),
    agentName: trimmedOrNull(origin.agent_name),
    titleAtSend: trimmedOrNull(origin.title_at_send),
    hop: Number.isFinite(origin.hop) ? origin.hop : 0,
    notifyOnComplete: origin.notify_on_complete === true,
    replyWatchId: trimmedOrNull(origin.reply_watch_id),
  };
}

// UI message metadata is untyped on the wire (`metadata: unknown`): the origin
// key carries the same DTO as the event and queue payloads, checked here once.
function originPayloadFromRecord(value: unknown): PromptOriginPayload | null {
  if (!isRecord(value)) return null;
  const { kind, session_id: sessionId, workspace_id: workspaceId, hop } = value;
  if (typeof kind !== "string" || typeof sessionId !== "string") return null;
  return {
    kind,
    session_id: sessionId,
    workspace_id: typeof workspaceId === "string" ? workspaceId : "",
    hop: typeof hop === "number" ? hop : 0,
    agent_name: stringField(value, "agent_name"),
    title_at_send: stringField(value, "title_at_send"),
    notify_on_complete: value.notify_on_complete === true,
    reply_watch_id: stringField(value, "reply_watch_id"),
  };
}

/** The sender of a user message (S1); `null` for the operator's own prompts. */
export function sessionMessageOrigin(metadata: unknown): SessionMessageOrigin | null {
  return sessionMessageOriginFromPayload(originPayloadFromRecord(customMetadata(metadata)?.origin));
}

/** The sender of a queued input (S4); `null` for operator and other-actor entries. */
export function queuedInputOrigin(input: SessionInputPayload): SessionMessageOrigin | null {
  return sessionMessageOriginFromPayload(input.origin);
}

/**
 * `transcript.inputUISyntheticMeta`: the typed subset of a synthetic wake the
 * UI metadata carries (`metadata.synthetic`). `summary` (the reply text) is
 * projected only for `session_reply`.
 */
interface SessionReplySyntheticPayload {
  kind: string;
  wake_event_id?: string;
  child_session_id?: string;
  child_workspace_id?: string;
  child_agent_name?: string;
  reason?: string;
  /** The reply text, bounded to 12,000 runes by the daemon. */
  summary?: string;
  reply_truncated?: boolean;
  hop?: number;
}

function replySyntheticPayload(value: unknown): SessionReplySyntheticPayload | null {
  if (!isRecord(value) || value.kind !== SYNTHETIC_KIND_SESSION_REPLY) return null;
  return {
    kind: SYNTHETIC_KIND_SESSION_REPLY,
    wake_event_id: stringField(value, "wake_event_id"),
    child_session_id: stringField(value, "child_session_id"),
    child_workspace_id: stringField(value, "child_workspace_id"),
    child_agent_name: stringField(value, "child_agent_name"),
    reason: stringField(value, "reason"),
    summary: stringField(value, "summary"),
    reply_truncated: value.reply_truncated === true,
    hop: typeof value.hop === "number" ? value.hop : undefined,
  };
}

/** The reply wake of a system message (S2); `null` for every other synthetic kind. */
export function sessionReplyMeta(metadata: unknown): SessionReplyMeta | null {
  const synthetic = replySyntheticPayload(customMetadata(metadata)?.synthetic);
  if (!synthetic) return null;
  const watchId = trimmedOrNull(synthetic.wake_event_id);
  const targetSessionId = trimmedOrNull(synthetic.child_session_id);
  if (!watchId || !targetSessionId) return null;
  const reason = synthetic.reason?.trim() ?? "";
  return {
    watchId,
    targetSessionId,
    targetWorkspaceId: synthetic.child_workspace_id?.trim() ?? "",
    targetAgentName: trimmedOrNull(synthetic.child_agent_name),
    outcome: REPLY_OUTCOMES.has(reason) ? (reason as SessionReplyOutcome) : "unknown",
    text: synthetic.summary?.trim() ?? "",
    truncated: synthetic.reply_truncated === true,
  };
}

/** `transcript.UISessionMessagePayload`: the `data-compozy-session-message` part. */
interface SessionMessagePartPayload {
  tool_call_id: string;
  target_session_id: string;
  target_workspace_id?: string;
  message_id?: string;
  /** The requested mode (`queue` · `steer` · `interrupt`). */
  mode?: string;
  /** The result's delivery: `none` · `direct` · `after_turn` · `interrupt_then_prompt`; absent until the call returns. */
  delivery?: string;
  reply_watch_id?: string;
  /** `running` while the call is in flight, `done` once it returned, `error` when it failed. */
  state: string;
}

function sessionMessagePartPayload(data: unknown): SessionMessagePartPayload | null {
  if (!isRecord(data)) return null;
  const toolCallId = stringField(data, "tool_call_id");
  const targetSessionId = stringField(data, "target_session_id");
  if (toolCallId === undefined || targetSessionId === undefined) return null;
  return {
    tool_call_id: toolCallId,
    target_session_id: targetSessionId,
    target_workspace_id: stringField(data, "target_workspace_id"),
    message_id: stringField(data, "message_id"),
    mode: stringField(data, "mode"),
    delivery: stringField(data, "delivery"),
    reply_watch_id: stringField(data, "reply_watch_id"),
    state: stringField(data, "state") ?? "",
  };
}

const SENT_CALL_STATE: Record<string, SessionSentCallState> = {
  running: "sending",
  done: "sent",
  error: "failed",
};

const INTERRUPT_THEN_PROMPT = "interrupt_then_prompt";

// The requested mode earns its chip only when the result says the target's
// turn was steered or interrupted; `direct` (idle target), `after_turn` and
// `none` read as plain delivery, and so does a call with no result yet.
function sentMode(mode: string | undefined, delivery: string | undefined): SessionMessageMode {
  if (delivery !== INTERRUPT_THEN_PROMPT) return "queue";
  return mode === "steer" || mode === "interrupt" ? mode : "queue";
}

/** The S3 part, or `null` when the value is not one the card can name. */
export function sessionSentMessagePart(name: string, data: unknown): SessionSentMessagePart | null {
  if (name !== SESSION_MESSAGE_PART_NAME) return null;
  const part = sessionMessagePartPayload(data);
  const targetSessionId = trimmedOrNull(part?.target_session_id);
  if (!part || !targetSessionId) return null;
  return {
    toolCallId: part.tool_call_id.trim(),
    targetSessionId,
    targetWorkspaceId: part.target_workspace_id?.trim() ?? "",
    messageId: trimmedOrNull(part.message_id),
    mode: sentMode(part.mode?.trim(), part.delivery?.trim()),
    replyWatchId: trimmedOrNull(part.reply_watch_id),
    // An unknown state from a newer daemon reads as admitted: no spinner it cannot end.
    state: SENT_CALL_STATE[part.state.trim()] ?? "sent",
  };
}

interface ThreadMessageShape {
  role?: string;
  metadata?: unknown;
}

const repliesByMessages = new WeakMap<object, ReadonlyMap<string, SessionReplyOutcome>>();

/**
 * Reply outcomes keyed by watch id over the loaded thread messages, so a sent
 * card resolves "Waiting for reply" when its reply card exists. Cached per
 * message list identity.
 */
export function sessionReplyOutcomes(
  messages: readonly ThreadMessageShape[]
): ReadonlyMap<string, SessionReplyOutcome> {
  const cached = repliesByMessages.get(messages);
  if (cached) return cached;
  const outcomes = new Map<string, SessionReplyOutcome>();
  for (const message of messages) {
    const reply = sessionReplyMeta(message.metadata);
    if (reply) outcomes.set(reply.watchId, reply.outcome);
  }
  repliesByMessages.set(messages, outcomes);
  return outcomes;
}

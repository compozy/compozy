// Session messages (`_uiux.md` S1–S4): the one wire → view mapping for the
// daemon's typed facts about agent-to-agent messages. Every surface reads these
// view models, never the raw payloads, so the shapes in `_dx.md` change here
// only:
// - `metadata.custom.origin` on a user message (`PromptOriginMeta`, S1);
// - `metadata.custom.synthetic` with kind `session_reply` on a system message (S2);
// - the `data-compozy-session-message` part beside a `compozy__session_prompt` call (S3);
// - `origin` on a queued input (`SessionInputPayload.origin`, S4).
// S1/S4 read the generated `PromptOriginPayload`; S2/S3 still follow the
// `_dx.md` shapes until their generated types land.
// The reply wake's text is daemon-authored; identity, outcome and links come
// only from the typed fields, never from that text.

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
  truncated: boolean;
}

/** How a sent message reached the target: only steer and interrupt earn a chip (Gap 3). */
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
  /** The daemon's refusal sentence when the part carries one. */
  error: string | null;
}

const REPLY_OUTCOMES: ReadonlySet<string> = new Set<SessionReplyOutcome>([
  "completed",
  "failed",
  "canceled",
  "dropped",
  "unknown",
]);

function trimmed(record: Record<string, unknown>, key: string): string | null {
  const value = stringField(record, key)?.trim();
  return value ? value : null;
}

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

/** The reply wake of a system message (S2); `null` for every other synthetic kind. */
export function sessionReplyMeta(metadata: unknown): SessionReplyMeta | null {
  const synthetic = customMetadata(metadata)?.synthetic;
  if (!isRecord(synthetic) || synthetic.kind !== SYNTHETIC_KIND_SESSION_REPLY) return null;
  const watchId = trimmed(synthetic, "wake_event_id");
  const targetSessionId = trimmed(synthetic, "child_session_id");
  if (!watchId || !targetSessionId) return null;
  const reason = trimmed(synthetic, "reason") ?? "";
  return {
    watchId,
    targetSessionId,
    targetWorkspaceId: trimmed(synthetic, "child_workspace_id") ?? "",
    targetAgentName: trimmed(synthetic, "child_agent_name"),
    outcome: REPLY_OUTCOMES.has(reason) ? (reason as SessionReplyOutcome) : "unknown",
    truncated: synthetic.reply_truncated === true,
  };
}

const NO_REPLY_TEXT = "(no reply text)";
const REPLY_SEPARATOR = "\n---\n";
const TRUNCATION_NOTE = "[Reply truncated at";

/**
 * The target's answer inside the daemon's reply wake (`_dx.md` §Reply wake):
 * the text between the header's `---` and the truncation note, or `""` when the
 * turn left none. Only the body is lifted; identity and outcome never come from
 * this text.
 */
export function sessionReplyText(wakeText: string): string {
  const separator = wakeText.indexOf(REPLY_SEPARATOR);
  let body = separator === -1 ? wakeText : wakeText.slice(separator + REPLY_SEPARATOR.length);
  const note = body.lastIndexOf(TRUNCATION_NOTE);
  if (note !== -1) body = body.slice(0, note);
  body = body.trim();
  return body === NO_REPLY_TEXT ? "" : body;
}

function sentCallState(raw: string | null): SessionSentCallState {
  switch (raw) {
    case "failed":
    case "error":
    case "output-error":
      return "failed";
    case "sending":
    case "running":
    case "streaming":
    case "input-streaming":
    case "input-available":
      return "sending";
    default:
      return "sent";
  }
}

function sentMode(raw: string | null): SessionMessageMode {
  return raw === "steer" || raw === "interrupt" ? raw : "queue";
}

/** The S3 part, or `null` when the value is not one the card can name. */
export function sessionSentMessagePart(name: string, data: unknown): SessionSentMessagePart | null {
  if (name !== SESSION_MESSAGE_PART_NAME || !isRecord(data)) return null;
  const targetSessionId = trimmed(data, "target_session_id");
  if (!targetSessionId) return null;
  return {
    toolCallId: trimmed(data, "tool_call_id") ?? "",
    targetSessionId,
    targetWorkspaceId: trimmed(data, "target_workspace_id") ?? "",
    messageId: trimmed(data, "message_id"),
    mode: sentMode(trimmed(data, "mode")),
    replyWatchId: trimmed(data, "reply_watch_id"),
    state: sentCallState(trimmed(data, "state")),
    error: trimmed(data, "error"),
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

// "Sent to" projection for the sender's transcript (`_uiux.md` S3). The daemon
// projects one `data-compozy-session-message` part per `compozy__session_prompt`
// call, as it does `data-compozy-subagent` for delegations. A call that has a
// part is drawn as the card, never as a tool row, failed or not: the card owns
// "Could not send to {title}". The card renders where its part sits.

import {
  SESSION_MESSAGE_PART_NAME,
  sessionSentMessagePart,
} from "@/systems/session/lib/session-message-payload";
import type {
  SessionTimelineDataPart,
  SessionTimelinePart,
  SessionTimelineToolPart,
} from "@/systems/session/lib/session-timeline-parts";

import type { SessionSentMessageRow } from "./session-timeline-rows";

export function isSessionMessagePart(part: SessionTimelineDataPart): boolean {
  return sessionSentMessagePart(part.name, part.data) !== null;
}

/** Any session-message part, including one the card cannot name (no target yet, an unread shape). */
export function isSessionMessageDataPart(part: SessionTimelineDataPart): boolean {
  return part.name === SESSION_MESSAGE_PART_NAME;
}

export interface SessionMessagePartition {
  /** The flow with every card-owned `compozy__session_prompt` call removed. */
  flow: readonly SessionTimelinePart[];
  /** The removed calls by tool call id: the card reads the message text and error from them. */
  tools: ReadonlyMap<string, SessionTimelineToolPart>;
  /** Flow parts that directly follow a removed call: prose on either side is not joined. */
  boundaries: ReadonlySet<SessionTimelinePart>;
}

const NO_TOOLS: ReadonlyMap<string, SessionTimelineToolPart> = new Map();
const NO_BOUNDARIES: ReadonlySet<SessionTimelinePart> = new Set();

/**
 * A send made inside a provider-native subagent belongs to that subagent's card. The call's tool part
 * carries the attribution on both the live stream and the history; its card part inherits it here so
 * the subagent partition can nest both.
 */
export function attributeSessionMessageParts(
  parts: readonly SessionTimelinePart[]
): readonly SessionTimelinePart[] {
  const parents = new Map<string, string>();
  for (const part of parts) {
    if (part.kind === "tool" && part.parentToolCallId)
      parents.set(part.toolCallId, part.parentToolCallId);
  }
  if (parents.size === 0) return parts;
  return parts.map(part => {
    if (part.kind !== "data" || part.parentToolCallId) return part;
    const toolCallId = sessionSentMessagePart(part.name, part.data)?.toolCallId;
    const parentToolCallId = toolCallId ? parents.get(toolCallId) : undefined;
    return parentToolCallId ? { ...part, parentToolCallId } : part;
  });
}

export function partitionSessionMessageParts(
  parts: readonly SessionTimelinePart[]
): SessionMessagePartition {
  const owned = new Set<string>();
  for (const part of parts) {
    if (part.kind !== "data") continue;
    const data = sessionSentMessagePart(part.name, part.data);
    if (data && data.toolCallId !== "") owned.add(data.toolCallId);
  }
  if (owned.size === 0) return { flow: parts, tools: NO_TOOLS, boundaries: NO_BOUNDARIES };

  const flow: SessionTimelinePart[] = [];
  const tools = new Map<string, SessionTimelineToolPart>();
  const boundaries = new Set<SessionTimelinePart>();
  let removed = false;
  for (const part of parts) {
    if (part.kind === "tool" && owned.has(part.toolCallId)) {
      tools.set(part.toolCallId, part);
      removed = true;
      continue;
    }
    if (removed) boundaries.add(part);
    removed = false;
    flow.push(part);
  }
  return { flow, tools, boundaries };
}

export function sentMessageRow(
  part: SessionTimelineDataPart,
  tools: ReadonlyMap<string, SessionTimelineToolPart>
): SessionSentMessageRow {
  const toolCallId = sessionSentMessagePart(part.name, part.data)?.toolCallId ?? "";
  return {
    kind: "session-message",
    id: `session-message:${toolCallId || part.id}`,
    turnId: part.turnId,
    timestamp: part.timestamp,
    part,
    toolPart: tools.get(toolCallId) ?? null,
  };
}

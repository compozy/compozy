// Explicit replies (#760): a session message may name the message it answers
// (`in_reply_to`). This index correlates both ends inside one loaded
// transcript, from the daemon's typed facts only:
// - the sender's side holds the "Sent to" part whose `message_id` was answered;
// - the target's side holds the received prompt, whose thread message id is
//   that same message id.

import { isRecord, stringField } from "./timeline-message-parts";
import {
  SESSION_MESSAGE_PART_NAME,
  sessionMessageOrigin,
  sessionSentMessagePart,
} from "./session-message-payload";

interface ReplyIndexMessage {
  id: string;
  role?: string;
  content?: unknown;
  metadata?: unknown;
}

export interface SessionMessageReplyIndex {
  /** The thread message that holds the "Sent to" card for a message id. */
  sentByMessageId: ReadonlyMap<string, string>;
  /** Message ids some inbound message in this transcript answers. */
  answered: ReadonlySet<string>;
  /** Every thread message id; a received prompt's id is its message id. */
  threadMessageIds: ReadonlySet<string>;
}

const indexes = new WeakMap<object, SessionMessageReplyIndex>();

function partName(part: Record<string, unknown>): string | null {
  const type = stringField(part, "type");
  if (type === "data") {
    const name = stringField(part, "name");
    return name ? `data-${name}` : null;
  }
  return type?.startsWith("data-") ? type : null;
}

function sentMessageIds(message: ReplyIndexMessage): string[] {
  if (message.role !== "assistant" || !Array.isArray(message.content)) return [];
  const ids: string[] = [];
  for (const part of message.content) {
    if (!isRecord(part) || partName(part) !== SESSION_MESSAGE_PART_NAME) continue;
    const messageId = sessionSentMessagePart(SESSION_MESSAGE_PART_NAME, part.data)?.messageId;
    if (messageId) ids.push(messageId);
  }
  return ids;
}

/** The reply index over the loaded messages, cached per message list identity. */
export function sessionMessageReplyIndex(
  messages: readonly ReplyIndexMessage[]
): SessionMessageReplyIndex {
  const cached = indexes.get(messages);
  if (cached) return cached;
  const sentByMessageId = new Map<string, string>();
  const answered = new Set<string>();
  const threadMessageIds = new Set<string>();
  for (const message of messages) {
    threadMessageIds.add(message.id);
    for (const messageId of sentMessageIds(message)) sentByMessageId.set(messageId, message.id);
    const inReplyTo =
      message.role === "user" ? sessionMessageOrigin(message.metadata)?.inReplyTo : undefined;
    if (inReplyTo) answered.add(inReplyTo);
  }
  const index = { sentByMessageId, answered, threadMessageIds };
  indexes.set(messages, index);
  return index;
}

/**
 * The thread message holding the message `inReplyTo` names: this session's
 * "Sent to" card, or the received prompt itself. `null` when it is not loaded.
 */
export function sessionReplyTargetMessageId(
  index: SessionMessageReplyIndex,
  inReplyTo: string | null | undefined
): string | null {
  if (!inReplyTo) return null;
  return (
    index.sentByMessageId.get(inReplyTo) ??
    (index.threadMessageIds.has(inReplyTo) ? inReplyTo : null)
  );
}

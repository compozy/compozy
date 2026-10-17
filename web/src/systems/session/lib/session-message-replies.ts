// Explicit replies (#760): a session message may name the message it answers
// (`in_reply_to`). This index correlates both ends inside one loaded
// transcript, from the daemon's typed facts only:
// - the sender's side holds the "Sent to" part whose `message_id` was answered;
// - the target's side holds the received prompt, whose thread message id is
//   that same message id.
// Message ids are chosen per send, so the same id may go to several targets:
// sent cards and answers are keyed by the peer session plus the message id.

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
  /** The thread message holding the "Sent to" card, by `sessionReplyKey(target, messageId)`. */
  sentByMessageId: ReadonlyMap<string, string>;
  /** `sessionReplyKey(answering session, messageId)` for every answer in this transcript. */
  answered: ReadonlySet<string>;
  /** Every thread message id; a received prompt's id is its message id. */
  threadMessageIds: ReadonlySet<string>;
}

const indexes = new WeakMap<object, SessionMessageReplyIndex>();

/** The key that pairs one sent message with its answers: the peer session and the message id. */
export function sessionReplyKey(peerSessionId: string, messageId: string): string {
  return `${peerSessionId}\u0000${messageId}`;
}

function partName(part: Record<string, unknown>): string | null {
  const type = stringField(part, "type");
  if (type === "data") {
    const name = stringField(part, "name");
    return name ? `data-${name}` : null;
  }
  return type?.startsWith("data-") ? type : null;
}

function sentMessageKeys(message: ReplyIndexMessage): string[] {
  if (message.role !== "assistant" || !Array.isArray(message.content)) return [];
  const keys: string[] = [];
  for (const part of message.content) {
    if (!isRecord(part) || partName(part) !== SESSION_MESSAGE_PART_NAME) continue;
    const sent = sessionSentMessagePart(SESSION_MESSAGE_PART_NAME, part.data);
    if (sent?.messageId) keys.push(sessionReplyKey(sent.targetSessionId, sent.messageId));
  }
  return keys;
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
    for (const key of sentMessageKeys(message)) sentByMessageId.set(key, message.id);
    const origin = message.role === "user" ? sessionMessageOrigin(message.metadata) : null;
    if (origin?.inReplyTo) answered.add(sessionReplyKey(origin.sessionId, origin.inReplyTo));
  }
  const index = { sentByMessageId, answered, threadMessageIds };
  indexes.set(messages, index);
  return index;
}

/**
 * The thread message holding the message `inReplyTo` names: this session's
 * "Sent to" card for `peerSessionId`, or the received prompt itself. `null`
 * when it is not loaded.
 */
export function sessionReplyTargetMessageId(
  index: SessionMessageReplyIndex,
  inReplyTo: string | null | undefined,
  peerSessionId: string
): string | null {
  if (!inReplyTo) return null;
  return (
    index.sentByMessageId.get(sessionReplyKey(peerSessionId, inReplyTo)) ??
    (index.threadMessageIds.has(inReplyTo) ? inReplyTo : null)
  );
}

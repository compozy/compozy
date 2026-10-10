// The other session a session-message surface names (S1–S4), and the copy
// built from it. COPY.md §6 "Session Message Terms".

/**
 * The other session a message names: its current title, or `null` once its
 * detail read settled as gone ("a deleted session", no link).
 */
export interface SessionMessageParty {
  sessionId: string;
  workspaceId: string;
  /**
   * `null` = deleted: the label reads "a deleted session" and is not a link —
   * unless `pending`, when the read has not answered yet and nothing is named.
   */
  title: string | null;
  /** The detail read is in flight and no title was recorded: the label waits (aria-busy). */
  pending?: boolean;
  /** Provider mark for the avatar; `null` draws the bot glyph. */
  agentName: string | null;
  /** Set only when the party lives in another workspace than this transcript (Gap 5). */
  workspaceName?: string | null;
}

export interface SessionMessageOpenOptions {
  /** ⌘/Ctrl held: open the session in a new window. */
  newWindow: boolean;
}

export type SessionMessageOpen = (
  party: SessionMessageParty,
  options: SessionMessageOpenOptions
) => void;

export const DELETED_SESSION_LABEL = "a deleted session";

/** The verb phrases that name the other session in an accessible name. */
export type SessionPartyVerb =
  | "Message from"
  | "Reply from"
  | "Sending to"
  | "Sent to"
  | "Could not send to";

// While the label read is pending nothing is named, so each verb phrase
// stands alone as a noun phrase instead of dangling ("Reply from, …").
const PENDING_PHRASE: Record<SessionPartyVerb, string> = {
  "Message from": "Message",
  "Reply from": "Reply",
  "Sending to": "Sending message",
  "Sent to": "Sent message",
  "Could not send to": "Message not sent",
};

/**
 * "{verb} {title}" for accessible names; the deleted form never names a title,
 * and a pending one names nothing until the read settles ("Reply", "Sent message").
 */
export function sessionPartyPhrase(verb: SessionPartyVerb, party: SessionMessageParty): string {
  if (party.pending) return PENDING_PHRASE[verb];
  return `${verb} ${party.title ?? DELETED_SESSION_LABEL}`;
}

/** The left-aligned card frame of agent-authored turns: the subagent card's grammar. */
export const SESSION_MESSAGE_FRAME_CLASS =
  "w-full max-w-140 self-start rounded-md border border-line-soft bg-canvas-soft text-fg";

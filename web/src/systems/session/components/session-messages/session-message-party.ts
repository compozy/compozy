// The other session a session-message surface names (S1–S4), and the copy
// built from it. COPY.md §6 "Session Message Terms".

/**
 * The other session a message names: its current title, or `null` once its
 * detail read settled as gone ("a deleted session", no link).
 */
export interface SessionMessageParty {
  sessionId: string;
  workspaceId: string;
  /** `null` = deleted: the label reads "a deleted session" and is not a link. */
  title: string | null;
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

/** "{verb} {title}" for accessible names; the deleted form never names a title. */
export function sessionPartyPhrase(verb: string, party: SessionMessageParty): string {
  return `${verb} ${party.title ?? DELETED_SESSION_LABEL}`;
}

/** The left-aligned card frame of agent-authored turns: the subagent card's grammar. */
export const SESSION_MESSAGE_FRAME_CLASS =
  "w-full max-w-140 self-start rounded-md border border-line-soft bg-canvas-soft text-fg";

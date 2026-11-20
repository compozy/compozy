import type { StateGlyphState } from "@compozy/ui";

import type { SessionSideItem } from "../adapters/session-side-api";
import type { SessionPayload } from "../types";
import { toSessionBadge } from "./session-badge";
import { hasActivePrompt } from "./session-running";

/**
 * Side chat read models (`_spec.md` side chat, `_uiux.md` S1–S3). A side chat is a hidden user
 * session with lineage kind `side`, listed only under its source session.
 */

/** Product copy for the side chat surfaces; `COPY.md` §Side Chat Terms owns these words. */
export const SIDE_CHAT_COPY = {
  toggle: "Side chats",
  newSideChat: "New side chat",
  closeSideChat: "Close side chat",
  hide: "Hide side chats",
  untitled: "Side chat",
  questionLabel: "Side chat question",
  questionPlaceholder: "Ask a side question. The session keeps working.",
  send: "Ask",
  opening: "Opening the side chat…",
  openFailed: "Couldn't open the side chat.",
  retry: "Retry",
  closeConfirmTitle: "Stop this side chat and close it?",
  closeConfirmBody: "It's still answering. Closing stops it and removes it from the side chats.",
  closeConfirm: "Stop and close",
  closeFailed: "Couldn't close the side chat.",
  attachmentsRefused: "Side chat questions can't include files yet. Send the question alone.",
  stoppedNote: "This side chat stopped. Sending a message resumes it.",
  deleteLine: "Also deletes its side chats.",
  listFailed: "Couldn't load side chats.",
  nested: "Side chats can't open side chats.",
  unsupported: "Side chats are available on your own sessions.",
} as const;

/** A side chat is a session whose lineage kind is `side`. */
export function isSideChatSession(session: Pick<SessionPayload, "lineage">): boolean {
  return session.lineage?.kind === "side";
}

/**
 * Why a session can't hold side chats, or null when it can (Business Rule 2). Mirrors the daemon's
 * `/side` availability (`internal/session/side_ingress.go`): a side chat can't nest one, and only
 * a user session that no agent spawned can hold them.
 */
export function sideChatHostRefusal(
  session: Pick<SessionPayload, "lineage" | "type">
): string | null {
  if (isSideChatSession(session)) return SIDE_CHAT_COPY.nested;
  if ((session.type ?? "user") !== "user" || session.lineage?.kind === "spawn") {
    return SIDE_CHAT_COPY.unsupported;
  }
  return null;
}

export function canHostSideChats(session: Pick<SessionPayload, "lineage" | "type">): boolean {
  return sideChatHostRefusal(session) === null;
}

/**
 * A composer submission whose leading token is `/side`: `message` is the question, empty for a
 * bare `/side` (which opens a new, empty side chat and sends nothing).
 */
export function parseSideCommand(text: string): { message: string } | null {
  const match = /^\/side(?:\s+([\s\S]*))?$/.exec(text.trim());
  if (!match) return null;
  return { message: (match[1] ?? "").trim() };
}

export type SideMainStatusKind =
  | "working"
  | "needs-approval"
  | "needs-input"
  | "finished"
  | "failed"
  | "stopped";

export interface SideMainStatus {
  kind: SideMainStatusKind;
  label: string;
  glyph: StateGlyphState;
}

const MAIN_STATUS: Record<SideMainStatusKind, SideMainStatus> = {
  working: { kind: "working", label: "Main working", glyph: "running" },
  "needs-approval": { kind: "needs-approval", label: "Main needs approval", glyph: "attention" },
  "needs-input": { kind: "needs-input", label: "Main needs input", glyph: "attention" },
  finished: { kind: "finished", label: "Main finished", glyph: "done" },
  failed: { kind: "failed", label: "Main failed", glyph: "failed" },
  stopped: { kind: "stopped", label: "Main stopped", glyph: "stopped" },
};

export function sideMainStatusOf(kind: SideMainStatusKind): SideMainStatus {
  return MAIN_STATUS[kind];
}

/**
 * The main session's state as the side panel shows it (US-005). Truthful UI: while the main
 * session's state is not known (no read yet, or its stream is reconnecting), or it reads a state
 * outside this vocabulary (stuck, unhealthy, unknown), the chip shows nothing, never a stale value.
 * "Main working" shows only while a prompt is in flight.
 */
export function sideMainStatus(
  session: SessionPayload | undefined,
  known: boolean
): SideMainStatus | null {
  if (!session || !known) return null;
  const badge = toSessionBadge(session.badge);
  if (badge === "waiting-for-auth") return MAIN_STATUS["needs-approval"];
  if (badge === "waiting-for-input") return MAIN_STATUS["needs-input"];
  if (badge === "failed") return MAIN_STATUS.failed;
  if (badge === "stopped" || session.state === "stopped") return MAIN_STATUS.stopped;
  if (
    badge === "hung" ||
    badge === "unhealthy" ||
    badge === "unknown" ||
    badge === "needs-attention"
  ) {
    return null;
  }
  if (hasActivePrompt(session)) return MAIN_STATUS.working;
  return MAIN_STATUS.finished;
}

/** A side chat asks for a permission or an answer: the head toggle shows the attention dot. */
export function sideChatNeedsYou(side: SessionSideItem): boolean {
  return side.pending_permission_count > 0 || side.pending_clarify_count > 0;
}

export function sideChatTitle(side: Pick<SessionSideItem, "name">): string {
  return side.name.trim() || SIDE_CHAT_COPY.untitled;
}

/** The daemon list is ordered newest first; closed side chats never show in the switcher. */
export function openSideChats(sides: readonly SessionSideItem[]): SessionSideItem[] {
  return sides.filter(side => side.archived_at === null);
}

/**
 * Which side chat the panel shows when it opens without an explicit target (US-003.AC-2): the
 * one last viewed in this window while it is still open, else the most recently updated.
 */
export function defaultSideChatId(
  sides: readonly SessionSideItem[],
  lastViewedId: string | null
): string | null {
  const open = openSideChats(sides);
  if (lastViewedId && open.some(side => side.id === lastViewedId)) return lastViewedId;
  return open[0]?.id ?? null;
}

/** After closing `closedId`, the panel moves to the next most recent side chat or hides (US-008.EC-1). */
export function nextSideChatId(sides: readonly SessionSideItem[], closedId: string): string | null {
  return openSideChats(sides).find(side => side.id !== closedId)?.id ?? null;
}

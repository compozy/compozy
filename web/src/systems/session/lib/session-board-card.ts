/**
 * Pure view model for one board card (`_uiux.md` S5, US-013/US-014). The card
 * shows at most one status: the needs-you reason first, else the shared PR
 * pill (`pullRequestPill`, the same mapping behind the topbar chip). Color
 * marks only a decision or readiness.
 */
import type { PillTone, StateGlyphState } from "@compozy/ui";

import { pullRequestPill, pullRequestShape, type PullRequestShape } from "./pull-request-signal";
import type { SessionBoardColumnId, SessionBoardPayload } from "./session-board-contract";
import { ENDED_EARLY_ATTENTION } from "./session-attention-reason";
import { sessionBadgeSignal } from "./session-badge";
import { type SessionOriginKind, sessionOriginView } from "./session-origin";
import { BACK_FROM_LATER_REASON } from "./session-attention-reason";
import { pendingPermissionCount } from "./session-pending-interactions";

export interface SessionBoardCardStatus {
  label: string;
  tone: PillTone;
}

export type SessionBoardPrShape = PullRequestShape;

export interface SessionBoardCardMeta {
  /** PR shape when the card names a PR; a bare branch otherwise. */
  shape: SessionBoardPrShape | "branch" | null;
  branch: string | null;
  number: number | null;
  /** All projects without a PR: the workspace name takes the meta slot. */
  workspaceName: string | null;
}

export interface SessionBoardCardRollup {
  total: number;
  needsYou: boolean;
  /** Per-column counts for the tooltip, in board order. */
  breakdown: { label: string; count: number }[];
}

/**
 * The card's lineage line. A child names its parent; a fork or continuation is a
 * top-level session, so it names its source as provenance instead.
 */
export interface SessionBoardCardLineage {
  kind: "child" | SessionOriginKind;
  /** `Child of` / `Forked from` / `Continued from`, ahead of the title in the card's name. */
  verb: string;
  title: string;
}

export interface SessionBoardCardWaiting {
  glyph: StateGlyphState;
  word: "Waiting" | "Monitoring";
}

function needsYouReason(session: SessionBoardPayload): SessionBoardCardStatus | null {
  // The badge leads: a failure or a pending answer outranks the return, as the
  // daemon's badge order does. Only the plain needs-attention badge a return
  // date leaves behind reads "Back from Later".
  switch (session.badge) {
    case "failed":
      return { label: "Failed", tone: "danger" };
    case "waiting-for-auth":
      return { label: "Auth", tone: "accent" };
    case "waiting-for-input":
      return pendingPermissionCount(session) > 0
        ? { label: "Approval", tone: "accent" }
        : { label: "Input", tone: "accent" };
    case "needs-attention":
      if (session.returned_from_later_at) return { label: BACK_FROM_LATER_REASON, tone: "accent" };
      if (session.attention === ENDED_EARLY_ATTENTION)
        return { label: "Stopped early", tone: "accent" };
      return { label: sessionBadgeSignal(session.badge).displayLabel, tone: "accent" };
    default:
      return null;
  }
}

function pullRequestStatus(session: SessionBoardPayload): SessionBoardCardStatus | null {
  return pullRequestPill(session.pull_request, session.pull_request_lookup);
}

export function sessionBoardCardStatus(
  session: SessionBoardPayload,
  column: SessionBoardColumnId | undefined = session.board_column
): SessionBoardCardStatus | null {
  if (column === "needs_you") return needsYouReason(session) ?? pullRequestStatus(session);
  return pullRequestStatus(session);
}

export function sessionBoardCardMeta(
  session: SessionBoardPayload,
  workspaceName: string | null
): SessionBoardCardMeta {
  const shape = session.pull_request ? pullRequestShape(session.pull_request) : null;
  const branch = session.branch?.trim() || null;
  if (workspaceName !== null && shape === null) {
    return { shape: null, branch: null, number: null, workspaceName };
  }
  if (shape !== null) {
    return { shape, branch, number: session.pull_request!.number, workspaceName: null };
  }
  return { shape: branch ? "branch" : null, branch, number: null, workspaceName: null };
}

export function sessionBoardCardRollup(
  session: SessionBoardPayload
): SessionBoardCardRollup | null {
  const children = session.children;
  if (!children || children.total <= 0) return null;
  return {
    total: children.total,
    needsYou: children.needs_you > 0,
    breakdown: [
      { label: "Needs you", count: children.needs_you },
      { label: "Working", count: children.working },
      { label: "Waiting", count: children.waiting },
      { label: "In review", count: children.in_review },
      { label: "Done", count: children.done },
    ],
  };
}

export function sessionBoardCardLineage(
  session: SessionBoardPayload
): SessionBoardCardLineage | null {
  const title = session.parent_title?.trim() ?? "";
  // An unreadable source has no title, but the origin still names it, as the
  // transcript divider does (its agent for a continuation, else its id).
  const origin = sessionOriginView(session, title ? { title } : null);
  if (origin !== null) {
    return { kind: origin.kind, verb: origin.verb, title: origin.dividerSubject };
  }
  return title ? { kind: "child", verb: "Child of", title } : null;
}

/** A parked card's footer names the wake source it waits on (Waiting column, #1075). */
export function sessionBoardCardWaiting(
  session: SessionBoardPayload
): SessionBoardCardWaiting | null {
  if (session.badge !== "waiting") return null;
  return session.waiting_on === "monitor"
    ? { glyph: "monitoring", word: "Monitoring" }
    : { glyph: "delegated", word: "Waiting" };
}

/** `done` is the daemon's finished-unseen class: the card carries the unread dot. */
export function sessionBoardCardUnread(session: SessionBoardPayload): boolean {
  return session.badge === "done";
}

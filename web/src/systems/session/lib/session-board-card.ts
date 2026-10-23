/**
 * Pure view model for one board card (`_uiux.md` S5, US-013/US-014). The card
 * shows at most one status: the needs-you reason first, else the most
 * actionable PR signal (reason › failing › changes › conflicts › ready ›
 * draft › merged › closed). Color marks only a decision or readiness.
 */
import type { PillTone, StateGlyphState } from "@compozy/ui";

import type { SessionBoardColumnId, SessionBoardPayload } from "./session-board-contract";
import { sessionBadgeSignal } from "./session-badge";
import { pendingPermissionCount } from "./session-pending-interactions";

export interface SessionBoardCardStatus {
  label: string;
  tone: PillTone;
}

export type SessionBoardPrShape = "open" | "draft" | "merged" | "closed";

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

export interface SessionBoardCardWaiting {
  glyph: StateGlyphState;
  word: "Waiting" | "Monitoring";
}

function needsYouReason(session: SessionBoardPayload): SessionBoardCardStatus | null {
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
      return { label: sessionBadgeSignal(session.badge).displayLabel, tone: "accent" };
    default:
      return null;
  }
}

/** Ready only when the daemon says so and every fact is current (Business Rule 9). */
function readyToMerge(session: SessionBoardPayload): boolean {
  const lookup = session.pull_request_lookup;
  return (
    session.pull_request?.ready_to_merge === true &&
    lookup?.state === "ok" &&
    lookup.stale === false
  );
}

function pullRequestStatus(session: SessionBoardPayload): SessionBoardCardStatus | null {
  const pr = session.pull_request;
  if (!pr) return null;
  if (pr.state === "merged") return { label: "Merged", tone: "info" };
  if (pr.state === "closed") return { label: "Closed", tone: "neutral" };
  if (pr.checks.state === "failing") return { label: "Checks failing", tone: "danger" };
  if (pr.review_decision === "changes_requested") {
    return { label: "Changes requested", tone: "danger" };
  }
  if (pr.mergeable === "conflicting") return { label: "Conflicts", tone: "danger" };
  if (readyToMerge(session)) return { label: "Ready to merge", tone: "success" };
  if (pr.draft === true) return { label: "Draft", tone: "neutral" };
  return null;
}

export function sessionBoardCardStatus(
  session: SessionBoardPayload,
  column: SessionBoardColumnId | undefined = session.board_column
): SessionBoardCardStatus | null {
  if (column === "needs_you") return needsYouReason(session) ?? pullRequestStatus(session);
  return pullRequestStatus(session);
}

function pullRequestShape(session: SessionBoardPayload): SessionBoardPrShape | null {
  const pr = session.pull_request;
  if (!pr) return null;
  if (pr.state === "merged" || pr.state === "closed") return pr.state;
  return pr.draft === true ? "draft" : "open";
}

export function sessionBoardCardMeta(
  session: SessionBoardPayload,
  workspaceName: string | null
): SessionBoardCardMeta {
  const shape = pullRequestShape(session);
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
      { label: "In review", count: children.in_review },
      { label: "Done", count: children.done },
    ],
  };
}

/** A parked card would read as running; its footer names why it waits. */
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

/**
 * Pure attention derivation for the shell: bell sections, badge counts, and the
 * row projections every attention surface renders.
 *
 * Two authority rules hold here and nowhere else:
 *
 * 1. **Counts come from the daemon's summary projection, never from row pages.**
 *    Rows are cursor-paginated, so counting them would drift the moment the
 *    operator passes a page boundary. `GET /api/sessions/attention-summary`
 *    returns exact cross-workspace totals and is the only session count source.
 * 2. **Staleness is first-class.** A stale source contributes zero to every
 *    count and fires no notification, but its already-listed rows stay
 *    renderable and clickable as a fallback jump (US-005.EC-2) — marked stale
 *    rather than silently dropped.
 *
 * `done` lives in its own Finished section and never inflates needs-you.
 */
import {
  isFinishedBadge,
  isNeedsYouBadge,
  pendingInteractionReason,
  sessionBadgeSignal,
  toSessionBadge,
  type SessionBadgeToken,
  type SessionPayload,
} from "@/systems/session";
import type { TaskDashboardView, TaskListItem } from "@/systems/tasks";

import { compareAttentionRecency } from "./attention-order";

export interface OsAttentionBadges {
  sessions?: number;
  tasks?: number;
}

export interface OsSessionAttentionRow {
  kind: "session";
  id: string;
  title: string;
  agentName: string;
  workspaceId: string;
  /** Human workspace name when known, else the id — never a fabricated label. */
  workspaceLabel: string;
  badge: SessionBadgeToken;
  /** Sanitized pending-interaction title, else the state word. */
  reason: string;
  changedAt: string;
  /** Muted workspaces keep their rows and counts; only delivery is silenced. */
  muted: boolean;
  /** Source disconnected: the row still jumps, but it counts for nothing. */
  stale: boolean;
}

export interface OsTaskAttentionRow {
  kind: "task";
  id: string;
  title: string;
  identifier?: string;
}

export interface OsLoopNodeAttentionRow {
  kind: "loop-node";
  id: "waiting" | "attention";
  title: string;
  state: "waiting" | "attention";
}

export type OsAttentionRow = OsSessionAttentionRow | OsTaskAttentionRow | OsLoopNodeAttentionRow;

export interface OsAttentionSections {
  /** Questions, permissions, failures, task approvals, loop nodes. */
  needsYou: OsAttentionRow[];
  /** Finished-unseen sessions — visible, never counted. */
  finished: OsSessionAttentionRow[];
}

export interface OsAttentionSummary {
  needsYou: number;
  finished: number;
}

const LOOP_NODE_ATTENTION_ROWS: readonly OsLoopNodeAttentionRow[] = [
  {
    kind: "loop-node",
    id: "waiting",
    title: "Loop nodes waiting on you",
    state: "waiting",
  },
  {
    kind: "loop-node",
    id: "attention",
    title: "Loop nodes needing attention",
    state: "attention",
  },
];

/** The needs-you class: a question, a permission gate, or a failure. */
export function isNeedsYouSession(session: SessionPayload): boolean {
  return isNeedsYouBadge(session.badge);
}

/** Finished-unseen work — the bell's second section. */
export function isFinishedSession(session: SessionPayload): boolean {
  return isFinishedBadge(session.badge);
}

function visibleCount(count: number, stale: boolean): number | undefined {
  return !stale && count > 0 ? count : undefined;
}

export interface DeriveAttentionBadgesInput {
  /** Exact cross-workspace counts from the daemon; null while unavailable. */
  summary: OsAttentionSummary | null;
  summaryStale: boolean;
  dashboard: TaskDashboardView | null;
  tasksStale: boolean;
}

export function deriveAttentionBadges(input: DeriveAttentionBadgesInput): OsAttentionBadges {
  return {
    sessions: visibleCount(input.summary?.needsYou ?? 0, input.summaryStale),
    tasks: visibleCount(input.dashboard?.totals.awaiting_approval_tasks ?? 0, input.tasksStale),
  };
}

export interface DeriveAttentionSectionsInput {
  /** Cross-workspace needs-you and finished sessions, already fetched. */
  sessions: readonly SessionPayload[];
  sessionRowsStale: boolean;
  /** Workspace id → display name. A missing entry falls back to the id. */
  workspaceLabels: ReadonlyMap<string, string>;
  mutedWorkspaceIds: ReadonlySet<string>;
  tasks: readonly TaskListItem[];
  taskRowsStale: boolean;
  /** Daemon presence probes — the row renders only when that state has an item. */
  loopWaitingPresent: boolean;
  loopAttentionPresent: boolean;
}

/**
 * Why this row is here, in plain words. The daemon's sanitized pending-interaction
 * title wins; `done` carries the one phrase that explains the marker; anything
 * else falls back to the exact state word rather than inventing a sentence.
 */
function rowReason(session: SessionPayload, badge: SessionBadgeToken): string {
  const pending = pendingInteractionReason(session);
  if (pending !== null) return pending;
  if (badge === "done") return "finished while you were away";
  return sessionBadgeSignal(badge).label;
}

function toSessionRow(
  session: SessionPayload,
  input: DeriveAttentionSectionsInput
): OsSessionAttentionRow {
  const workspaceId = session.workspace_id ?? "";
  const badge = toSessionBadge(session.badge);
  return {
    kind: "session",
    id: session.id,
    title: session.name?.trim() || session.id,
    agentName: session.agent_name,
    workspaceId,
    workspaceLabel: input.workspaceLabels.get(workspaceId) ?? workspaceId,
    badge,
    reason: rowReason(session, badge),
    changedAt: session.attention_changed_at ?? session.updated_at,
    muted: input.mutedWorkspaceIds.has(workspaceId),
    stale: input.sessionRowsStale,
  };
}

/**
 * Needs you first, Finished second; inside each, the newest transition first.
 * Sections render only when populated — no empty headers.
 */
export function deriveAttentionSections(input: DeriveAttentionSectionsInput): OsAttentionSections {
  const needsYouSessions = input.sessions.filter(isNeedsYouSession).sort(compareAttentionRecency);
  const finishedSessions = input.sessions.filter(isFinishedSession).sort(compareAttentionRecency);

  const taskRows: OsTaskAttentionRow[] = [];
  if (!input.taskRowsStale) {
    for (const task of input.tasks) {
      if (task.approval_state !== "pending") continue;
      taskRows.push({
        kind: "task",
        id: task.id,
        title: task.title,
        ...(task.identifier ? { identifier: task.identifier } : {}),
      });
    }
  }

  const loopRows: OsLoopNodeAttentionRow[] = [];
  if (input.loopWaitingPresent) loopRows.push(LOOP_NODE_ATTENTION_ROWS[0]);
  if (input.loopAttentionPresent) loopRows.push(LOOP_NODE_ATTENTION_ROWS[1]);

  return {
    needsYou: [
      ...needsYouSessions.map(session => toSessionRow(session, input)),
      ...taskRows,
      ...loopRows,
    ],
    finished: finishedSessions.map(session => toSessionRow(session, input)),
  };
}

export function attentionCount(badges: OsAttentionBadges): number {
  return (badges.sessions ?? 0) + (badges.tasks ?? 0);
}

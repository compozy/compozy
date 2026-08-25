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
import type { LoopRequestAttentionItem } from "@/systems/loops";
import type { CallAttentionCause, CallAttentionRow } from "@/systems/agent-comms";

import { compareAttentionRecency } from "./attention-order";

export interface OsAttentionBadges {
  sessions?: number;
  tasks?: number;
  loops?: number;
  /** Delegations that need a look. Lights the Agents tile. */
  calls?: number;
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

export interface OsLoopRequestAttentionRow {
  kind: "loop-request";
  id: string;
  title: string;
  workspaceId: string;
  workspaceLabel: string;
  runId: string;
  loopName: string;
  nodeId: string;
  itemIndex: number;
  requestKind: "ask" | "review";
  openedAt: string;
  expiresAt?: string;
  stale: boolean;
}

/**
 * A delegation that needs the operator.
 *
 * Exactly three causes reach this row, and one of them — `blocked-on-decision` —
 * is a fact about the *child session* rather than about the call. It is joined
 * in by `@/systems/agent-comms`, which also reports which child sessions it
 * covered so their bare session rows can be suppressed: the call row says the
 * same thing with the agent and the tree attached, which is strictly more useful.
 */
export interface OsCallAttentionRow {
  kind: "call";
  /** The call id, or `tree:<root>` once a storm has been coalesced. */
  id: string;
  cause: CallAttentionCause;
  title: string;
  callId: string;
  rootSessionId: string;
  changedAt: string;
  /** More than one when this row stands for a whole tree that went wrong. */
  count: number;
  stale: boolean;
}

export type OsAttentionRow =
  | OsSessionAttentionRow
  | OsTaskAttentionRow
  | OsLoopNodeAttentionRow
  | OsLoopRequestAttentionRow
  | OsCallAttentionRow;

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
  loopsPending?: number;
  /**
   * Delegation causes needing a look, already counted by the daemon
   * (`CallsResponse.total` over an exact state filter) and already zeroed by
   * `@/systems/agent-comms` when its source is stale.
   */
  callsNeedsYou?: number;
}

export function deriveAttentionBadges(input: DeriveAttentionBadgesInput): OsAttentionBadges {
  const loops = visibleCount(input.loopsPending ?? 0, false);
  const calls = visibleCount(input.callsNeedsYou ?? 0, false);
  return {
    sessions: visibleCount(input.summary?.needsYou ?? 0, input.summaryStale),
    tasks: visibleCount(input.dashboard?.totals.awaiting_approval_tasks ?? 0, input.tasksStale),
    ...(loops === undefined ? {} : { loops }),
    ...(calls === undefined ? {} : { calls }),
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
  loopRequests?: readonly LoopRequestAttentionItem[];
  /** Delegation rows, already coalesced per tree by the agent-comms system. */
  callRows?: readonly CallAttentionRow[];
  callRowsStale?: boolean;
  /**
   * Child sessions a call row already speaks for. Their bare session rows are
   * suppressed so one blocked child is one row, not two.
   */
  callCoveredSessionIds?: ReadonlySet<string>;
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
const NO_COVERED_SESSIONS: ReadonlySet<string> = new Set();

/** Plain-language reason per delegation cause. The state word stays in the chip. */
const CALL_CAUSE_REASON: Record<CallAttentionCause, string> = {
  "invalid-result": "the answer didn't match what was asked, even after a retry",
  "completed-without-result": "finished but sent nothing back",
  "blocked-on-decision": "waiting on your decision to continue",
};

function toCallRow(row: CallAttentionRow, stale: boolean): OsCallAttentionRow {
  return {
    kind: "call",
    id: row.id,
    cause: row.cause,
    title:
      row.count > 1
        ? `${row.count} calls need your look in this tree`
        : `${row.agentName ?? row.callId} · ${CALL_CAUSE_REASON[row.cause]}`,
    callId: row.callId,
    rootSessionId: row.rootSessionId,
    changedAt: row.changedAt,
    count: row.count,
    stale,
  };
}

export function deriveAttentionSections(input: DeriveAttentionSectionsInput): OsAttentionSections {
  // A blocked child already has a call row naming its agent and its tree; the
  // bare session row would say strictly less about the same situation.
  const covered = input.callCoveredSessionIds ?? NO_COVERED_SESSIONS;
  const sessions =
    covered.size === 0
      ? input.sessions
      : input.sessions.filter(session => !covered.has(session.id));
  const needsYouSessions = sessions.filter(isNeedsYouSession).sort(compareAttentionRecency);
  const finishedSessions = sessions.filter(isFinishedSession).sort(compareAttentionRecency);

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

  const requestRows: OsLoopRequestAttentionRow[] = (input.loopRequests ?? []).map(
    ({ request, workspaceId, workspaceLabel, stale }) => ({
      kind: "loop-request",
      id: `${workspaceId}:${request.loop_run_id}:${request.node_id}:${request.item_index}`,
      title: request.node_id,
      workspaceId,
      workspaceLabel,
      runId: request.loop_run_id,
      loopName: request.loop_name?.trim() || request.loop_run_id,
      nodeId: request.node_id,
      itemIndex: request.item_index,
      requestKind: request.kind === "review" ? "review" : "ask",
      openedAt: request.opened_at,
      ...(request.expires_at ? { expiresAt: request.expires_at } : {}),
      stale,
    })
  );

  const callRows = (input.callRows ?? []).map(row => toCallRow(row, input.callRowsStale ?? false));

  return {
    needsYou: [
      ...needsYouSessions.map(session => toSessionRow(session, input)),
      ...callRows,
      ...taskRows,
      ...requestRows,
      ...loopRows,
    ],
    finished: finishedSessions.map(session => toSessionRow(session, input)),
  };
}

export function attentionCount(badges: OsAttentionBadges): number {
  return (badges.sessions ?? 0) + (badges.tasks ?? 0) + (badges.loops ?? 0) + (badges.calls ?? 0);
}

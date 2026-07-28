import type { SessionPayload } from "@/systems/session";
import type { TaskDashboardView, TaskListItem } from "@/systems/tasks";

export interface OsAttentionBadges {
  sessions?: number;
  tasks?: number;
}

export interface OsSessionAttentionRow {
  kind: "session";
  id: string;
  title: string;
  agentName: string;
}

export interface OsTaskAttentionRow {
  kind: "task";
  id: string;
  title: string;
  identifier?: string;
}

export type OsAttentionRow = OsSessionAttentionRow | OsTaskAttentionRow;

export function isWaitingSession(session: SessionPayload): boolean {
  return session.badge === "waiting-for-auth";
}

function visibleCount(count: number, stale: boolean): number | undefined {
  return !stale && count > 0 ? count : undefined;
}

export function deriveAttentionBadges(input: {
  sessions: readonly SessionPayload[];
  sessionsStale: boolean;
  dashboard: TaskDashboardView | null;
  tasksStale: boolean;
}): OsAttentionBadges {
  return {
    sessions: visibleCount(input.sessions.filter(isWaitingSession).length, input.sessionsStale),
    tasks: visibleCount(input.dashboard?.totals.awaiting_approval_tasks ?? 0, input.tasksStale),
  };
}

export function deriveAttentionRows(input: {
  sessions: readonly SessionPayload[];
  sessionRowsStale: boolean;
  tasks: readonly TaskListItem[];
  taskRowsStale: boolean;
}): OsAttentionRow[] {
  const sessionRows: OsSessionAttentionRow[] = [];
  if (!input.sessionRowsStale) {
    for (const session of input.sessions) {
      if (!isWaitingSession(session)) continue;
      sessionRows.push({
        kind: "session",
        id: session.id,
        title: session.name?.trim() || session.id,
        agentName: session.agent_name,
      });
    }
  }
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

  return [...sessionRows, ...taskRows];
}

export function attentionCount(badges: OsAttentionBadges): number {
  return (badges.sessions ?? 0) + (badges.tasks ?? 0);
}

import type { TaskListItem } from "../types";

export interface TaskListTree {
  /** Tasks rendered as top-level rows: no parent, or parent not on this page. */
  roots: TaskListItem[];
  /** Children keyed by parent task id, in the incoming list order. */
  childrenByParent: Map<string, TaskListItem[]>;
}

/**
 * Nest every task whose parent is present in the same page under that parent.
 * A child whose parent is not loaded stays a root so nothing silently vanishes
 * behind pagination.
 */
export function buildTaskListTree(tasks: TaskListItem[]): TaskListTree {
  const byId = new Set(tasks.map(task => task.id));
  const roots: TaskListItem[] = [];
  const childrenByParent = new Map<string, TaskListItem[]>();
  for (const task of tasks) {
    const parentId = task.parent_task_id ?? "";
    if (parentId !== "" && byId.has(parentId)) {
      const siblings = childrenByParent.get(parentId);
      if (siblings) {
        siblings.push(task);
      } else {
        childrenByParent.set(parentId, [task]);
      }
      continue;
    }
    roots.push(task);
  }
  return { roots, childrenByParent };
}

export interface SubtaskSignalSummary {
  total: number;
  running: number;
  needsAttention: number;
  blocked: number;
  failed: number;
  done: number;
}

/** Aggregate child statuses so a collapsed parent never hides an escalation. */
export function summarizeSubtaskSignals(children: TaskListItem[]): SubtaskSignalSummary {
  const summary: SubtaskSignalSummary = {
    total: children.length,
    running: 0,
    needsAttention: 0,
    blocked: 0,
    failed: 0,
    done: 0,
  };
  for (const child of children) {
    switch (child.status) {
      case "in_progress":
        summary.running += 1;
        break;
      case "needs_attention":
        summary.needsAttention += 1;
        break;
      case "blocked":
        summary.blocked += 1;
        break;
      case "failed":
      case "canceled":
        summary.failed += 1;
        break;
      case "completed":
        summary.done += 1;
        break;
      default:
        break;
    }
  }
  return summary;
}

/** Human summary line for a collapsed subtask group, escalations first. */
export function subtaskSummaryLabel(summary: SubtaskSignalSummary): string {
  const parts: string[] = [`${summary.total} ${summary.total === 1 ? "subtask" : "subtasks"}`];
  if (summary.needsAttention > 0) parts.push(`${summary.needsAttention} needs attention`);
  if (summary.blocked > 0) parts.push(`${summary.blocked} blocked`);
  if (summary.failed > 0) parts.push(`${summary.failed} failed`);
  if (summary.running > 0) parts.push(`${summary.running} running`);
  if (summary.done > 0) parts.push(`${summary.done} done`);
  return parts.join(" · ");
}

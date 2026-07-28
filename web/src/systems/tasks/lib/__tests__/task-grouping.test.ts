import { describe, expect, it } from "vitest";

import {
  getKanbanColumns,
  getTaskListGroups,
  groupTasksForKanban,
  groupTasksForList,
  listGroupDotProps,
  resolveKanbanColumnId,
  resolveTaskListGroupId,
} from "../task-grouping";
import type { TaskListItem } from "../../types";

function buildTask(id: string, status: TaskListItem["status"]): TaskListItem {
  return {
    id,
    title: `Task ${id}`,
    status,
    scope: "workspace",
    origin: { kind: "web", ref: "op" },
    created_at: "2026-04-11T09:00:00Z",
    updated_at: "2026-04-11T09:00:00Z",
    created_by: { kind: "human", ref: "op" },
  } as TaskListItem;
}

describe("task-grouping", () => {
  it("Should attach a stable StatusDot tone and variant to every list group", () => {
    const expected: Record<string, { tone: string; variant: string }> = {
      active: { tone: "accent", variant: "ring" },
      // blocked → danger, needs_attention → warning: distinct escalation buckets,
      // no coercion (matches TASK_STATUS_TONE).
      blocked: { tone: "danger", variant: "solid" },
      needs_attention: { tone: "warning", variant: "solid" },
      queued: { tone: "faint", variant: "ring" },
      done: { tone: "faint", variant: "solid" },
      failed: { tone: "danger", variant: "solid" },
    };

    for (const group of getTaskListGroups()) {
      expect(listGroupDotProps(group.id)).toEqual(expected[group.id]);
      expect(group.dotTone).toBe(expected[group.id]?.tone);
      expect(group.dotVariant).toBe(expected[group.id]?.variant);
    }
  });

  it("Should return the canonical columns including a distinct needs_attention column in order", () => {
    const columns = getKanbanColumns();
    expect(columns.map(column => column.id)).toEqual([
      "pending",
      "in_progress",
      "blocked",
      "needs_attention",
      "done",
    ]);
    expect(columns.map(column => column.label)).toEqual([
      "Pending",
      "In progress",
      "Blocked",
      "Needs attention",
      "Done",
    ]);
  });

  it("Should map production task statuses to their kanban column, needs_attention distinct from blocked", () => {
    expect(resolveKanbanColumnId("draft")).toBe("pending");
    expect(resolveKanbanColumnId("pending")).toBe("pending");
    expect(resolveKanbanColumnId("ready")).toBe("pending");
    expect(resolveKanbanColumnId("blocked")).toBe("blocked");
    expect(resolveKanbanColumnId("needs_attention")).toBe("needs_attention");
    expect(resolveKanbanColumnId("in_progress")).toBe("in_progress");
    expect(resolveKanbanColumnId("completed")).toBe("done");
    expect(resolveKanbanColumnId("failed")).toBe("done");
    expect(resolveKanbanColumnId("canceled")).toBe("done");
  });

  it("Should reject non-production status aliases", () => {
    expect(resolveKanbanColumnId("running")).toBeNull();
    expect(resolveKanbanColumnId("done")).toBeNull();
  });

  it("Should group tasks into the five columns, routing needs_attention to its own column", () => {
    const tasks: TaskListItem[] = [
      buildTask("a", "draft"),
      buildTask("b", "pending"),
      buildTask("c", "ready"),
      buildTask("d", "in_progress"),
      buildTask("e", "failed"),
      buildTask("f", "canceled"),
      buildTask("g", "blocked"),
      buildTask("h", "needs_attention"),
    ];

    const groups = groupTasksForKanban(tasks);
    const byId = new Map(groups.map(group => [group.column.id, group.tasks.map(t => t.id)]));

    expect(byId.get("pending")).toEqual(["a", "b", "c"]);
    expect(byId.get("in_progress")).toEqual(["d"]);
    expect(byId.get("blocked")).toEqual(["g"]);
    expect(byId.get("needs_attention")).toEqual(["h"]);
    expect(byId.get("done")).toEqual(["e", "f"]);
    expect(groups).toHaveLength(5);
  });

  it("Should route a needs_attention task to its own list group so the escalation is visible", () => {
    expect(resolveTaskListGroupId("needs_attention")).toBe("needs_attention");
    expect(resolveTaskListGroupId("blocked")).toBe("blocked");

    const buckets = groupTasksForList([
      buildTask("g", "blocked"),
      buildTask("h", "needs_attention"),
      buildTask("d", "in_progress"),
    ]);
    const byId = new Map(buckets.map(bucket => [bucket.group.id, bucket.tasks.map(t => t.id)]));

    // The escalated task lands in its own bucket, never dropped from every group.
    expect(byId.get("needs_attention")).toEqual(["h"]);
    expect(byId.get("blocked")).toEqual(["g"]);
    expect(byId.get("active")).toEqual(["d"]);
  });
});

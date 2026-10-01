import { describe, expect, it } from "vitest";

import {
  getKanbanColumns,
  getTaskListGroups,
  groupTasksForKanban,
  groupTasksForList,
  listGroupGlyph,
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
  it("Should attach a canonical StateGlyph state to every list group and kanban column", () => {
    // blocked and needs_attention both wait on a person (attention); their labels
    // keep the two escalation buckets distinct (no coercion).
    expect(
      Object.fromEntries(getTaskListGroups().map(group => [group.id, listGroupGlyph(group.id)]))
    ).toEqual({
      active: "running",
      blocked: "attention",
      needs_attention: "attention",
      queued: "queued",
      done: "done",
      failed: "failed",
    });
    expect(Object.fromEntries(getKanbanColumns().map(column => [column.id, column.glyph]))).toEqual(
      {
        pending: "queued",
        in_progress: "running",
        blocked: "attention",
        needs_attention: "attention",
        done: "done",
      }
    );
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

  it("Should list canceled tasks under Done instead of the danger-toned Failed group", () => {
    expect(resolveTaskListGroupId("canceled")).toBe("done");
    expect(resolveTaskListGroupId("failed")).toBe("failed");
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

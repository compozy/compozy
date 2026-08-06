import { describe, expect, it } from "vitest";

import { buildTaskListTree, subtaskSummaryLabel, summarizeSubtaskSignals } from "../task-hierarchy";
import type { TaskListItem } from "../../types";

function item(id: string, overrides: Partial<TaskListItem> = {}): TaskListItem {
  return {
    id,
    title: id,
    status: "ready",
    scope: "workspace",
    created_at: "2026-08-05T10:00:00Z",
    updated_at: "2026-08-05T10:00:00Z",
    ...overrides,
  } as TaskListItem;
}

describe("buildTaskListTree", () => {
  it("Should nest children under a parent that is present on the page", () => {
    const parent = item("loop.run-1.coordinator");
    const childA = item("loop.run-1.g1.node.execute_task.0", {
      parent_task_id: "loop.run-1.coordinator",
    });
    const childB = item("loop.run-1.g1.node.review.0", {
      parent_task_id: "loop.run-1.coordinator",
    });
    const standalone = item("task-manual");

    const tree = buildTaskListTree([childA, parent, standalone, childB]);

    expect(tree.roots.map(task => task.id)).toEqual(["loop.run-1.coordinator", "task-manual"]);
    expect(tree.childrenByParent.get("loop.run-1.coordinator")?.map(task => task.id)).toEqual([
      "loop.run-1.g1.node.execute_task.0",
      "loop.run-1.g1.node.review.0",
    ]);
  });

  it("Should keep a child whose parent is not loaded as a visible root", () => {
    const orphan = item("loop.run-2.g1.node.verify.0", {
      parent_task_id: "loop.run-2.coordinator",
    });

    const tree = buildTaskListTree([orphan]);

    expect(tree.roots.map(task => task.id)).toEqual(["loop.run-2.g1.node.verify.0"]);
    expect(tree.childrenByParent.size).toBe(0);
  });
});

describe("subtaskSummaryLabel", () => {
  it("Should lead with escalations so collapsing never hides them", () => {
    const summary = summarizeSubtaskSignals([
      item("a", { status: "needs_attention" }),
      item("b", { status: "failed" }),
      item("c", { status: "in_progress" }),
      item("d", { status: "completed" }),
    ]);

    expect(subtaskSummaryLabel(summary)).toBe(
      "4 subtasks · 1 needs attention · 1 failed · 1 running · 1 done"
    );
  });
});

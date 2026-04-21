import { afterEach, describe, expect, it } from "vitest";

import { installFetchStub, matchPath } from "@/test/utils";

import { getWorkflowBoard, getWorkflowTask } from "./tasks-api";

describe("tasks api adapter", () => {
  let restore: (() => void) | null = null;

  afterEach(() => {
    restore?.();
    restore = null;
  });

  it("Should GET the workflow board with the active workspace header", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/board"),
        status: 200,
        body: {
          board: {
            workspace: {
              id: "ws-1",
              name: "one",
              root_dir: "/tmp/one",
              created_at: "2026-01-01T00:00:00Z",
              updated_at: "2026-01-01T00:00:00Z",
            },
            workflow: { id: "wf-1", slug: "alpha", workspace_id: "ws-1" },
            task_counts: { total: 1, completed: 0, pending: 1 },
            lanes: [
              {
                status: "pending",
                title: "Pending",
                items: [
                  {
                    task_id: "task_01",
                    task_number: 1,
                    title: "Bootstrap",
                    status: "pending",
                    type: "frontend",
                    updated_at: "2026-01-01T00:00:00Z",
                  },
                ],
              },
            ],
          },
        },
      },
    ]);
    restore = stub.restore;
    const result = await getWorkflowBoard({ workspaceId: "ws-1", slug: "alpha" });
    expect(result.task_counts.total).toBe(1);
    expect(result.lanes?.[0]?.items?.[0]?.task_id).toBe("task_01");
    expect(stub.calls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should surface transport errors from the board endpoint", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/ghost/board"),
        status: 404,
        body: { code: "workflow_not_found", message: "workflow missing", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await expect(getWorkflowBoard({ workspaceId: "ws-1", slug: "ghost" })).rejects.toThrow(
      /workflow missing/
    );
  });

  it("Should GET the workflow task detail with the workspace header", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/items/task_02"),
        status: 200,
        body: {
          task: {
            workspace: {
              id: "ws-1",
              name: "one",
              root_dir: "/tmp/one",
              created_at: "2026-01-01T00:00:00Z",
              updated_at: "2026-01-01T00:00:00Z",
            },
            workflow: { id: "wf-1", slug: "alpha", workspace_id: "ws-1" },
            task: {
              task_id: "task_02",
              task_number: 2,
              title: "Implement board",
              status: "in_progress",
              type: "frontend",
              depends_on: ["task_01"],
              updated_at: "2026-01-02T00:00:00Z",
            },
            document: {
              id: "task-doc",
              kind: "task",
              title: "Implement board",
              updated_at: "2026-01-02T00:00:00Z",
              markdown: "## Task body",
            },
            memory_entries: [],
            related_runs: [],
            live_tail_available: false,
          },
        },
      },
    ]);
    restore = stub.restore;
    const result = await getWorkflowTask({
      workspaceId: "ws-1",
      slug: "alpha",
      taskId: "task_02",
    });
    expect(result.task.task_id).toBe("task_02");
    expect(result.document.markdown).toContain("Task body");
    expect(stub.calls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should surface transport errors from the task detail endpoint", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/items/task_missing"),
        status: 404,
        body: { code: "task_not_found", message: "task missing", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await expect(
      getWorkflowTask({ workspaceId: "ws-1", slug: "alpha", taskId: "task_missing" })
    ).rejects.toThrow(/task missing/);
  });
});

import { afterEach, describe, expect, it } from "vitest";

import { toTransportError } from "@/lib/api-client";
import { installFetchStub, matchPath } from "@/test/utils";

import { archiveWorkflow, listWorkflows, syncWorkflow } from "./workflows-api";

describe("workflows api adapter", () => {
  let restore: (() => void) | null = null;

  afterEach(() => {
    restore?.();
    restore = null;
  });

  it("Should list workflows for the active workspace", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks"),
        status: 200,
        body: {
          workflows: [
            { id: "wf-1", slug: "alpha", workspace_id: "ws-1" },
            { id: "wf-2", slug: "beta", workspace_id: "ws-1" },
          ],
        },
      },
    ]);
    restore = stub.restore;
    const result = await listWorkflows("ws-1");
    expect(result).toHaveLength(2);
    expect(stub.calls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should POST sync with the workspace body and header", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/sync", "POST"),
        status: 200,
        body: {
          workflow_slug: "alpha",
          workspace_id: "ws-1",
          workflows_scanned: 1,
          task_items_upserted: 3,
        },
      },
    ]);
    restore = stub.restore;
    const result = await syncWorkflow({ workspaceId: "ws-1", workflowSlug: "alpha" });
    expect(result.task_items_upserted).toBe(3);
    const call = stub.calls[0];
    expect(call?.headers["x-compozy-workspace-id"]).toBe("ws-1");
    expect(call?.body).toBe(JSON.stringify({ workflow_slug: "alpha", workspace: "ws-1" }));
  });

  it("Should POST archive against the slug path", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/archive", "POST"),
        status: 200,
        body: { archived: true, archived_at: "2026-01-01T00:00:00Z" },
      },
    ]);
    restore = stub.restore;
    const result = await archiveWorkflow({ workspaceId: "ws-1", slug: "alpha" });
    expect(result.archived).toBe(true);
    expect(stub.calls[0]?.body).toBe(JSON.stringify({ workspace: "ws-1" }));
  });

  it("Should include force when retrying archive confirmation", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/archive", "POST"),
        status: 200,
        body: { archived: true, forced: true, completed_tasks: 2, resolved_review_issues: 1 },
      },
    ]);
    restore = stub.restore;
    const result = await archiveWorkflow({ workspaceId: "ws-1", slug: "alpha", force: true });
    expect(result.forced).toBe(true);
    expect(stub.calls[0]?.body).toBe(JSON.stringify({ workspace: "ws-1", force: true }));
  });

  it("Should surface transport errors from sync", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/sync", "POST"),
        status: 412,
        body: { code: "workspace_context_stale", message: "stale", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await expect(syncWorkflow({ workspaceId: "missing" })).rejects.toThrow(/stale/);
  });

  it("Should preserve transport error details for archive confirmation retries", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/archive", "POST"),
        status: 409,
        body: {
          code: "workflow_force_required",
          message: "needs confirmation",
          request_id: "req-1",
          details: {
            workflow_slug: "alpha",
            task_non_terminal: 2,
            review_unresolved: 1,
          },
        },
      },
    ]);
    restore = stub.restore;

    try {
      await archiveWorkflow({ workspaceId: "ws-1", slug: "alpha" });
      throw new Error("expected archiveWorkflow to throw");
    } catch (error) {
      expect(toTransportError(error)).toEqual({
        code: "workflow_force_required",
        message: "needs confirmation",
        request_id: "req-1",
        details: {
          workflow_slug: "alpha",
          task_non_terminal: 2,
          review_unresolved: 1,
        },
      });
    }
  });
});

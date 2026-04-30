import { afterEach, describe, expect, it } from "vitest";

import { installFetchStub, matchPath } from "@/test/utils";

import { listWorkspaces, resolveWorkspace, syncWorkspaces } from "./workspaces-api";

describe("workspaces api adapter", () => {
  let restoreFetch: (() => void) | null = null;

  afterEach(() => {
    restoreFetch?.();
    restoreFetch = null;
  });

  it("Should return the workspaces array from the payload", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 200,
        body: {
          workspaces: [
            {
              id: "ws-1",
              name: "one",
              root_dir: "/tmp/one",
              filesystem_state: "present",
              read_only: false,
              has_catalog_data: true,
              workflow_count: 1,
              run_count: 0,
              created_at: "2026-01-01T00:00:00Z",
              updated_at: "2026-01-01T00:00:00Z",
            },
          ],
        },
      },
    ]);
    restoreFetch = stub.restore;
    const result = await listWorkspaces();
    expect(result).toHaveLength(1);
    expect(result[0]?.id).toBe("ws-1");
  });

  it("Should throw the transport error message on non-success responses", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces"),
        status: 500,
        body: { code: "server_error", message: "daemon down", request_id: "req" },
      },
    ]);
    restoreFetch = stub.restore;
    await expect(listWorkspaces()).rejects.toThrow(/daemon down/);
  });

  it("Should send the requested path when resolving a workspace", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces/resolve", "POST"),
        status: 200,
        body: {
          workspace: {
            id: "ws-new",
            name: "new",
            root_dir: "/tmp/new",
            filesystem_state: "present",
            read_only: false,
            has_catalog_data: false,
            workflow_count: 0,
            run_count: 0,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      },
    ]);
    restoreFetch = stub.restore;
    const result = await resolveWorkspace({ path: "/tmp/new" });
    expect(result.id).toBe("ws-new");
    expect(stub.calls[0]?.body).toBe(JSON.stringify({ path: "/tmp/new" }));
  });

  it("Should request manual workspace sync and return the summary payload", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/workspaces/sync", "POST"),
        status: 200,
        body: {
          checked: 3,
          removed: 1,
          missing: 1,
          synced: 1,
          snapshots_upserted: 4,
          task_items_upserted: 2,
          review_rounds_upserted: 0,
          review_issues_upserted: 0,
          warnings: ["one workspace path is missing"],
        },
      },
    ]);
    restoreFetch = stub.restore;
    const result = await syncWorkspaces();
    expect(result.checked).toBe(3);
    expect(result.warnings?.[0]).toMatch(/missing/);
    expect(stub.calls[0]?.method).toBe("POST");
  });
});

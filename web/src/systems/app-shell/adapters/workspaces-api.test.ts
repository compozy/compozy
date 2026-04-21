import { afterEach, describe, expect, it } from "vitest";

import { installFetchStub, matchPath } from "@/test/utils";

import { listWorkspaces, resolveWorkspace } from "./workspaces-api";

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
});

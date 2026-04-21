import { afterEach, describe, expect, it } from "vitest";

import { installFetchStub, matchPath } from "@/test/utils";

import { getWorkflowMemoryFile, getWorkflowMemoryIndex } from "./memory-api";

describe("memory api adapter", () => {
  let restore: (() => void) | null = null;

  afterEach(() => {
    restore?.();
    restore = null;
  });

  it("Should GET the workflow memory index and surface opaque file ids", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/memory"),
        status: 200,
        body: {
          memory: {
            workspace: {
              id: "ws-1",
              name: "one",
              root_dir: "/tmp/one",
              created_at: "2026-01-01T00:00:00Z",
              updated_at: "2026-01-01T00:00:00Z",
            },
            workflow: { id: "wf-1", slug: "alpha", workspace_id: "ws-1" },
            entries: [
              {
                file_id: "file-shared",
                display_path: ".compozy/memory/alpha/MEMORY.md",
                kind: "shared",
                size_bytes: 2048,
                title: "MEMORY.md",
                updated_at: "2026-01-02T00:00:00Z",
              },
              {
                file_id: "file-task-01",
                display_path: ".compozy/memory/alpha/task_01.md",
                kind: "task",
                size_bytes: 512,
                title: "task_01.md",
                updated_at: "2026-01-02T00:01:00Z",
              },
            ],
          },
        },
      },
    ]);
    restore = stub.restore;
    const result = await getWorkflowMemoryIndex({ workspaceId: "ws-1", slug: "alpha" });
    expect(result.entries).toHaveLength(2);
    expect(result.entries?.[0]?.file_id).toBe("file-shared");
    expect(stub.calls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should GET a memory file by opaque file_id", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/memory/files/file-shared"),
        status: 200,
        body: {
          document: {
            id: "memory-file-shared",
            kind: "shared",
            title: "MEMORY.md",
            updated_at: "2026-01-02T00:00:00Z",
            markdown: "## Shared memory body",
          },
        },
      },
    ]);
    restore = stub.restore;
    const doc = await getWorkflowMemoryFile({
      workspaceId: "ws-1",
      slug: "alpha",
      fileId: "file-shared",
    });
    expect(doc.markdown).toContain("Shared memory body");
    const request = stub.calls[0];
    expect(request?.url).toContain("/api/tasks/alpha/memory/files/file-shared");
    expect(request?.url).not.toContain("MEMORY.md");
  });

  it("Should surface transport errors when the memory file is missing", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/memory/files/missing"),
        status: 404,
        body: { code: "memory_file_not_found", message: "file missing", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await expect(
      getWorkflowMemoryFile({ workspaceId: "ws-1", slug: "alpha", fileId: "missing" })
    ).rejects.toThrow(/file missing/);
  });
});

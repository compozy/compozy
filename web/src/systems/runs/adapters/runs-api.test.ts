import { afterEach, describe, expect, it } from "vitest";

import { installFetchStub, matchPath } from "@/test/utils";

import { cancelRun, getRun, getRunSnapshot, listRuns, startWorkflowRun } from "./runs-api";

describe("runs api adapter", () => {
  let restore: (() => void) | null = null;

  afterEach(() => {
    restore?.();
    restore = null;
  });

  it("Should list runs with workspace, status, and mode filters", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/runs?workspace=ws-1&status=active&mode=task&limit=25"),
        status: 200,
        body: {
          runs: [
            {
              run_id: "run-1",
              mode: "task",
              presentation_mode: "text",
              workspace_id: "ws-1",
              started_at: "2026-01-01T00:00:00Z",
              status: "running",
            },
          ],
        },
      },
    ]);
    restore = stub.restore;
    const result = await listRuns({
      workspaceId: "ws-1",
      status: "active",
      mode: "task",
      limit: 25,
    });
    expect(result).toHaveLength(1);
    expect(result[0]?.run_id).toBe("run-1");
    expect(stub.calls[0]?.method).toBe("GET");
    expect(stub.calls[0]?.url).toContain("/api/runs");
    expect(stub.calls[0]?.url).toContain("workspace=ws-1");
    expect(stub.calls[0]?.url).toContain("status=active");
    expect(stub.calls[0]?.url).toContain("mode=task");
    expect(stub.calls[0]?.url).toContain("limit=25");
  });

  it("Should omit filters when set to 'all'", async () => {
    const stub = installFetchStub([
      {
        matcher: (input, init) => {
          const url = typeof input === "string" ? input : (input as Request).url;
          return url.includes("/api/runs") && (init?.method ?? "GET") === "GET";
        },
        status: 200,
        body: { runs: [] },
      },
    ]);
    restore = stub.restore;
    await listRuns({ workspaceId: "ws-1", status: "all", mode: "all" });
    const url = stub.calls[0]?.url ?? "";
    expect(url).not.toContain("status=");
    expect(url).not.toContain("mode=");
    expect(url).toContain("workspace=ws-1");
  });

  it("Should fetch a run summary by id", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/runs/run-9"),
        status: 200,
        body: {
          run: {
            run_id: "run-9",
            mode: "task",
            presentation_mode: "text",
            workspace_id: "ws-1",
            started_at: "2026-01-01T00:00:00Z",
            status: "running",
          },
        },
      },
    ]);
    restore = stub.restore;
    const result = await getRun("run-9");
    expect(result.run_id).toBe("run-9");
  });

  it("Should fetch a run snapshot with transcript and jobs", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/runs/run-2/snapshot"),
        status: 200,
        body: {
          run: {
            run_id: "run-2",
            mode: "task",
            presentation_mode: "text",
            workspace_id: "ws-1",
            started_at: "2026-01-01T00:00:00Z",
            status: "running",
          },
          jobs: [
            {
              index: 0,
              job_id: "job-1",
              status: "running",
              updated_at: "2026-01-01T00:01:00Z",
            },
          ],
          transcript: [
            {
              content: "hello",
              role: "assistant",
              sequence: 1,
              stream: "stdout",
              timestamp: "2026-01-01T00:01:30Z",
            },
          ],
          next_cursor: "2026-01-01T00:01:30Z|00000000000000000001",
        },
      },
    ]);
    restore = stub.restore;
    const snapshot = await getRunSnapshot("run-2");
    expect(snapshot.run.run_id).toBe("run-2");
    expect(snapshot.jobs ?? []).toHaveLength(1);
    expect(snapshot.transcript ?? []).toHaveLength(1);
    expect(snapshot.next_cursor).toMatch(/^2026-01-01/);
  });

  it("Should POST run cancellation", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/runs/run-3/cancel", "POST"),
        status: 202,
        body: { accepted: true },
      },
    ]);
    restore = stub.restore;
    await expect(cancelRun({ runId: "run-3" })).resolves.toBeUndefined();
    expect(stub.calls[0]?.method).toBe("POST");
  });

  it("Should surface transport errors from cancellation", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/runs/run-gone/cancel", "POST"),
        status: 404,
        body: { code: "run_not_found", message: "unknown run", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await expect(cancelRun({ runId: "run-gone" })).rejects.toThrow(/unknown run/);
  });

  it("Should POST workflow run start with workspace header and body", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/runs", "POST"),
        status: 201,
        body: {
          run: {
            run_id: "run-new",
            mode: "task",
            presentation_mode: "text",
            workspace_id: "ws-1",
            started_at: "2026-01-01T00:00:00Z",
            status: "queued",
          },
        },
      },
    ]);
    restore = stub.restore;
    const started = await startWorkflowRun({
      workspaceId: "ws-1",
      slug: "alpha",
      body: { presentation_mode: "text" },
    });
    expect(started.run_id).toBe("run-new");
    const call = stub.calls[0];
    expect(call?.headers["x-compozy-workspace-id"]).toBe("ws-1");
    const body = JSON.parse(call?.body ?? "{}");
    expect(body.workspace).toBe("ws-1");
    expect(body.presentation_mode).toBe("text");
  });
});

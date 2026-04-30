import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, createRouter, RouterProvider } from "@tanstack/react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createTestQueryClient, installFetchStub } from "@/test/utils";

import { routeTree } from "../routeTree.gen";
import { resetActiveWorkspaceStoreForTests } from "../systems/app-shell";
import {
  setRunStreamFactoryOverrideForTests,
  type OpenRunStreamOptions,
  type RunStreamHandler,
} from "../systems/runs";

const workspaceOne = {
  id: "ws-1",
  name: "one",
  root_dir: "/tmp/one",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const runningRun = {
  run_id: "run-1",
  mode: "task",
  presentation_mode: "text",
  workspace_id: "ws-1",
  started_at: "2026-01-01T00:00:00Z",
  status: "running",
  workflow_slug: "alpha",
};

const completedRun = {
  run_id: "run-9",
  mode: "task",
  presentation_mode: "text",
  workspace_id: "ws-1",
  started_at: "2026-01-01T00:00:00Z",
  ended_at: "2026-01-01T00:01:00Z",
  status: "completed",
  workflow_slug: "beta",
};

const transcriptBody = {
  run_id: "run-1",
  messages: [
    {
      id: "msg-1",
      role: "assistant",
      parts: [{ type: "text", text: "run detail transcript" }],
    },
  ],
};

function matchUrl(pattern: string, method: string = "GET") {
  return (input: RequestInfo | URL, init?: RequestInit) => {
    const url =
      typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    const requestMethod =
      input instanceof Request ? input.method.toUpperCase() : (init?.method ?? "GET").toUpperCase();
    return url.includes(pattern) && requestMethod === method.toUpperCase();
  };
}

interface HarnessController {
  options: OpenRunStreamOptions;
  handler: RunStreamHandler;
  closed: boolean;
}

function createStreamHarness() {
  const controllers: HarnessController[] = [];
  setRunStreamFactoryOverrideForTests((options, handler) => {
    const controller: HarnessController = { options, handler, closed: false };
    controllers.push(controller);
    queueMicrotask(() => handler({ type: "open" }));
    return {
      close() {
        controller.closed = true;
      },
    };
  });
  return {
    controllers,
    teardown: () => setRunStreamFactoryOverrideForTests(null),
  };
}

async function renderApp(initialPath: string) {
  const queryClient = createTestQueryClient();
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    defaultPreload: false,
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
  await router.load();
  return { router, queryClient };
}

describe("runs console integration", () => {
  let restore: (() => void) | null = null;
  let streamHarness: ReturnType<typeof createStreamHarness> | null = null;

  beforeEach(async () => {
    resetActiveWorkspaceStoreForTests();
    window.sessionStorage.clear();
    const { useActiveWorkspaceStore } =
      await import("../systems/app-shell/stores/active-workspace-store");
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: "ws-1" });
  });

  afterEach(() => {
    restore?.();
    restore = null;
    streamHarness?.teardown();
    streamHarness = null;
    resetActiveWorkspaceStoreForTests();
    vi.clearAllMocks();
  });

  it("Should render the run inventory against the typed daemon contract", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/runs"),
        status: 200,
        body: { runs: [runningRun, completedRun] },
      },
    ]);
    restore = stub.restore;
    await renderApp("/runs");
    await screen.findByTestId("runs-list-view");
    expect(await screen.findByTestId("runs-list-row-run-1")).toBeInTheDocument();
    expect(screen.getByTestId("runs-list-row-run-9")).toBeInTheDocument();
    const runsCalls = stub.calls.filter(call => call.url.includes("/api/runs"));
    expect(runsCalls[0]?.url).toContain("workspace=ws-1");
  });

  it("Should re-query the run list when changing the status filter", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/runs"),
        status: 200,
        body: { runs: [runningRun] },
      },
    ]);
    restore = stub.restore;
    await renderApp("/runs");
    await screen.findByTestId("runs-list-view");
    await userEvent.selectOptions(screen.getByTestId("runs-filter-status"), "active");
    await waitFor(() => {
      const runsCalls = stub.calls.filter(call => call.url.includes("/api/runs"));
      expect(runsCalls.some(call => call.url.includes("status=active"))).toBe(true);
    });
  });

  it("Should render the run detail snapshot and open a live stream", async () => {
    streamHarness = createStreamHarness();
    const snapshotBody = {
      run: runningRun,
      jobs: [{ index: 0, job_id: "job-1", status: "running", updated_at: "2026-01-01T00:01:00Z" }],
      transcript: [],
      next_cursor: "2026-01-01T00:00:00Z|00000000000000000001",
    };
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/runs/run-1/snapshot"),
        status: 200,
        body: snapshotBody,
      },
      {
        matcher: matchUrl("/api/runs/run-1/transcript"),
        status: 200,
        body: transcriptBody,
      },
    ]);
    restore = stub.restore;
    await renderApp("/runs/run-1");
    expect(await screen.findByTestId("run-detail-view")).toBeInTheDocument();
    expect(screen.getByTestId("run-detail-status")).toHaveTextContent("running");
    expect(screen.getByTestId("run-detail-job-row-job-1")).toBeInTheDocument();
    await waitFor(() => expect(streamHarness!.controllers).toHaveLength(1));
    expect(streamHarness!.controllers[0]!.options.runId).toBe("run-1");
    expect(streamHarness!.controllers[0]!.options.lastEventId).toBe(
      "2026-01-01T00:00:00Z|00000000000000000001"
    );
  });

  it("Should cancel a running run through the daemon contract", async () => {
    streamHarness = createStreamHarness();
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/runs/run-1/snapshot"),
        status: 200,
        body: { run: runningRun, jobs: [], transcript: [] },
      },
      {
        matcher: matchUrl("/api/runs/run-1/transcript"),
        status: 200,
        body: transcriptBody,
      },
      {
        matcher: matchUrl("/api/runs/run-1/cancel", "POST"),
        status: 202,
        body: { accepted: true },
      },
    ]);
    restore = stub.restore;
    await renderApp("/runs/run-1");
    await screen.findByTestId("run-detail-view");
    await userEvent.click(screen.getByTestId("run-detail-cancel"));
    await waitFor(() => {
      const cancelCalls = stub.calls.filter(
        call => call.url.includes("/api/runs/run-1/cancel") && call.method === "POST"
      );
      expect(cancelCalls).toHaveLength(1);
    });
    await screen.findByTestId("run-detail-cancel-success");
  });

  it("Should invalidate the snapshot cache after the stream overflows", async () => {
    streamHarness = createStreamHarness();
    const snapshotBody = {
      run: runningRun,
      jobs: [],
      transcript: [],
      next_cursor: "2026-01-01T00:00:00Z|00000000000000000001",
    };
    let snapshotCalls = 0;
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: (input, init) => {
          const url =
            typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
          const method = (init?.method ?? "GET").toUpperCase();
          if (url.includes("/api/runs/run-1/snapshot") && method === "GET") {
            snapshotCalls += 1;
            return true;
          }
          return false;
        },
        status: 200,
        body: snapshotBody,
      },
      {
        matcher: matchUrl("/api/runs/run-1/transcript"),
        status: 200,
        body: transcriptBody,
      },
    ]);
    restore = stub.restore;
    await renderApp("/runs/run-1");
    await screen.findByTestId("run-detail-view");
    await waitFor(() => expect(streamHarness!.controllers).toHaveLength(1));
    const firstController = streamHarness!.controllers[0]!;
    firstController.handler({
      type: "overflow",
      eventId: "2026-01-01T00:00:10Z|00000000000000000010",
      payload: { reason: "replay truncated" },
    });
    expect(await screen.findByTestId("run-detail-stream-overflow")).toHaveTextContent(
      "replay truncated"
    );
    await waitFor(() => {
      expect(snapshotCalls).toBeGreaterThan(1);
    });
  });

  it("Should reconnect the stream when the operator presses the reconnect action", async () => {
    streamHarness = createStreamHarness();
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/runs/run-1/snapshot"),
        status: 200,
        body: {
          run: runningRun,
          jobs: [],
          transcript: [],
          next_cursor: "2026-01-01T00:00:00Z|00000000000000000001",
        },
      },
      {
        matcher: matchUrl("/api/runs/run-1/transcript"),
        status: 200,
        body: transcriptBody,
      },
    ]);
    restore = stub.restore;
    await renderApp("/runs/run-1");
    await screen.findByTestId("run-detail-view");
    await waitFor(() => expect(streamHarness!.controllers).toHaveLength(1));
    await userEvent.click(screen.getByTestId("run-detail-reconnect"));
    await waitFor(() => expect(streamHarness!.controllers).toHaveLength(2));
    expect(streamHarness!.controllers[0]!.closed).toBe(true);
  });
});

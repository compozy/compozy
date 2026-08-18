import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { HttpResponse } from "msw";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { createMswFetch } from "@/test/msw-fetch";
import { handlers } from "@/systems/loops/mocks";
import {
  useAmendLoopNode,
  useApproveLoopRun,
  useCreateLoop,
  useDeleteLoop,
  useForkLoopRun,
  usePatchLoop,
  usePauseLoopRun,
  usePutLoopConfig,
  useRerunLoopRun,
  useRespondLoopRequest,
  useRunLoop,
  useValidateLoop,
} from "@/systems/loops";
import {
  GRAPH_ENG_FORK_RUN_ID,
  GRAPH_ENG_RUN_ID,
} from "@/systems/loops/mocks/fixture-graph-eng-requests";

const WS = "ws_1";

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidate = vi.spyOn(queryClient, "invalidateQueries");
  const remove = vi.spyOn(queryClient, "removeQueries");
  return { queryClient, invalidate, remove, wrapper: createWrapper(queryClient) };
}

describe("loop mutation hooks", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("Should invalidate the catalog + created loop after useCreateLoop", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useCreateLoop(), { wrapper });

    await result.current.mutateAsync({
      workspaceId: WS,
      data: { fork_from_name: "implement-tasks" },
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "catalog", WS] });
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: ["loops", "detail", WS, "implement-tasks"],
    });
  });

  it("Should invalidate the fork target when useCreateLoop returns a conflict", async () => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => [
        compozyApiMock.post("/api/workspaces/{workspace_id}/loops", () =>
          HttpResponse.json({ error: "Workspace copy already exists" }, { status: 409 })
        ),
        ...handlers,
      ])
    );
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useCreateLoop(), { wrapper });

    await expect(
      result.current.mutateAsync({
        workspaceId: WS,
        data: { fork_from_name: "implement-tasks" },
      })
    ).rejects.toMatchObject({ status: 409 });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["loops", "detail", WS, "implement-tasks"],
      });
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: ["loops", "config", WS, "implement-tasks"],
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: ["loops", "annotations", WS, "implement-tasks"],
    });
  });

  it("Should invalidate definition + config + annotations after usePatchLoop", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => usePatchLoop(), { wrapper });

    await result.current.mutateAsync({
      workspaceId: WS,
      name: "implement-tasks",
      data: { definition: {} } as never,
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["loops", "detail", WS, "implement-tasks"],
      });
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: ["loops", "config", WS, "implement-tasks"],
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: ["loops", "annotations", WS, "implement-tasks"],
    });
  });

  it("Should evict exact Loop projections then invalidate the catalog after useDeleteLoop", async () => {
    const { invalidate, remove, wrapper } = setup();
    const { result } = renderHook(() => useDeleteLoop(), { wrapper });

    await result.current.mutateAsync({ workspaceId: WS, name: "implement-tasks" });

    await waitFor(() => {
      expect(remove).toHaveBeenCalledWith({
        queryKey: ["loops", "detail", WS, "implement-tasks"],
      });
    });
    expect(remove).toHaveBeenCalledWith({
      queryKey: ["loops", "config", WS, "implement-tasks"],
    });
    expect(remove).toHaveBeenCalledWith({
      queryKey: ["loops", "annotations", WS, "implement-tasks"],
    });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "catalog", WS] });
  });

  it("Should preserve cached Loop projections when useDeleteLoop fails", async () => {
    const { invalidate, remove, wrapper } = setup();
    const { result } = renderHook(() => useDeleteLoop(), { wrapper });

    await expect(
      result.current.mutateAsync({ workspaceId: WS, name: "missing-loop" })
    ).rejects.toThrow("Loop not found: missing-loop");

    expect(remove).not.toHaveBeenCalled();
    expect(invalidate).not.toHaveBeenCalled();
  });

  it("Should invalidate runs on a real run but never on a dry run", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useRunLoop(), { wrapper });

    await result.current.mutateAsync({
      workspaceId: WS,
      name: "implement-tasks",
      data: { inputs: { slug: "hook-run" } },
    });
    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "runs", WS] });
    });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "catalog", WS] });

    invalidate.mockClear();
    await result.current.mutateAsync({
      workspaceId: WS,
      name: "implement-tasks",
      data: { inputs: { slug: "hook-run" } },
      dry: true,
    });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(invalidate).not.toHaveBeenCalled();
  });

  it("Should invalidate the run detail + runs list after a run control", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => usePauseLoopRun(), { wrapper });

    await result.current.mutateAsync({ workspaceId: WS, runId: "looprun_running" });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["loops", "run-detail", WS, "looprun_running"],
      });
    });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "runs", WS] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "catalog", WS] });
  });

  it("Should invalidate the run after an approval decision", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useApproveLoopRun(), { wrapper });

    await result.current.mutateAsync({
      workspaceId: WS,
      runId: "looprun_running",
      data: { decision: "approve", gate_id: "gate_1" },
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["loops", "run-detail", WS, "looprun_running"],
      });
    });
  });

  it("Should return the validate verdict without invalidating any cache", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useValidateLoop(), { wrapper });

    const verdict = await result.current.mutateAsync({
      workspaceId: WS,
      name: "implement-tasks",
      data: { definition: {} } as never,
    });

    expect(verdict.valid).toBe(true);
    expect(invalidate).not.toHaveBeenCalled();
  });

  it("Should invalidate only the config after usePutLoopConfig", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => usePutLoopConfig(), { wrapper });

    await result.current.mutateAsync({
      workspaceId: WS,
      name: "implement-tasks",
      data: { config: { iteration_cap: 8 } },
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["loops", "config", WS, "implement-tasks"],
      });
    });
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: ["loops", "runs", WS] });
  });
});

describe("graph-completion mutation hooks", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  const REQUEST_KEYS = [
    ["loops", "run-detail", WS, GRAPH_ENG_RUN_ID],
    ["loops", "runs", WS],
    ["loops", "node-inventory", WS],
    ["loops", "requests", WS],
  ];

  it("Should reconcile an answered request through every owner key without an optimistic paint", async () => {
    const { queryClient, invalidate, wrapper } = setup();
    const setData = vi.spyOn(queryClient, "setQueryData");
    const { result } = renderHook(() => useRespondLoopRequest(), { wrapper });

    await result.current.mutateAsync({
      workspaceId: WS,
      runId: GRAPH_ENG_RUN_ID,
      nodeId: "confirm-rollout",
      data: { generation: 3, payload: { regions: ["eu"], canary: true } },
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: REQUEST_KEYS[0] });
    });
    for (const queryKey of REQUEST_KEYS) {
      expect(invalidate).toHaveBeenCalledWith({ queryKey });
    }

    expect(setData).not.toHaveBeenCalled();
  });

  it("Should still reconcile after the daemon refuses the answer", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useRespondLoopRequest(), { wrapper });

    await expect(
      result.current.mutateAsync({
        workspaceId: WS,
        runId: GRAPH_ENG_RUN_ID,
        nodeId: "confirm-rollout",
        data: { generation: 3, payload: { regions: [] } },
      })
    ).rejects.toMatchObject({ code: "request_validation_failed" });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "requests", WS] });
    });
  });

  it("Should reconcile an amendment through the request lifecycle key set", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useAmendLoopNode(), { wrapper });

    await result.current.mutateAsync({
      workspaceId: WS,
      runId: GRAPH_ENG_RUN_ID,
      nodeId: "render-notes",
      data: { payload: { risk: "medium" }, reason: "over-rated" },
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: REQUEST_KEYS[0] });
    });
    for (const queryKey of REQUEST_KEYS) {
      expect(invalidate).toHaveBeenCalledWith({ queryKey });
    }
  });

  it("Should invalidate the run and every cached comparison after a rerun", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useRerunLoopRun(), { wrapper });

    await result.current.mutateAsync({
      workspaceId: WS,
      runId: GRAPH_ENG_RUN_ID,
      data: { from_node: "render-notes" },
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["loops", "run-detail", WS, GRAPH_ENG_RUN_ID],
      });
    });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "run-diff"] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "runs", WS] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "catalog", WS] });
  });

  it("Should invalidate both sides of the lineage after a fork", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => useForkLoopRun(), { wrapper });

    const created = await result.current.mutateAsync({
      workspaceId: WS,
      runId: GRAPH_ENG_RUN_ID,
      data: { generation: 2, inputs: { severity: "p0" } },
    });
    expect(created.run.id).toBe(GRAPH_ENG_FORK_RUN_ID);

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["loops", "run-detail", WS, GRAPH_ENG_RUN_ID],
      });
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: ["loops", "run-detail", WS, GRAPH_ENG_FORK_RUN_ID],
    });
  });
});

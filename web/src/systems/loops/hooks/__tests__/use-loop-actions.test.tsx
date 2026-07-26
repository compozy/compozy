import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";
import { handlers } from "@/systems/loops/mocks";
import {
  useApproveLoopRun,
  useCreateLoop,
  useDeleteLoop,
  usePatchLoop,
  usePauseLoopRun,
  usePutLoopConfig,
  useRunLoop,
  useValidateLoop,
} from "@/systems/loops";

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
      data: { fork_from_name: "software-delivery" },
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "catalog", WS] });
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: ["loops", "detail", WS, "software-delivery"],
    });
  });

  it("Should invalidate definition + config + annotations after usePatchLoop", async () => {
    const { invalidate, wrapper } = setup();
    const { result } = renderHook(() => usePatchLoop(), { wrapper });

    await result.current.mutateAsync({
      workspaceId: WS,
      name: "software-delivery",
      data: { definition: {} } as never,
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["loops", "detail", WS, "software-delivery"],
      });
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: ["loops", "config", WS, "software-delivery"],
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: ["loops", "annotations", WS, "software-delivery"],
    });
  });

  it("Should evict exact Loop projections then invalidate the catalog after useDeleteLoop", async () => {
    const { invalidate, remove, wrapper } = setup();
    const { result } = renderHook(() => useDeleteLoop(), { wrapper });

    await result.current.mutateAsync({ workspaceId: WS, name: "software-delivery" });

    await waitFor(() => {
      expect(remove).toHaveBeenCalledWith({
        queryKey: ["loops", "detail", WS, "software-delivery"],
      });
    });
    expect(remove).toHaveBeenCalledWith({
      queryKey: ["loops", "config", WS, "software-delivery"],
    });
    expect(remove).toHaveBeenCalledWith({
      queryKey: ["loops", "annotations", WS, "software-delivery"],
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
      name: "software-delivery",
      data: { inputs: {} },
    });
    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "runs", WS] });
    });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["loops", "catalog", WS] });

    invalidate.mockClear();
    await result.current.mutateAsync({
      workspaceId: WS,
      name: "software-delivery",
      data: { inputs: {} },
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
      name: "software-delivery",
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
      name: "software-delivery",
      data: { config: { iteration_cap: 8 } },
    });

    await waitFor(() => {
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["loops", "config", WS, "software-delivery"],
      });
    });
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: ["loops", "runs", WS] });
  });
});

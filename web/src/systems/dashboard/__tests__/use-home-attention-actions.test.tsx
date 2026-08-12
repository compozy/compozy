import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

interface Deferred {
  promise: Promise<void>;
  resolve: () => void;
  reject: (error: Error) => void;
}

const deferreds = new Map<string, Deferred>();
const mutationCalls = new Map<string, number>();
const approveExecutorVersions: number[] = [];
let latestApproveExecutorVersion = 0;

function makeDeferred(id: string): Promise<void> {
  let resolve!: () => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<void>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  deferreds.set(id, { promise, resolve, reject });
  return promise;
}

function mutationStub() {
  return {
    mutateAsync: (vars: { id?: string; runId?: string }) => {
      const id = vars.id ?? vars.runId ?? "";
      mutationCalls.set(id, (mutationCalls.get(id) ?? 0) + 1);
      return makeDeferred(id);
    },
  };
}

function approveMutationStub() {
  const version = latestApproveExecutorVersion + 1;
  latestApproveExecutorVersion = version;
  const execute = (vars: { id?: string; runId?: string }) => {
    approveExecutorVersions.push(version);
    const id = vars.id ?? vars.runId ?? "";
    mutationCalls.set(id, (mutationCalls.get(id) ?? 0) + 1);
    return makeDeferred(id);
  };
  return { mutateAsync: execute };
}

vi.mock("@/systems/tasks/hooks/task-actions-public-api", () => ({
  useApproveTask: () => approveMutationStub(),
  useRejectTask: () => mutationStub(),
  useRetryTaskRun: () => mutationStub(),
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));

import { toast } from "sonner";

import { createHomeAttentionActionsLogic } from "../hooks/home-attention-actions-store";
import { useHomeAttentionActions } from "../hooks/use-home-attention-actions";

function wrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useHomeAttentionActions", () => {
  beforeEach(() => {
    deferreds.clear();
    mutationCalls.clear();
    approveExecutorVersions.length = 0;
    latestApproveExecutorVersion = 0;
    vi.mocked(toast.error).mockClear();
  });

  it("Should keep each concurrent row pending until its own request settles, in reverse order", async () => {
    const { result } = renderHook(() => useHomeAttentionActions(), { wrapper: wrapper() });

    act(() => {
      result.current.onApprove("task-a");
      result.current.onReject("task-b");
    });
    await waitFor(() => {
      expect(result.current.pendingIds.has("task-a")).toBe(true);
      expect(result.current.pendingIds.has("task-b")).toBe(true);
    });

    // Settle the second decision first: only its id clears; the first stays pending.
    deferreds.get("task-b")?.resolve();
    await waitFor(() => {
      expect(result.current.pendingIds.has("task-b")).toBe(false);
    });
    expect(result.current.pendingIds.has("task-a")).toBe(true);
    expect(result.current.resolvedById).toEqual({ "task-b": "rejected" });

    deferreds.get("task-a")?.resolve();
    await waitFor(() => {
      expect(result.current.pendingIds.has("task-a")).toBe(false);
    });
    expect(result.current.pendingIds.size).toBe(0);
  });

  it("Should reject stale settlements and execute one same-tick request per row", () => {
    const approve = vi.fn(() => new Promise<never>(() => undefined));
    const store = createHomeAttentionActionsLogic().createStore();
    const refreshOverview = vi.fn().mockResolvedValue(undefined);
    store.trigger.approveRequested({
      id: "task-a",
      execute: approve,
      refreshOverview,
    });
    const firstRequest = store.getSnapshot().context.operations["task-a"];
    if (!firstRequest) throw new Error("expected first request");
    store.trigger.approveRequested({
      id: "task-a",
      execute: approve,
      refreshOverview,
    });

    expect(approve).toHaveBeenCalledOnce();
    expect(store.getSnapshot().context.nextRequestId).toBe(1);

    const [snapshot] = store.transition(store.getSnapshot(), {
      type: "rowResolved",
      id: "task-a",
      kind: "approved",
      refreshOverview,
      requestId: firstRequest.requestId - 1,
    });

    expect(snapshot.context.operations["task-a"]).toMatchObject({
      requestId: 1,
      status: "pending",
    });
  });

  it("Should execute the mutation callback from the render that issues the intent", () => {
    const { result, rerender } = renderHook(() => useHomeAttentionActions(), {
      wrapper: wrapper(),
    });
    rerender();
    act(() => result.current.onApprove("task-current"));

    expect(approveExecutorVersions).toEqual([2]);
  });

  it("Should execute one mutation for duplicate same-tick hook intents", () => {
    const { result } = renderHook(() => useHomeAttentionActions(), { wrapper: wrapper() });

    act(() => {
      result.current.onApprove("task-a");
      result.current.onApprove("task-a");
    });

    expect(mutationCalls.get("task-a")).toBe(1);
  });

  it("Should report an approval failure without resolving the row and clear only its pending id", async () => {
    const { result } = renderHook(() => useHomeAttentionActions(), { wrapper: wrapper() });

    act(() => {
      result.current.onApprove("task-a");
    });
    await waitFor(() => {
      expect(result.current.pendingIds.has("task-a")).toBe(true);
    });

    const deferred = deferreds.get("task-a");
    if (!deferred) throw new Error("expected the approval mutation to be pending");
    const failure = new Error("Approval unavailable");
    deferred.reject(failure);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Approval unavailable");
      expect(result.current.pendingIds.has("task-a")).toBe(false);
    });
    expect(result.current.resolvedById).toEqual({});
  });

  it("Should report an accepted approval failure after the owning component unmounts", async () => {
    const { result, unmount } = renderHook(() => useHomeAttentionActions(), { wrapper: wrapper() });

    act(() => result.current.onApprove("task-unmounted"));
    const deferred = deferreds.get("task-unmounted");
    if (!deferred) throw new Error("expected the approval mutation to be pending");

    unmount();
    deferred.reject(new Error("Approval failed after leaving Home"));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Approval failed after leaving Home");
    });
  });
});

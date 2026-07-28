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
    mutateAsync: (vars: { id?: string; runId?: string }) =>
      makeDeferred(vars.id ?? vars.runId ?? ""),
  };
}

vi.mock("@/systems/tasks", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/tasks")>();
  return {
    ...actual,
    useApproveTask: (() => mutationStub()) as unknown as typeof actual.useApproveTask,
    useRejectTask: (() => mutationStub()) as unknown as typeof actual.useRejectTask,
    useRetryTaskRun: (() => mutationStub()) as unknown as typeof actual.useRetryTaskRun,
  };
});

vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));

import { toast } from "sonner";

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

    deferreds.get("task-a")?.resolve();
    await waitFor(() => {
      expect(result.current.pendingIds.has("task-a")).toBe(false);
    });
    expect(result.current.pendingIds.size).toBe(0);
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
});

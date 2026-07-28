// Suite: useSupportBundleDownload
// Invariant: support bundle polling is canceled by lifecycle reset and cannot remain pending forever.
// Boundary IN: hook mutation lifecycle, polling interval, timeout, and AbortSignal propagation.
// Boundary OUT: HTTP transport and browser file persistence.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/support-api", () => ({
  supportApi: {
    create: vi.fn(),
    download: vi.fn(),
    get: vi.fn(),
  },
}));

import { supportApi } from "../../adapters/support-api";
import type { SupportBundleOperation } from "../../types";
import { createSupportBundleDownloadLogic } from "../support-bundle-download-store";
import { useSupportBundleDownload } from "../use-support-bundle-download";

const pendingOperation: SupportBundleOperation = {
  created_at: "2026-07-21T12:00:00Z",
  operation_id: "support_001",
  status: "pending",
  status_url: "/api/support/bundles/support_001",
  updated_at: "2026-07-21T12:00:00Z",
};

const completedOperation: SupportBundleOperation = {
  ...pendingOperation,
  file_name: "support.tar.gz",
  status: "completed",
};

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("useSupportBundleDownload", () => {
  it("Should reject completion from a superseded support-bundle request", () => {
    const execute = async () => ({ fileName: "support.tar.gz", operation: pendingOperation });
    const logic = createSupportBundleDownloadLogic();
    const store = logic.createStore();
    const [first] = store.transition(store.getInitialSnapshot(), {
      type: "downloadRequested",
      execute,
      input: { yes: true },
    });
    const [second] = store.transition(first, {
      type: "downloadRequested",
      execute,
      input: { yes: true },
    });
    const [afterLateCompletion] = store.transition(second, {
      type: "downloadCompleted",
      requestId: first.context.requestId,
      result: {
        fileName: "support.tar.gz",
        operation: { ...pendingOperation, status: "completed" },
      },
    });

    expect(afterLateCompletion.context.phase).toBe("pending");
    expect(afterLateCompletion.context.requestId).toBe(second.context.requestId);
  });

  it("Should keep can checks observational while a request is active", async () => {
    let activeSignal: AbortSignal | undefined;
    const execute = (_input: { yes: true }, signal: AbortSignal) => {
      activeSignal = signal;
      return new Promise<never>((_resolve, reject) => {
        signal.addEventListener("abort", () => reject(signal.reason), { once: true });
      });
    };
    const logic = createSupportBundleDownloadLogic();
    const store = logic.createStore();

    store.trigger.downloadRequested({ execute, input: { yes: true } });
    expect(activeSignal?.aborted).toBe(false);

    expect(store.can.lifecycleReset()).toBe(true);
    expect(activeSignal?.aborted).toBe(false);

    store.trigger.lifecycleReset();
    expect(activeSignal?.aborted).toBe(true);
    await Promise.resolve();
  });

  it("Should isolate cancellation between stores created from the same logic", async () => {
    const signals: AbortSignal[] = [];
    const execute = (_input: { yes: true }, signal: AbortSignal) => {
      signals.push(signal);
      return new Promise<never>((_resolve, reject) => {
        signal.addEventListener("abort", () => reject(signal.reason), { once: true });
      });
    };
    const logic = createSupportBundleDownloadLogic();
    const firstStore = logic.createStore();
    const secondStore = logic.createStore();

    firstStore.trigger.downloadRequested({ execute, input: { yes: true } });
    secondStore.trigger.downloadRequested({ execute, input: { yes: true } });
    firstStore.trigger.lifecycleReset();

    expect(signals).toHaveLength(2);
    expect(signals[0]?.aborted).toBe(true);
    expect(signals[1]?.aborted).toBe(false);

    secondStore.trigger.lifecycleReset();
    await Promise.resolve();
  });

  it("Should execute the callback carried by the accepted request", () => {
    const currentExecute = vi.fn(() => new Promise<never>(() => undefined));
    const store = createSupportBundleDownloadLogic().createStore();

    store.trigger.downloadRequested({ execute: currentExecute, input: { yes: true } });

    expect(currentExecute).toHaveBeenCalledOnce();
    store.trigger.lifecycleReset();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(supportApi.create).mockResolvedValue(pendingOperation);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("Should abort active polling when the mutation lifecycle resets", async () => {
    vi.mocked(supportApi.get).mockImplementation((_operationId, signal) => {
      return new Promise((_resolve, reject) => {
        signal?.addEventListener("abort", () => reject(signal.reason), { once: true });
      });
    });
    const { result } = renderHook(() => useSupportBundleDownload(), { wrapper: createWrapper() });

    let request: Promise<unknown> | undefined;
    act(() => {
      request = result.current.create({ yes: true });
    });
    await waitFor(() => expect(supportApi.get).toHaveBeenCalledOnce());
    const aborted = expect(request).rejects.toMatchObject({ name: "AbortError" });

    act(() => result.current.reset());

    await aborted;
    expect(vi.mocked(supportApi.get).mock.calls[0]?.[1]).toHaveProperty("aborted", true);
  });

  it("Should reject polling that never reaches a terminal state within two minutes", async () => {
    vi.useFakeTimers();
    vi.mocked(supportApi.get).mockResolvedValue(pendingOperation);
    const { result } = renderHook(() => useSupportBundleDownload(), { wrapper: createWrapper() });

    let request: Promise<unknown> | undefined;
    act(() => {
      request = result.current.create({ yes: true });
    });
    const timedOut = expect(request).rejects.toThrow(
      "Support bundle did not complete within 2 minutes"
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(120_000);
      await timedOut;
    });
    expect(supportApi.download).not.toHaveBeenCalled();
  });

  it("Should time out when support-bundle creation never settles", async () => {
    vi.useFakeTimers();
    vi.mocked(supportApi.create).mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useSupportBundleDownload(), { wrapper: createWrapper() });

    let request: Promise<unknown> | undefined;
    act(() => {
      request = result.current.create({ yes: true });
    });
    const timedOut = expect(request).rejects.toThrow(
      "Support bundle did not complete within 2 minutes"
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(120_000);
      await timedOut;
    });
    expect(supportApi.get).not.toHaveBeenCalled();
    expect(supportApi.download).not.toHaveBeenCalled();
  });

  it("Should time out when the completed bundle download never settles", async () => {
    vi.useFakeTimers();
    vi.mocked(supportApi.get).mockResolvedValue(completedOperation);
    vi.mocked(supportApi.download).mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useSupportBundleDownload(), { wrapper: createWrapper() });

    let request: Promise<unknown> | undefined;
    act(() => {
      request = result.current.create({ yes: true });
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(supportApi.download).toHaveBeenCalledOnce();
    const timedOut = expect(request).rejects.toThrow(
      "Support bundle did not complete within 2 minutes"
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(120_000);
      await timedOut;
    });
  });
});

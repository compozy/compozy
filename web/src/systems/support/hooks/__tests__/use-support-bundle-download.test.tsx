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
import { useSupportBundleDownload } from "../use-support-bundle-download";

const pendingOperation: SupportBundleOperation = {
  created_at: "2026-07-21T12:00:00Z",
  operation_id: "support_001",
  status: "pending",
  status_url: "/api/support/bundles/support_001",
  updated_at: "2026-07-21T12:00:00Z",
};

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("useSupportBundleDownload", () => {
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

    const request = result.current.create({ yes: true });
    const timedOut = expect(request).rejects.toThrow(
      "Support bundle did not complete within 2 minutes"
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(120_000);
    });

    await timedOut;
    expect(supportApi.download).not.toHaveBeenCalled();
  });
});

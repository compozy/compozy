// Suite: operator session presence lifecycle
// Invariant: one mounted session window owns at most one acquire/renew request
// at a time, so a slow request cannot leak a second presence lease.
// Owning layer: session presence hook. No prior suite owned this lifecycle.
import { QueryClient, QueryClientProvider, useMutation } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { sessionKeys } from "../../lib/query-keys";
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/session-presence-api", () => ({
  acquireSessionPresence: vi.fn(),
  releaseSessionPresence: vi.fn(() => Promise.resolve()),
  renewSessionPresence: vi.fn(() => Promise.resolve("renewed")),
}));

import {
  acquireSessionPresence,
  releaseSessionPresence,
  renewSessionPresence,
} from "../../adapters/session-presence-api";
import { SessionApiError } from "../../adapters/session-api-errors";
import { useSessionPresence } from "../use-session-presence";

let client: QueryClient;
function wrapper({ children }: PropsWithChildren) {
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe("useSessionPresence", () => {
  it("Should stop acquiring a lease after the session is removed", async () => {
    vi.mocked(acquireSessionPresence).mockRejectedValue(new SessionApiError("gone", 404));
    renderHook(() => useSessionPresence("ws-alpha", "sess-alpha", true), { wrapper });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });
    expect(acquireSessionPresence).toHaveBeenCalledTimes(1);
    expect(renewSessionPresence).not.toHaveBeenCalled();
  });

  beforeEach(() => {
    client = new QueryClient();
    vi.useFakeTimers();
    vi.mocked(acquireSessionPresence).mockReset();
    vi.mocked(acquireSessionPresence).mockResolvedValue("lease-alpha");
    vi.mocked(releaseSessionPresence).mockReset();
    vi.mocked(releaseSessionPresence).mockResolvedValue();
    vi.mocked(renewSessionPresence).mockReset();
    vi.mocked(renewSessionPresence).mockResolvedValue("renewed");
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("Should pause presence during deletion and recover after a rejected deletion", async () => {
    let rejectDelete!: (error: Error) => void;
    const { result } = renderHook(
      () => {
        useSessionPresence("ws-alpha", "sess-alpha", true);
        return useMutation({
          mutationKey: sessionKeys.deletions(),
          mutationFn: (_target: { id: string }) =>
            new Promise<void>((_resolve, reject) => {
              rejectDelete = reject;
            }),
        });
      },
      { wrapper }
    );
    await act(async () => {
      await Promise.resolve();
    });
    await act(async () => {
      result.current.mutate({ id: "sess-alpha" });
      await vi.advanceTimersByTimeAsync(1);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15_000);
    });
    expect(renewSessionPresence).not.toHaveBeenCalled();
    expect(releaseSessionPresence).not.toHaveBeenCalled();
    expect(acquireSessionPresence).toHaveBeenCalledTimes(1);
    await act(async () => {
      rejectDelete(new Error("Deletion refused"));
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(acquireSessionPresence).toHaveBeenCalledTimes(2);
  });

  it("Should not overlap lease acquisition while the first request is pending", async () => {
    vi.mocked(acquireSessionPresence).mockReturnValue(new Promise(() => undefined));

    renderHook(() => useSessionPresence("ws-alpha", "sess-alpha", true), { wrapper });

    expect(acquireSessionPresence).toHaveBeenCalledTimes(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15_000);
    });

    expect(acquireSessionPresence).toHaveBeenCalledTimes(1);
  });

  it("Should not overlap lease renewal while the current request is pending", async () => {
    vi.mocked(acquireSessionPresence).mockResolvedValue("lease-alpha");
    vi.mocked(renewSessionPresence).mockReturnValue(new Promise(() => undefined));

    renderHook(() => useSessionPresence("ws-alpha", "sess-alpha", true), { wrapper });
    await act(async () => Promise.resolve());
    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000);
    });

    expect(renewSessionPresence).toHaveBeenCalledTimes(1);
  });

  it("Should retain and retry the current lease after a transient renewal failure", async () => {
    vi.mocked(renewSessionPresence)
      .mockResolvedValueOnce("retryable-error")
      .mockResolvedValueOnce("renewed");

    renderHook(() => useSessionPresence("ws-alpha", "sess-alpha", true), { wrapper });
    await act(async () => Promise.resolve());
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });

    expect(acquireSessionPresence).toHaveBeenCalledTimes(1);
    expect(renewSessionPresence).toHaveBeenCalledTimes(2);
    expect(renewSessionPresence).toHaveBeenNthCalledWith(
      2,
      "ws-alpha",
      "sess-alpha",
      "lease-alpha",
      expect.any(AbortSignal)
    );
  });

  it("Should acquire a new lease after the daemon rejects the current lease", async () => {
    vi.mocked(renewSessionPresence).mockResolvedValueOnce("lease-invalid");

    renderHook(() => useSessionPresence("ws-alpha", "sess-alpha", true), { wrapper });
    await act(async () => Promise.resolve());
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });

    expect(renewSessionPresence).toHaveBeenCalledTimes(1);
    expect(acquireSessionPresence).toHaveBeenCalledTimes(2);
  });

  it("Should release a lease that arrives after the watcher unmounts", async () => {
    let resolveAcquire: ((leaseId: string) => void) | undefined;
    vi.mocked(acquireSessionPresence).mockReturnValue(
      new Promise(resolve => {
        resolveAcquire = resolve;
      })
    );
    const { unmount } = renderHook(() => useSessionPresence("ws-alpha", "sess-alpha", true), {
      wrapper,
    });

    unmount();
    await act(async () => resolveAcquire?.("lease-late"));

    expect(releaseSessionPresence).toHaveBeenCalledExactlyOnceWith(
      "ws-alpha",
      "sess-alpha",
      "lease-late"
    );
  });
});

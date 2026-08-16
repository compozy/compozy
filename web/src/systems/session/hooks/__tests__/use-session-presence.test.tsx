// Suite: operator session presence lifecycle
// Invariant: one mounted session window owns at most one acquire/renew request
// at a time, so a slow request cannot leak a second presence lease.
// Owning layer: session presence hook. No prior suite owned this lifecycle.
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/session-presence-api", () => ({
  acquireSessionPresence: vi.fn(),
  releaseSessionPresence: vi.fn(() => Promise.resolve()),
  renewSessionPresence: vi.fn(() => Promise.resolve(true)),
}));

import {
  acquireSessionPresence,
  releaseSessionPresence,
  renewSessionPresence,
} from "../../adapters/session-presence-api";
import { useSessionPresence } from "../use-session-presence";

describe("useSessionPresence", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("Should not overlap lease acquisition while the first request is pending", async () => {
    vi.mocked(acquireSessionPresence).mockReturnValue(new Promise(() => undefined));

    renderHook(() => useSessionPresence("ws-alpha", "sess-alpha", true));

    expect(acquireSessionPresence).toHaveBeenCalledTimes(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15_000);
    });

    expect(acquireSessionPresence).toHaveBeenCalledTimes(1);
  });

  it("Should not overlap lease renewal while the current request is pending", async () => {
    vi.mocked(acquireSessionPresence).mockResolvedValue("lease-alpha");
    vi.mocked(renewSessionPresence).mockReturnValue(new Promise(() => undefined));

    renderHook(() => useSessionPresence("ws-alpha", "sess-alpha", true));
    await act(async () => Promise.resolve());
    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000);
    });

    expect(renewSessionPresence).toHaveBeenCalledTimes(1);
  });

  it("Should release a lease that arrives after the watcher unmounts", async () => {
    let resolveAcquire: ((leaseId: string) => void) | undefined;
    vi.mocked(acquireSessionPresence).mockReturnValue(
      new Promise(resolve => {
        resolveAcquire = resolve;
      })
    );
    const { unmount } = renderHook(() => useSessionPresence("ws-alpha", "sess-alpha", true));

    unmount();
    await act(async () => resolveAcquire?.("lease-late"));

    expect(releaseSessionPresence).toHaveBeenCalledExactlyOnceWith(
      "ws-alpha",
      "sess-alpha",
      "lease-late"
    );
  });
});

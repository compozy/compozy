// Invariant: speculative work follows settled intent and cannot outlive the intended target.
// Owner: shared browser intent hook; no prior suite owned preload scheduling.
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useIntentPreload } from "../use-intent-preload";

describe("useIntentPreload", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("Should merge pointer and focus intent and defer short pass-overs", async () => {
    const preload = vi.fn();
    const { result } = renderHook(() => useIntentPreload(preload));
    act(() => {
      result.current.onPointerEnter();
      result.current.onFocus();
    });
    await act(() => vi.advanceTimersByTimeAsync(99));
    expect(preload).not.toHaveBeenCalled();
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(preload).toHaveBeenCalledTimes(1);
  });

  it.each(["onPointerLeave", "onBlur"] as const)(
    "Should cancel speculative work on %s",
    async cancel => {
      const preload = vi.fn();
      const { result } = renderHook(() => useIntentPreload(preload));
      act(() => {
        result.current.onPointerEnter();
        result.current[cancel]();
      });
      await act(() => vi.advanceTimersByTimeAsync(100));
      expect(preload).not.toHaveBeenCalled();
    }
  );

  it("Should cancel a queued target when disabled, rebound or unmounted", async () => {
    const first = vi.fn();
    const second = vi.fn();
    const { result, rerender, unmount } = renderHook(
      ({ preload, enabled }) => useIntentPreload(preload, enabled),
      { initialProps: { preload: first, enabled: true } }
    );
    act(() => result.current.onFocus());
    rerender({ preload: first, enabled: false });
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(first).not.toHaveBeenCalled();

    rerender({ preload: first, enabled: true });
    act(() => result.current.onFocus());
    rerender({ preload: second, enabled: true });
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(first).not.toHaveBeenCalled();
    expect(second).not.toHaveBeenCalled();

    act(() => result.current.onFocus());
    unmount();
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(second).not.toHaveBeenCalled();
  });

  it("Should start touch intent immediately and contain a failed speculative read", async () => {
    const preload = vi.fn().mockRejectedValue(new Error("offline"));
    const { result } = renderHook(() => useIntentPreload(preload));
    await act(async () => {
      result.current.onPointerEnter();
      result.current.onTouchStart();
    });
    expect(preload).toHaveBeenCalledTimes(1);
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(preload).toHaveBeenCalledTimes(1);
  });
});

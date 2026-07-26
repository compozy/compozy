import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useLiveElapsed } from "../use-live-elapsed";

const NOW = new Date("2026-07-21T12:00:05.000Z");
const STARTED_AT = "2026-07-21T12:00:00.000Z";

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("useLiveElapsed", () => {
  it("Should tick only while active and release the shared clock after deactivation and unmount", () => {
    const setInterval = vi.spyOn(window, "setInterval");
    const clearInterval = vi.spyOn(window, "clearInterval");
    const { result, rerender, unmount } = renderHook(
      ({ active }: { active: boolean }) => useLiveElapsed(STARTED_AT, active),
      { initialProps: { active: false } }
    );

    expect(setInterval).not.toHaveBeenCalled();

    rerender({ active: true });
    expect(result.current).toBe("5s");
    expect(setInterval).toHaveBeenCalledTimes(1);

    act(() => vi.advanceTimersByTime(1_000));
    expect(result.current).toBe("6s");

    rerender({ active: false });
    expect(clearInterval).toHaveBeenCalledTimes(1);
    act(() => vi.advanceTimersByTime(1_000));
    expect(result.current).toBe("6s");

    rerender({ active: true });
    expect(result.current).toBe("7s");
    expect(setInterval).toHaveBeenCalledTimes(2);

    unmount();
    expect(clearInterval).toHaveBeenCalledTimes(2);
  });
});

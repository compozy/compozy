// @vitest-environment jsdom

import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { THREAD_OVERLAY_BREAKPOINT_PX, useThreadViewMode } from "../use-thread-view-mode";

function setMatchMedia(matches: boolean) {
  const listeners: Array<() => void> = [];
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: (query: string) => ({
      matches,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: (_event: string, listener: () => void) => {
        listeners.push(listener);
      },
      removeEventListener: (_event: string, listener: () => void) => {
        const index = listeners.indexOf(listener);
        if (index >= 0) {
          listeners.splice(index, 1);
        }
      },
      dispatchEvent: () => false,
    }),
  });
}

describe("useThreadViewMode", () => {
  it("Should return overlay when viewport is at or above the breakpoint", () => {
    setMatchMedia(true);
    const { result } = renderHook(() => useThreadViewMode());
    expect(result.current).toBe("overlay");
  });

  it("Should return fullpage when viewport is below the breakpoint", () => {
    setMatchMedia(false);
    const { result } = renderHook(() => useThreadViewMode());
    expect(result.current).toBe("fullpage");
  });

  it("Should expose the canonical 1024px breakpoint", () => {
    expect(THREAD_OVERLAY_BREAKPOINT_PX).toBe(1024);
  });
});

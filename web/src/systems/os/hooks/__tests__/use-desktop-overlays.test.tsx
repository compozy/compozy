// Suite: shell overlay ownership
// Invariant: one shell overlay is open at a time, a live palette intent IS the
// palette being open, and the palette's nested-view path is cleared on every
// open and every dismissal so the palette always reopens at its root.
// Boundary IN: the overlay hook and the window-manager interaction store.
// Boundary OUT: rendered palette contents and view registration.
import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { windowManagerStore } from "../../stores/window-manager-store";
import { useDesktopOverlays } from "../use-desktop-overlays";

function viewStack() {
  return windowManagerStore.getSnapshot().context.paletteViewStack;
}

afterEach(() => {
  windowManagerStore.trigger.paletteIntentCleared();
  windowManagerStore.trigger.paletteViewStackSet({ stack: [] });
  windowManagerStore.trigger.overlayClosed();
});

describe("useDesktopOverlays", () => {
  it("Should reopen the palette at its root after any dismissal [UT-059]", () => {
    const { result } = renderHook(() => useDesktopOverlays());

    act(() => {
      result.current.setOverlayOpen("palette", true);
      windowManagerStore.trigger.paletteViewStackSet({ stack: [{ viewId: "sessions" }] });
    });
    expect(result.current.activeOverlay).toBe("palette");
    expect(viewStack()).toEqual([{ viewId: "sessions" }]);

    act(() => {
      result.current.setOverlayOpen("palette", false);
    });
    expect(result.current.activeOverlay).toBeNull();
    expect(viewStack()).toEqual([]);

    act(() => {
      result.current.setOverlayOpen("palette", true);
    });
    expect(viewStack()).toEqual([]);
  });

  it("Should drop the view path when another overlay takes the surface", () => {
    const { result } = renderHook(() => useDesktopOverlays());

    act(() => {
      result.current.setOverlayOpen("palette", true);
      windowManagerStore.trigger.paletteViewStackSet({ stack: [{ viewId: "sessions" }] });
    });
    act(() => {
      result.current.setOverlayOpen("bell", true);
    });

    expect(result.current.activeOverlay).toBe("bell");
    expect(viewStack()).toEqual([]);
  });

  it("Should preserve the destination overlay when palette dismissal arrives last", () => {
    const { result } = renderHook(() => useDesktopOverlays());

    act(() => {
      result.current.setOverlayOpen("palette", true);
    });
    act(() => {
      result.current.setOverlayOpen("sessions", true);
      result.current.setOverlayOpen("palette", false);
    });

    expect(result.current.activeOverlay).toBe("sessions");
  });

  it("Should treat a live destination intent as the palette being open", () => {
    const { result } = renderHook(() => useDesktopOverlays());

    act(() => {
      result.current.setOverlayOpen("bell", true);
      windowManagerStore.trigger.paletteIntentRequested({
        intent: { kind: "destination", windowId: "window:new-tab" },
      });
    });
    expect(result.current.activeOverlay).toBe("palette");

    act(() => {
      result.current.setOverlayOpen("palette", false);
    });
    expect(result.current.activeOverlay).toBeNull();
    expect(windowManagerStore.getSnapshot().context.paletteIntent).toBeNull();
  });
});

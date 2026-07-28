// Suite: settings repeat-width track interaction state
// Invariant: a stop keeps its React identity even when a drag temporarily
// reaches a sibling's ratio, so focus and pointer capture stay with the stop
// the person is moving.
// Boundary IN: persisted repeat ratios and local stop-id reconciliation.
// Boundary OUT: track geometry, pointer events, and daemon persistence.
import { act, renderHook } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import type { WindowManagerConfig } from "@/systems/os";

import { useWindowManagerRatioTrack } from "../use-window-manager-ratio-track";

const INITIAL_CONFIG: WindowManagerConfig = {
  bindings: { bottomCenter: "none", topCenter: "zoom" },
  desktopTransition: "slide",
  dragAwayPolicy: "window",
  focusFollowsPointer: false,
  focusPolicy: "click_directional",
  focusWrap: true,
  gaps: { bottom: 8, inner: 8, left: 8, right: 8, top: 8 },
  groupMoveModifier: "alt",
  historyLimit: 100,
  newWindowPolicy: "floating",
  raiseOnFocus: true,
  shortcuts: {},
  smallViewportPolicy: "stack",
  snap: {
    cornerReach: 150,
    edgeBand: 32,
    exitSlack: 16,
    repeatRatios: [0.33, 0.5, 0.67],
  },
  swapModifier: "shift",
};

function useRatioTrackHarness() {
  const [draft, setDraft] = useState(INITIAL_CONFIG);
  return useWindowManagerRatioTrack(draft, setDraft);
}

describe("useWindowManagerRatioTrack", () => {
  it("Should retain a dragged stop identity when its ratio matches a sibling", () => {
    const { result } = renderHook(() => useRatioTrackHarness());
    const firstId = result.current.stopIdAt(0);
    const secondId = result.current.stopIdAt(1);

    act(() => {
      result.current.setRatioAt(1, 0.33);
    });

    expect(result.current.ratios).toEqual([0.33, 0.33, 0.67]);
    expect(result.current.stopIdAt(0)).toBe(firstId);
    expect(result.current.stopIdAt(1)).toBe(secondId);
  });
});

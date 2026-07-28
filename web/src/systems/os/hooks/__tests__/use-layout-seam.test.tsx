// Suite: layout seam keyboard resizing
// Invariant: keyboard resizing obeys the same projection travel and adjacent-weight floors as pointer dragging.
// Boundary IN: one projected seam and keyboard input.
// Boundary OUT: command transport and daemon layout reduction.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ProjectedSeam } from "../../lib/window-manager-types";
import { useLayoutSeam } from "../use-layout-seam";

const SEAM: ProjectedSeam = {
  id: "split:root:0",
  splitId: "split:root",
  boundaryIndex: 0,
  orientation: "vertical",
  rect: { x: 496, y: 0, w: 8, h: 700 },
  value: 500,
  minValue: 500,
  maxValue: 700,
  axisSpan: 992,
  leadingWeight: 0.5,
  trailingWeight: 0.5,
  leadingWindowIds: ["window:left"],
  trailingWindowIds: ["window:right"],
};

function SeamHarness({
  seam,
  onResize,
}: {
  seam: ProjectedSeam;
  onResize: (splitId: string, boundaryIndex: number, delta: number) => void;
}) {
  const model = useLayoutSeam(seam, onResize, vi.fn(), vi.fn());
  return <div data-testid="seam" onKeyDown={model.handleKeyDown} />;
}

describe("useLayoutSeam", () => {
  it("Should skip a keyboard resize that exceeds the projected travel", () => {
    const onResize = vi.fn();
    render(<SeamHarness seam={SEAM} onResize={onResize} />);
    const event = new KeyboardEvent("keydown", {
      bubbles: true,
      cancelable: true,
      key: "ArrowLeft",
    });

    fireEvent(screen.getByTestId("seam"), event);

    expect(event.defaultPrevented).toBe(true);
    expect(onResize).not.toHaveBeenCalled();
  });

  it("Should convert an accepted keyboard step through seam weight geometry", () => {
    const onResize = vi.fn();
    render(<SeamHarness seam={SEAM} onResize={onResize} />);

    fireEvent.keyDown(screen.getByTestId("seam"), { key: "ArrowRight" });

    expect(onResize).toHaveBeenCalledWith("split:root", 0, 0.02);
  });
});

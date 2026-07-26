// Suite: settings seam drag geometry
// Invariant: a divider drag resolves every pointer position against the seam
// captured at pointer-down, so re-projection cannot make a long drag undershoot
// the pointer after intermediate preview updates.
// Boundary IN: projected seam geometry, pointer travel, and draft desktop state.
// Boundary OUT: canvas markup, daemon validation, and live-shell resize commands.
import { fireEvent, render, screen } from "@testing-library/react";
import { useRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";

import type { ProjectedSeam } from "@/systems/os";

import { useLayoutDraftSeam } from "../use-layout-draft-seam";
import type { WindowManagerLayoutDesktop } from "../../lib/window-manager-layout-types";

const INITIAL_DESKTOP: WindowManagerLayoutDesktop = {
  id: "desktop-one",
  name: "Build",
  order: 0,
  purpose: "standard",
  focusOwner: null,
  floating: [],
  groups: [
    {
      id: "group-main",
      frame: { x: 0, y: 0, w: 1, h: 1 },
      root: {
        id: "split-main",
        kind: "split",
        axis: "horizontal",
        weights: [0.5, 0.5],
        children: [
          { id: "leaf-left", kind: "leaf", windowId: "app:left" },
          { id: "leaf-right", kind: "leaf", windowId: "app:right" },
        ],
      },
    },
  ],
};

function currentLeadingWeight(desktop: WindowManagerLayoutDesktop): number {
  const root = desktop.groups[0]?.root;
  if (!root || root.kind !== "split") throw new Error("Expected a split root.");
  return root.weights[0] ?? 0.5;
}

function seamFor(desktop: WindowManagerLayoutDesktop): ProjectedSeam {
  const leadingWeight = currentLeadingWeight(desktop);
  return {
    id: "split-main:0",
    splitId: "split-main",
    boundaryIndex: 0,
    orientation: "vertical",
    rect: { x: leadingWeight * 1440, y: 0, w: 1, h: 900 },
    value: leadingWeight * 1440,
    minValue: 0,
    maxValue: 1440,
    axisSpan: 1440,
    leadingWeight,
    trailingWeight: 1 - leadingWeight,
    leadingWindowIds: ["app:left"],
    trailingWindowIds: ["app:right"],
  };
}

function Harness({ onChange }: { onChange: (desktop: WindowManagerLayoutDesktop) => void }) {
  const [desktop, setDesktop] = useState(INITIAL_DESKTOP);
  const canvasRef = useRef<HTMLDivElement>(null);
  const seam = seamFor(desktop);
  const model = useLayoutDraftSeam(seam, desktop, canvasRef, next => {
    setDesktop(next);
    onChange(next);
  });

  return (
    <div data-testid="canvas" ref={canvasRef}>
      <button data-testid="seam" type="button" onPointerDown={model.onPointerDown} />
    </div>
  );
}

describe("useLayoutDraftSeam", () => {
  it("Should keep a long drag anchored to the pointer-down seam after re-projection", () => {
    let latest: WindowManagerLayoutDesktop | undefined;
    const onChange = vi.fn((desktop: WindowManagerLayoutDesktop) => {
      latest = desktop;
    });
    render(<Harness onChange={onChange} />);
    const canvas = screen.getByTestId("canvas");
    const handle = screen.getByTestId("seam");
    Object.defineProperty(canvas, "getBoundingClientRect", {
      value: () => ({ bottom: 900, height: 900, left: 0, right: 1440, top: 0, width: 1440 }),
    });
    Object.defineProperty(handle, "setPointerCapture", { value: vi.fn() });

    fireEvent.pointerDown(handle, { button: 0, clientX: 720, pointerId: 1 });
    fireEvent.pointerMove(handle, { clientX: 1152, pointerId: 1 });
    fireEvent.pointerMove(handle, { clientX: 1411, pointerId: 1 });

    if (!latest) throw new Error("Expected the seam drag to produce a draft desktop.");
    expect(currentLeadingWeight(latest)).toBeCloseTo(0.98, 2);
  });
});

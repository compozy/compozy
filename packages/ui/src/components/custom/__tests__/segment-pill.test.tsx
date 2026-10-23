import { createRef, type Ref } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SEGMENT_PILL_TARGET } from "../hooks/use-segment-pill";
import { SegmentPill } from "../segment-pill";
import { UIProvider } from "../ui-provider";

// jsdom lays nothing out: each segment reports the box its data attributes name.
function stubBox(prop: "offsetLeft" | "offsetWidth" | "offsetHeight", attr: string) {
  return vi.spyOn(HTMLElement.prototype, prop, "get").mockImplementation(function (
    this: HTMLElement
  ) {
    return Number(this.dataset[attr] ?? 0);
  });
}

function Segments({ index, pillRef }: { index: number | null; pillRef?: Ref<HTMLSpanElement> }) {
  return (
    <div className="relative inline-flex">
      <button type="button" {...{ [SEGMENT_PILL_TARGET]: "" }} data-l="4" data-w="40" data-h="20">
        Children
      </button>
      <button type="button" {...{ [SEGMENT_PILL_TARGET]: "" }} data-l="48" data-w="56" data-h="20">
        Subagents
      </button>
      <SegmentPill ref={pillRef} index={index} data-testid="pill" />
    </div>
  );
}

// Invariant: the plate rests on the selected segment's box (reduced motion jumps there)
// and hides with no selection. Owner: @compozy/ui custom primitives suite.
describe("SegmentPill", () => {
  beforeEach(() => {
    stubBox("offsetLeft", "l");
    stubBox("offsetWidth", "w");
    stubBox("offsetHeight", "h");
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  const parts = () => [...screen.getByTestId("pill").firstElementChild!.children] as HTMLElement[];
  const edges = () => [parts()[0]!.style.transform, parts()[2]!.style.transform];

  it("Should rest on the selected segment and follow a new selection", async () => {
    const { rerender } = render(
      <UIProvider reducedMotion="always">
        <Segments index={0} />
      </UIProvider>
    );
    expect(screen.getByTestId("pill")).toHaveAttribute("data-state", "shown");
    // Motion writes styles on its next frame.
    await waitFor(() => expect(edges()).toEqual(["translateX(4px)", "translateX(34px)"]));

    rerender(
      <UIProvider reducedMotion="always">
        <Segments index={1} />
      </UIProvider>
    );
    // Left cap at the segment's left edge; right cap one radius inside its right edge.
    await waitFor(() => expect(edges()).toEqual(["translateX(48px)", "translateX(94px)"]));
    expect(parts()[1]!.style.transform).toContain("scaleX(37)");
  });

  it("Should measure the selected target while forwarding the caller ref", async () => {
    const ref = createRef<HTMLSpanElement>();
    render(
      <UIProvider reducedMotion="always">
        <Segments index={1} pillRef={ref} />
      </UIProvider>
    );
    expect(ref.current).toBe(screen.getByTestId("pill"));
    await waitFor(() => expect(edges()).toEqual(["translateX(48px)", "translateX(94px)"]));
  });

  it("Should settle on the current target bounds after a resize during travel", async () => {
    const { rerender } = render(
      <UIProvider reducedMotion="never">
        <Segments index={0} />
      </UIProvider>
    );
    await waitFor(() => expect(edges()).toEqual(["translateX(4px)", "translateX(34px)"]));
    rerender(
      <UIProvider reducedMotion="never">
        <Segments index={1} />
      </UIProvider>
    );
    // Layout is the I/O boundary: the target narrows while the real motion runs.
    screen.getByRole("button", { name: "Subagents" }).dataset.w = "40";
    await waitFor(() => expect(edges()).toEqual(["translateX(48px)", "translateX(78px)"]));
  });

  it("Should hide without a selection", () => {
    render(
      <UIProvider reducedMotion="always">
        <Segments index={null} />
      </UIProvider>
    );
    expect(screen.getByTestId("pill")).toHaveAttribute("data-state", "hidden");
    expect(screen.getByTestId("pill")).toHaveAttribute("aria-hidden", "true");
  });
});

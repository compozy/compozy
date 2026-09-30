import { render, screen } from "@testing-library/react";
import { MotionConfig } from "motion/react";
import { describe, expect, it } from "vitest";

import { LoopStatusMark } from "@/systems/loops";

describe("LoopStatusMark", () => {
  it("Should render the human label for a known run status", () => {
    render(<LoopStatusMark status="needs-approval" />);
    expect(screen.getByText("Needs approval")).toBeInTheDocument();
  });

  it("Should fall back to the raw value for an unknown status and Unknown for a blank one", () => {
    const { rerender } = render(<LoopStatusMark status="mystery" />);
    expect(screen.getByText("mystery")).toBeInTheDocument();

    rerender(<LoopStatusMark status={null} />);
    expect(screen.getByText("Unknown")).toBeInTheDocument();
  });

  it("Should spin the glyph only for the live running/watching states", () => {
    const glyph = (container: HTMLElement) => container.querySelector('[data-slot="state-glyph"]');
    const { container, rerender } = render(<LoopStatusMark status="running" />);
    expect(glyph(container)).toHaveAttribute("data-spinning", "true");
    rerender(<LoopStatusMark status="watching" />);
    expect(glyph(container)).toHaveAttribute("data-spinning", "true");
    rerender(<LoopStatusMark status="done" />);
    expect(glyph(container)).not.toHaveAttribute("data-spinning");
    rerender(<LoopStatusMark status="needs-approval" />);
    expect(glyph(container)).toHaveAttribute("data-state", "attention");
    expect(glyph(container)).not.toHaveAttribute("data-spinning");
  });

  it("Should hold the glyph still under prefers-reduced-motion", () => {
    const { container } = render(
      <MotionConfig reducedMotion="always">
        <LoopStatusMark status="running" />
      </MotionConfig>
    );
    expect(container.querySelector('[data-slot="state-glyph"]')).not.toHaveAttribute(
      "data-spinning"
    );
  });
});

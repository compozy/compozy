import { render, screen } from "@testing-library/react";
import { MotionConfig } from "motion/react";
import { describe, expect, it } from "vitest";

import { LoopStatusPill } from "@/systems/loops";

describe("LoopStatusPill", () => {
  it("Should render the human label for a known run status", () => {
    render(<LoopStatusPill status="needs-approval" />);
    expect(screen.getByText("Needs Approval")).toBeInTheDocument();
  });

  it("Should fall back to the raw value for an unknown status and Unknown for a blank one", () => {
    const { rerender } = render(<LoopStatusPill status="mystery" />);
    expect(screen.getByText("mystery")).toBeInTheDocument();

    rerender(<LoopStatusPill status={null} />);
    expect(screen.getByText("Unknown")).toBeInTheDocument();
  });

  it("Should mark the dot as pulsing only for the live running/watching states", () => {
    const { container, rerender } = render(<LoopStatusPill status="running" />);
    expect(container.querySelector('[data-slot="pill-dot"]')).toHaveAttribute("data-pulse", "true");
    rerender(<LoopStatusPill status="watching" />);
    expect(container.querySelector('[data-slot="pill-dot"]')).toHaveAttribute("data-pulse", "true");
    rerender(<LoopStatusPill status="done" />);
    expect(container.querySelector('[data-slot="pill-dot"]')).not.toHaveAttribute("data-pulse");
    rerender(<LoopStatusPill status="needs-approval" />);
    expect(container.querySelector('[data-slot="pill-dot"]')).not.toHaveAttribute("data-pulse");
  });

  it("Should gate the pulse under prefers-reduced-motion", () => {
    const { container } = render(
      <MotionConfig reducedMotion="always">
        <LoopStatusPill status="running" />
      </MotionConfig>
    );
    expect(container.querySelector('[data-slot="pill-dot"]')).not.toHaveAttribute("data-pulse");
  });
});

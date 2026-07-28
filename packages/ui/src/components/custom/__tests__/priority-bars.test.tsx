import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PriorityBars } from "../priority-bars";

describe("PriorityBars", () => {
  it("Should render exactly three bars (count never varies with level)", () => {
    for (const level of ["low", "medium", "high", "urgent"] as const) {
      const { container } = render(<PriorityBars level={level} />);
      const bars = container.querySelectorAll('[data-slot="priority-bars-bar"]');
      expect(bars).toHaveLength(3);
    }
  });

  it("Should expose role=img and aria-label='{level} priority' by default", () => {
    const { container } = render(<PriorityBars level="high" />);
    const root = container.querySelector('[data-slot="priority-bars"]');
    expect(root).toHaveAttribute("role", "img");
    expect(root).toHaveAttribute("aria-label", "high priority");
    expect(root).toHaveAttribute("data-level", "high");
  });

  it("Should honor a caller-provided aria-label override", () => {
    const { container } = render(<PriorityBars ariaLabel="Critical" level="urgent" />);
    const root = container.querySelector('[data-slot="priority-bars"]');
    expect(root).toHaveAttribute("aria-label", "Critical");
  });
});

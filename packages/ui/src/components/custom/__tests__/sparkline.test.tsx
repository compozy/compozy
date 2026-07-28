import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Sparkline } from "../sparkline";

describe("Sparkline", () => {
  it("Should render one bar per bucket with deterministic heights", () => {
    const { container } = render(<Sparkline values={[1, 2, 4]} max={4} ariaLabel="bars" />);
    const bars = Array.from(container.querySelectorAll<HTMLElement>('[data-slot="sparkline-bar"]'));
    expect(bars).toHaveLength(3);
    expect(bars[0].style.height).toBe("25%");
    expect(bars[1].style.height).toBe("50%");
    expect(bars[2].style.height).toBe("100%");
  });

  it("Should clamp to a minimum of one bar when no values are supplied", () => {
    const { container } = render(<Sparkline values={[]} ariaLabel="empty" />);
    expect(container.querySelectorAll('[data-slot="sparkline-bar"]')).toHaveLength(1);
  });
});

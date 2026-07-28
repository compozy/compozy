import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Metric } from "../metric";
import { MetricGrid } from "../metric-grid";

describe("MetricGrid", () => {
  it("Should render the default four-column responsive grid template", () => {
    const { container } = render(
      <MetricGrid>
        <Metric label="Sessions" value="12" />
      </MetricGrid>
    );

    const grid = container.querySelector<HTMLElement>('[data-slot="metric-grid"]');
    expect(grid).not.toBeNull();
    expect(grid).toHaveAttribute("data-columns", "4");
    expect(grid?.className).toContain("grid");
    expect(grid?.className).toContain("gap-3");
    expect(grid?.className).toContain("sm:grid-cols-2");
    expect(grid?.className).toContain("xl:grid-cols-4");
  });

  it("Should render the configured three-column responsive template", () => {
    const { container } = render(
      <MetricGrid columns={3}>
        <Metric label="Runs" value="08" />
      </MetricGrid>
    );

    const grid = container.querySelector<HTMLElement>('[data-slot="metric-grid"]');
    expect(grid).toHaveAttribute("data-columns", "3");
    expect(grid?.className).toContain("sm:grid-cols-2");
    expect(grid?.className).toContain("xl:grid-cols-3");
    expect(grid?.className).not.toContain("xl:grid-cols-4");
  });

  it("Should preserve caller classes without dropping the grid template", () => {
    const { container } = render(<MetricGrid columns={2} className="mt-2" />);
    const grid = container.querySelector<HTMLElement>('[data-slot="metric-grid"]');
    expect(grid?.className).toContain("mt-2");
    expect(grid?.className).toContain("sm:grid-cols-2");
  });
});

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StackedProgress } from "../stacked-progress";

describe("StackedProgress", () => {
  it("Should render one segment per non-zero value", () => {
    const { container } = render(
      <StackedProgress
        ariaLabel="Run breakdown"
        segments={[
          { value: 3, tone: "success" },
          { value: 0, tone: "warning" },
          { value: 1, tone: "danger" },
        ]}
      />
    );
    const segments = container.querySelectorAll('[data-slot="stacked-progress-segment"]');
    expect(segments).toHaveLength(2);
  });
});

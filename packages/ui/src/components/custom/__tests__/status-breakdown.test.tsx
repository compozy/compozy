import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusBreakdown } from "../status-breakdown";

describe("StatusBreakdown", () => {
  it("Should render one row per item with a sized bar", () => {
    const { container } = render(
      <StatusBreakdown
        items={[
          { label: "Running", value: 5, tone: "success" },
          { label: "Failed", value: 1, tone: "danger" },
        ]}
      />
    );
    const rows = container.querySelectorAll('[data-slot="status-breakdown-row"]');
    expect(rows).toHaveLength(2);
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
  });
});

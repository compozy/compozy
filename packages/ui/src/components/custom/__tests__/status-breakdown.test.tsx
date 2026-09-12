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

// Invariant: formatted counts change presentation while the magnitude remains numeric; owner: StatusBreakdown.
it("Should format a count without changing its bar magnitude", () => {
  const { container } = render(
    <StatusBreakdown
      total={100_000}
      items={[{ label: "Context", value: 12_400, formattedValue: "≈ 12.4K" }]}
    />
  );
  expect(screen.getByText("≈ 12.4K")).toBeInTheDocument();
  expect(container.querySelector('[data-slot="status-breakdown-bar"]')).toHaveStyle({
    width: "12%",
  });
});

// Invariant: label/value-only observations do not imply a measured magnitude; owner: StatusBreakdown.
it("Should render a label and formatted value without a magnitude bar when requested", () => {
  const { container } = render(
    <StatusBreakdown
      items={[{ label: "Context", value: 12_400, formattedValue: "≈ 12.4K", showBar: false }]}
    />
  );
  expect(screen.getByText("Context")).toBeInTheDocument();
  expect(screen.getByText("≈ 12.4K")).toBeInTheDocument();
  expect(container.querySelector('[data-slot="status-breakdown-bar"]')).toBeNull();
});

// Invariant: a caveat and a raw figure ride with their row without changing its magnitude; owner: StatusBreakdown.
it("Should keep a detail beside the value and a note under the row keyed by id", () => {
  const { container } = render(
    <StatusBreakdown
      total={256_000}
      items={[
        {
          id: "compozy",
          label: <b>Compozy</b>,
          value: 225_000,
          formattedValue: "225K",
          detail: "of ≈ 231K",
          note: "estimate exceeds reported",
          showBar: false,
        },
        { id: "free", label: <b>Free</b>, value: 31_000, formattedValue: "31K", showBar: false },
      ]}
    />
  );
  const rows = container.querySelectorAll('[data-slot="status-breakdown-row"]');
  expect(rows).toHaveLength(2);
  expect(rows[0]?.querySelector('[data-slot="status-breakdown-detail"]')).toHaveTextContent(
    "of ≈ 231K"
  );
  expect(rows[0]?.querySelector('[data-slot="status-breakdown-note"]')).toHaveTextContent(
    "estimate exceeds reported"
  );
  expect(rows[1]?.querySelector('[data-slot="status-breakdown-note"]')).toBeNull();
});

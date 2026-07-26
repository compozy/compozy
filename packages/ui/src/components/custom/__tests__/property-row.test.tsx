import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PropertyRow } from "../property-row";

describe("PropertyRow", () => {
  it("Should render label and value with the mono voice flagged", () => {
    render(
      <PropertyRow data-testid="row" label="Task id" mono>
        task_001
      </PropertyRow>
    );
    const row = screen.getByTestId("row");
    expect(row).toHaveTextContent("Task id");
    expect(row).toHaveTextContent("task_001");
    expect(row.querySelector('[data-slot="property-row-value"]')).toHaveAttribute(
      "data-mono",
      "true"
    );
  });

  it("Should render the editor slot instead of the static value", () => {
    render(
      <PropertyRow data-testid="row" editor={<button type="button">High</button>} label="Priority">
        ignored
      </PropertyRow>
    );
    const row = screen.getByTestId("row");
    expect(row).not.toHaveTextContent("ignored");
    expect(screen.getByRole("button", { name: "High" })).toBeInTheDocument();
  });

  it("Should expose the complete primitive value when the visible row truncates", () => {
    const value = "workspace_01J7GJ67XQ6S0C8R1DVKPX9F5N";
    render(<PropertyRow label="Workspace">{value}</PropertyRow>);

    expect(screen.getByTitle(value)).toHaveTextContent(value);
  });
});

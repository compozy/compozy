import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { BlockLoading } from "../block-loading";

describe("BlockLoading", () => {
  it("Should render a centered panel spinner with the default md height", () => {
    const { container } = render(<BlockLoading data-testid="loading-panel" />);

    const panel = screen.getByTestId("loading-panel");
    expect(panel).toHaveAttribute("data-slot", "block-loading");
    expect(panel).toHaveAttribute("data-size", "md");
    expect(panel).toHaveAttribute("data-surface", "panel");
    expect(container.querySelector('[role="status"]')).toBeInTheDocument();
  });

  it("Should render the bare small variant without panel chrome", () => {
    render(
      <BlockLoading data-testid="loading-bare" size="sm" surface="bare" label="Loading rows" />
    );

    const panel = screen.getByTestId("loading-bare");
    expect(panel).toHaveAttribute("data-size", "sm");
    expect(panel).toHaveAttribute("data-surface", "bare");
    expect(screen.getByLabelText("Loading rows")).toBeInTheDocument();
  });
});

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RequiredMark } from "../required-mark";

describe("RequiredMark", () => {
  it("Should spell the requirement out instead of relying on a glyph", () => {
    const { container } = render(<RequiredMark />);

    expect(screen.getByText("required")).toBeInTheDocument();
    expect(container.textContent).not.toContain("*");
    expect(container.querySelector('[data-slot="required-mark"]')).not.toBeNull();
  });

  it("Should accept a contract-specific affix and extra span props", () => {
    render(<RequiredMark data-testid="slot-affix">required by provider</RequiredMark>);

    expect(screen.getByTestId("slot-affix")).toHaveTextContent("required by provider");
  });
});

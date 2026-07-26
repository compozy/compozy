import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Separator } from "../separator";

describe("Separator", () => {
  it("Should render a plain horizontal separator by default", () => {
    const { container } = render(<Separator />);
    const separator = container.querySelector('[data-slot="separator"]');
    expect(separator).toHaveAttribute("data-orientation", "horizontal");
    expect(separator).not.toHaveTextContent("Replies");
  });

  it("Should render a centered label between two decorative rules", () => {
    const { container } = render(<Separator label="Replies" />);
    const separator = screen.getByRole("separator");
    const label = container.querySelector('[data-slot="separator-label"]');
    const lines = container.querySelectorAll('[aria-hidden="true"]');

    expect(separator).toHaveAttribute("data-slot", "separator");
    expect(separator).toHaveAttribute("data-orientation", "horizontal");
    expect(label).toHaveTextContent("Replies");
    expect(lines).toHaveLength(2);
  });

  it("Should expose the accent tone for labelled separators via data-tone", () => {
    render(<Separator label="New" tone="accent" />);
    const separator = screen.getByRole("separator");

    expect(separator).toHaveAttribute("data-tone", "accent");
  });
});

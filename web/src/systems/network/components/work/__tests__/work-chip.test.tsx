// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { WorkChip } from "../work-chip";

describe("WorkChip silence rules (`_design.md` §6.6)", () => {
  it.each(["submitted", "completed"] as const)("Should render nothing for %s", state => {
    const { container } = render(<WorkChip state={state} />);
    expect(container.querySelector('[data-testid="network-work-chip"]')).toBeNull();
  });

  it.each(["working", "needs_input", "failed"] as const)(
    "Should render a tinted chip for %s",
    state => {
      render(<WorkChip state={state} />);
      const chip = screen.getByTestId("network-work-chip");
      expect(chip).toHaveAttribute("data-state", state);
      expect(chip).toHaveAttribute("data-slot", "pill");
      const cls = chip.className;
      if (state === "failed") {
        expect(chip).toHaveAttribute("data-tone", "danger");
        expect(cls).toContain("text-danger");
      } else {
        expect(chip).toHaveAttribute("data-tone", "warning");
        expect(cls).toContain("text-warning");
      }
    }
  );

  it("Should render `canceled` in tertiary text only (no color tint)", () => {
    render(<WorkChip state="canceled" />);
    const chip = screen.getByTestId("network-work-chip");
    expect(chip).toHaveAttribute("data-state", "canceled");
    expect(chip.className).toContain("text-subtle");
    expect(chip.className).not.toContain("bg-warning-tint");
    expect(chip.className).not.toContain("bg-danger-tint");
  });

  it("Should render `working` chip text without an elapsed suffix when no startedAt is provided", () => {
    render(<WorkChip state="working" />);
    expect(screen.getByTestId("network-work-chip")).toHaveTextContent("working");
    expect(screen.getByTestId("network-work-chip").textContent).not.toMatch(/\d+s/);
  });

  it("Should render `needs input` label using the human-readable spelling", () => {
    render(<WorkChip state="needs_input" />);
    expect(screen.getByTestId("network-work-chip")).toHaveTextContent("needs input");
  });
});

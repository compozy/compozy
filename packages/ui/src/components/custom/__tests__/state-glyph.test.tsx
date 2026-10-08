import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { UIProvider } from "../ui-provider";
import { StateGlyph, type StateGlyphState } from "../state-glyph";

const STATES: StateGlyphState[] = [
  "running",
  "queued",
  "done",
  "attention",
  "failed",
  "stopped",
  "idle",
];

describe("StateGlyph", () => {
  it.each(STATES)("Should expose the %s state and stay decorative without a label", state => {
    const { container } = render(<StateGlyph state={state} />);
    const glyph = container.querySelector('[data-slot="state-glyph"]');
    expect(glyph).toHaveAttribute("data-state", state);
    expect(glyph).toHaveAttribute("aria-hidden", "true");
    expect(glyph).not.toHaveAttribute("role");
  });

  it("Should become a named image when a label is given", () => {
    render(<StateGlyph state="attention" label="Needs you" />);
    const glyph = screen.getByRole("img", { name: "Needs you" });
    expect(glyph).not.toHaveAttribute("aria-hidden");
  });

  it("Should map attention to the accent and never to the warning amber", () => {
    const { container } = render(<StateGlyph state="attention" />);
    const glyph = container.querySelector('[data-slot="state-glyph"]');
    expect(glyph).toHaveClass("text-accent");
    expect(glyph?.getAttribute("class")).not.toContain("warning");
  });

  it.each([
    ["failed", "text-danger"],
    ["stopped", "text-subtle"],
  ] as const)("Should paint the %s state with %s", (state, toneClass) => {
    const { container } = render(<StateGlyph state={state} />);
    expect(container.querySelector('[data-slot="state-glyph"]')).toHaveClass(toneClass);
  });

  it("Should spin only while running", () => {
    const { container, rerender } = render(<StateGlyph state="running" />);
    const glyph = () => container.querySelector('[data-slot="state-glyph"]');
    expect(glyph()).toHaveAttribute("data-spinning", "true");
    expect(glyph()).toHaveClass("animate-spin");
    rerender(<StateGlyph state="queued" />);
    expect(glyph()).not.toHaveAttribute("data-spinning");
    expect(glyph()).not.toHaveClass("animate-spin");
  });

  it("Should hold the running ring still under reduced motion", () => {
    const { container } = render(
      <UIProvider reducedMotion="always">
        <StateGlyph state="running" />
      </UIProvider>
    );
    const glyph = container.querySelector('[data-slot="state-glyph"]');
    expect(glyph).toHaveAttribute("data-state", "running");
    expect(glyph).not.toHaveAttribute("data-spinning");
    expect(glyph).not.toHaveClass("animate-spin");
  });

  it("Should hold the running ring still when still is set", () => {
    const { container } = render(<StateGlyph state="running" still />);
    const glyph = container.querySelector('[data-slot="state-glyph"]');
    expect(glyph).not.toHaveAttribute("data-spinning");
    expect(glyph).not.toHaveClass("animate-spin");
  });
});

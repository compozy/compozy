// Suite: sprite icon primitive
// Invariant: the rendered <use> always targets `${spriteUrl}#${name}` and stays
// decorative, so any catalog slug renders without per-icon imports.
// Boundary IN: the SpriteIcon element contract.
// Boundary OUT: sprite asset generation (lucide-static) and catalog validation (Go suites).

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SpriteIcon } from "../sprite-icon";

describe("SpriteIcon", () => {
  it("Should reference the named symbol inside the sprite", () => {
    const { container } = render(<SpriteIcon spriteUrl="/sprite.svg" name="rocket" />);
    expect(container.querySelector("use")).toHaveAttribute("href", "/sprite.svg#rocket");
  });

  it("Should stay decorative and inherit currentColor with lucide stroke geometry", () => {
    const { container } = render(
      <SpriteIcon spriteUrl="/sprite.svg" name="gem" className="size-6" strokeWidth={1.5} />
    );
    const svg = container.querySelector("svg");
    expect(svg).toHaveAttribute("aria-hidden", "true");
    expect(svg).toHaveAttribute("stroke", "currentColor");
    expect(svg).toHaveAttribute("stroke-width", "1.5");
    expect(svg).toHaveAttribute("viewBox", "0 0 24 24");
    expect(svg).toHaveClass("size-6");
  });
});

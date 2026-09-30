import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Surface } from "../surface";

describe("Surface", () => {
  it("Should render with the data-slot and default size", () => {
    const { container } = render(<Surface>body</Surface>);
    const root = container.querySelector<HTMLElement>('[data-slot="surface"]');
    expect(root).not.toBeNull();
    expect(root).toHaveAttribute("data-size", "default");
  });

  it("Should reflect the compact size via data-size", () => {
    const { container } = render(<Surface size="compact">body</Surface>);
    const root = container.querySelector<HTMLElement>('[data-slot="surface"]');
    expect(root?.getAttribute("data-size")).toBe("compact");
  });

  it("Should merge consumer className onto the surface tuple", () => {
    const { container } = render(<Surface className="flex flex-col gap-2">body</Surface>);
    const root = container.querySelector<HTMLElement>('[data-slot="surface"]');
    expect(root?.className).toContain("bg-canvas");
    expect(root?.className).toContain("flex");
  });

  it("Should lift the default card variant with the card shadow", () => {
    const { container } = render(<Surface>body</Surface>);
    const root = container.querySelector<HTMLElement>('[data-slot="surface"]');
    expect(root).toHaveAttribute("data-variant", "card");
    expect(root).toHaveClass("bg-canvas", "shadow-card");
  });

  it("Should recess the sunken variant without a border or shadow", () => {
    const { container } = render(<Surface variant="sunken">tool rows</Surface>);
    const root = container.querySelector<HTMLElement>('[data-slot="surface"]');
    expect(root).toHaveAttribute("data-variant", "sunken");
    expect(root).toHaveClass("bg-sunken", "rounded-lg");
    expect(root?.className).not.toMatch(/\bshadow-|\bborder\b/);
  });

  it("Should render a caller element with no padding in the flush size", () => {
    const { container } = render(
      <Surface render={<section aria-label="Zone" />} size="flush">
        body
      </Surface>
    );
    const root = container.querySelector<HTMLElement>('[data-slot="surface"]');
    expect(root?.tagName).toBe("SECTION");
    expect(root).toHaveAttribute("data-size", "flush");
    expect(root?.className).not.toMatch(/\b(px|py)-/);
  });
});

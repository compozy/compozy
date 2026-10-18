import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Surface } from "../surface";

describe("Surface", () => {
  it("Should merge consumer className onto the surface tuple", () => {
    const { container } = render(<Surface className="flex flex-col gap-2">body</Surface>);
    const root = container.querySelector<HTMLElement>('[data-slot="surface"]');
    expect(root?.className).toContain("bg-card");
    expect(root?.className).toContain("flex");
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

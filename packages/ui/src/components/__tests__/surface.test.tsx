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
    expect(root?.className).toContain("bg-canvas-soft");
    expect(root?.className).toContain("flex");
  });
});

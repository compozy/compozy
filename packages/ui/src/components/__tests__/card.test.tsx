import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Card } from "../card";

describe("Card", () => {
  it("Should render a card root with the stable data-slot", () => {
    const { container } = render(<Card>body</Card>);
    const root = container.querySelector<HTMLElement>('[data-slot="card"]');
    expect(root).not.toBeNull();
    expect(root).toHaveAttribute("data-size", "default");
  });

  it("Should reflect the sm size variant via data-size", () => {
    const { container } = render(<Card size="sm">body</Card>);
    const root = container.querySelector<HTMLElement>('[data-slot="card"]');
    expect(root?.getAttribute("data-size")).toBe("sm");
  });
});

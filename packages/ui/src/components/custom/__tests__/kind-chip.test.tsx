import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { KindChip } from "../kind-chip";

describe("KindChip", () => {
  it("Should render a mono pill keyed by `kind`", () => {
    render(<KindChip kind="capability" />);
    const pill = screen.getByText("capability");
    expect(pill.parentElement?.dataset.slot).toBe("kind-chip");
  });

  it("Should render a leading dot for known protocol kinds", () => {
    const { container } = render(<KindChip kind="say" />);
    expect(container.querySelector('[data-slot="pill-dot"]')).not.toBeNull();
  });

  it("Should respect an explicit dotColor override", () => {
    const { container } = render(<KindChip kind="custom-platform" dotColor="#abcdef" />);
    const dot = container.querySelector<HTMLElement>('[data-slot="pill-dot"]');
    expect(dot?.style.backgroundColor).toBe("rgb(171, 205, 239)");
  });
});

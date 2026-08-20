import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Eyebrow } from "../eyebrow";

function classes(node: HTMLElement): string[] {
  return node.className.split(/\s+/).filter(Boolean);
}

describe("Eyebrow", () => {
  // KEEP: DESIGN.md §3 eyebrow contract — the eyebrow utility itself is the product contract.
  it("Should render the single eyebrow utility (Geist 12/510/-0.005em, sentence case) by default", () => {
    render(<Eyebrow>Run state</Eyebrow>);
    const eyebrow = screen.getByText("Run state");

    expect(eyebrow).toHaveAttribute("data-slot", "eyebrow");
    expect(eyebrow).not.toHaveAttribute("data-variant");
    expect(classes(eyebrow)).toEqual(["eyebrow"]);
  });

  // KEEP: DESIGN.md §3 eyebrow contract — caps is the fixed uppercase rendition
  // (11/600/+0.06em), a single utility with no free parameters.
  it("Should render the eyebrow-caps rendition for variant='caps'", () => {
    render(<Eyebrow variant="caps">Run state</Eyebrow>);
    const eyebrow = screen.getByText("Run state");

    expect(eyebrow).toHaveAttribute("data-variant", "caps");
    expect(classes(eyebrow)).toEqual(["eyebrow-caps"]);
  });

  // KEEP: DESIGN.md §3 eyebrow contract — verifies utility is preserved when merging consumer className.
  it("Should merge a consumer className while keeping the eyebrow utility first", () => {
    render(<Eyebrow className="extra-token">Queue health</Eyebrow>);
    const eyebrow = screen.getByText("Queue health");

    expect(classes(eyebrow)).toContain("eyebrow");
    expect(classes(eyebrow)).toContain("extra-token");
  });

  it("Should forward span props (data-testid, aria, id) onto the rendered node", () => {
    render(
      <Eyebrow aria-label="meta" data-testid="eyebrow-target" id="eyebrow-id">
        Meta
      </Eyebrow>
    );
    const eyebrow = screen.getByTestId("eyebrow-target");

    expect(eyebrow.tagName).toBe("SPAN");
    expect(eyebrow).toHaveAttribute("id", "eyebrow-id");
    expect(eyebrow).toHaveAttribute("aria-label", "meta");
  });
});

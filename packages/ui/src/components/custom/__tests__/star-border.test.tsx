// Suite: StarBorder
// Invariant: the travelling glows exist only while the border is animated and
// motion is allowed; otherwise the static brand rim stays. The effect is
// decorative and never takes over the wrapped control's role or name.
// Boundary OUT: the rendered gradient and glow motion — a visual contract
// verified by story screenshots.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "../../button";
import { StarBorder } from "../star-border";
import { UIProvider } from "../ui-provider";

function glows(container: HTMLElement) {
  return container.querySelectorAll('[data-slot="star-border-glow"]');
}

describe("StarBorder", () => {
  it("Should run a glow along each edge around the wrapped control by default", () => {
    const { container } = render(
      <StarBorder>
        <Button>Update</Button>
      </StarBorder>
    );

    const border = container.querySelector('[data-slot="star-border"]');
    expect(border).toHaveAttribute("data-animated", "true");
    expect(screen.getByRole("button", { name: "Update" })).toBeInTheDocument();
    expect(Array.from(glows(container), glow => glow.getAttribute("data-edge"))).toEqual([
      "top",
      "bottom",
    ]);
    expect(container.querySelector('[data-slot="star-border-rim"]')).toHaveAttribute(
      "aria-hidden",
      "true"
    );
  });

  it("Should keep only the static rim when not animated", () => {
    const { container } = render(
      <StarBorder animated={false}>
        <Button disabled>Updating…</Button>
      </StarBorder>
    );

    expect(container.querySelector('[data-slot="star-border"]')).toHaveAttribute(
      "data-animated",
      "false"
    );
    expect(container.querySelector('[data-slot="star-border-rim"]')).not.toBeNull();
    expect(glows(container)).toHaveLength(0);
  });

  it("Should keep only the static rim under reduced motion", () => {
    const { container } = render(
      <UIProvider reducedMotion="always">
        <StarBorder>
          <Button>Update</Button>
        </StarBorder>
      </UIProvider>
    );

    expect(container.querySelector('[data-slot="star-border"]')).toHaveAttribute(
      "data-animated",
      "false"
    );
    expect(glows(container)).toHaveLength(0);
  });
});

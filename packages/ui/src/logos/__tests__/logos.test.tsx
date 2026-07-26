import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ClaudeLogo } from "../claude";
import { GeminiLogo } from "../gemini";
import { LinearLogo } from "../linear";
import { OpenAILogo } from "../openai";

function getIds(container: HTMLElement, selector: string) {
  return Array.from(container.querySelectorAll<SVGElement>(selector), element => element.id);
}

describe("ClaudeLogo", () => {
  it("applies the provided className", () => {
    const { container } = render(<ClaudeLogo className="size-6" />);

    expect(container.querySelector("svg")?.getAttribute("class")).toContain("size-6");
  });
});

describe("GeminiLogo", () => {
  it("namespaces SVG defs per render", () => {
    const { container } = render(
      <>
        <GeminiLogo />
        <GeminiLogo />
      </>
    );

    const ids = getIds(container, "mask[id], filter[id]");
    const maskRefs = Array.from(container.querySelectorAll<SVGGElement>("g[mask]"), element =>
      element.getAttribute("mask")
    );

    expect(ids.length).toBeGreaterThan(0);
    expect(new Set(ids).size).toBe(ids.length);
    expect(new Set(maskRefs).size).toBe(maskRefs.length);
  });
});

describe("LinearLogo", () => {
  it("forwards decorative SVG props to every variant", () => {
    for (const variant of ["icon", "logo", "wordmark"] as const) {
      const { container, unmount } = render(
        <LinearLogo variant={variant} aria-hidden="true" data-testid={`linear-${variant}`} />
      );

      const svg = container.querySelector("svg");
      expect(svg).toHaveAttribute("aria-hidden", "true");
      expect(svg).toHaveAttribute("data-testid", `linear-${variant}`);
      unmount();
    }
  });

  it("namespaces icon defs per render", () => {
    const { container } = render(
      <>
        <LinearLogo variant="icon" />
        <LinearLogo variant="icon" />
      </>
    );

    const ids = getIds(container, "linearGradient[id], radialGradient[id], filter[id]");

    expect(ids.length).toBeGreaterThan(0);
    expect(new Set(ids).size).toBe(ids.length);
  });
});

describe("OpenAILogo", () => {
  it("forwards decorative SVG props to every variant", () => {
    for (const variant of ["icon", "wordmark"] as const) {
      const { container, unmount } = render(
        <OpenAILogo variant={variant} aria-hidden="true" data-testid={`openai-${variant}`} />
      );

      const svg = container.querySelector("svg");
      expect(svg).toHaveAttribute("aria-hidden", "true");
      expect(svg).toHaveAttribute("data-testid", `openai-${variant}`);
      unmount();
    }
  });
});

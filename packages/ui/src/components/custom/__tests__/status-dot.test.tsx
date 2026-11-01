import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusDot } from "../status-dot";

describe("StatusDot", () => {
  it.each([
    ["success", "solid"],
    ["warning", "solid"],
    ["destructive", "solid"],
    ["warning", "ring"],
    ["brand", "solid"],
    ["info", "solid"],
    ["faint", "ring"],
  ] as const)("Should expose data-tone/data-variant for %s/%s", (tone, variant) => {
    const { container } = render(<StatusDot tone={tone} variant={variant} />);
    const node = container.querySelector<HTMLElement>('[data-slot="status-dot"]');
    expect(node?.dataset.tone).toBe(tone);
    expect(node?.dataset.variant).toBe(variant);
  });

  it("Should expose an accessible label only when provided", () => {
    const { container: bare } = render(<StatusDot tone="brand" />);
    expect(bare.querySelector('[data-slot="status-dot"]')?.getAttribute("aria-hidden")).toBe(
      "true"
    );

    const { container: labeled } = render(<StatusDot tone="brand" label="Mentions" />);
    const node = labeled.querySelector('[data-slot="status-dot"]');
    expect(node?.getAttribute("aria-hidden")).toBeNull();
    expect(node?.getAttribute("aria-label")).toBe("Mentions");
    expect(node?.getAttribute("role")).toBe("img");
  });
});

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StreamMarkdown } from "../stream-markdown";

function proseRoot(container: HTMLElement): Element | null {
  return container.querySelector('[data-slot="stream-markdown"]');
}

describe("StreamMarkdown", () => {
  it("Should render the reading density by default", () => {
    const { container } = render(<StreamMarkdown># Title</StreamMarkdown>);
    expect(proseRoot(container)).not.toHaveAttribute("data-compact");
    expect(proseRoot(container)).not.toHaveAttribute("data-rhythm");
  });

  it("Should mark the prose root compact so parts select the dense tier", () => {
    const { container } = render(<StreamMarkdown compact># Title</StreamMarkdown>);
    expect(proseRoot(container)).toHaveAttribute("data-compact", "true");
    expect(proseRoot(container)).not.toHaveAttribute("data-rhythm");
  });

  it("Should keep the dense tier and mark the relaxed rhythm for muted reading panels", () => {
    const { container } = render(<StreamMarkdown compact="relaxed"># Title</StreamMarkdown>);
    expect(proseRoot(container)).toHaveAttribute("data-compact", "true");
    expect(proseRoot(container)).toHaveAttribute("data-rhythm", "relaxed");
  });

  it("Should give each density its own paragraph rhythm", () => {
    const rhythm = (compact?: boolean | "relaxed") =>
      proseRoot(render(<StreamMarkdown compact={compact}>text</StreamMarkdown>).container)
        ?.className;
    expect(rhythm()).toContain("[&_p]:my-3.5");
    expect(rhythm(true)).toContain("[&_p]:my-1");
    expect(rhythm("relaxed")).toContain("[&_p]:my-2");
    expect(rhythm("relaxed")).not.toContain("[&_p]:my-1 ");
  });
});

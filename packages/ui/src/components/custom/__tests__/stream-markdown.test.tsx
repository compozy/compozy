import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { markdownSlotSource } from "../markdown-slots";
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

  it("Should render slot nodes inside the markdown that surrounds them", () => {
    const { container } = render(
      <StreamMarkdown slots={{ m0: <span data-testid="slot-m0">chip</span> }}>
        {`**Read ${markdownSlotSource("m0")} now**\n\n- see [docs](#intro)`}
      </StreamMarkdown>
    );
    // The slot sits inside the bold run the source wrote around it.
    expect(screen.getByTestId("slot-m0").closest("strong")).toHaveTextContent("Read chip now");
    expect(container.querySelector('a[href="#m0"]')).toBeNull();
    // A link that names no slot stays an ordinary anchor.
    expect(screen.getByRole("link", { name: "docs" })).toHaveAttribute("href", "#intro");
  });
});

import { render, screen } from "@testing-library/react";
import * as React from "react";
import { describe, expect, it } from "vitest";

import { composeMarkdownSlots, type StreamMarkdownSlots } from "../markdown-slots";
import { StreamMarkdown } from "../stream-markdown";

function proseRoot(container: HTMLElement): Element | null {
  return container.querySelector('[data-slot="stream-markdown"]');
}

const TOKEN = "@src/a.ts";

function chipSlots(): StreamMarkdownSlots {
  return { m0: { node: <span data-testid="slot-m0">chip</span>, text: TOKEN } };
}

// Places slot `m0` over every occurrence of TOKEN, as verified mentions would.
function slotted(lines: readonly string[]): string {
  const text = lines.join("\n");
  const placements = [];
  for (let at = text.indexOf(TOKEN); at !== -1; at = text.indexOf(TOKEN, at + 1)) {
    placements.push({ id: "m0", start: at, end: at + TOKEN.length });
  }
  return composeMarkdownSlots(text, placements);
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
    render(
      <StreamMarkdown slots={chipSlots()}>
        {slotted([`**Read ${TOKEN} now**`, "", "- see [docs](#intro)"])}
      </StreamMarkdown>
    );
    // The slot sits inside the bold run the source wrote around it.
    expect(screen.getByTestId("slot-m0").closest("strong")).toHaveTextContent("Read chip now");
    // A link that names no slot stays an ordinary anchor.
    expect(screen.getByRole("link", { name: "docs" })).toHaveAttribute("href", "#intro");
  });

  it("Should keep authored links and sentinel characters as authored text, never as slots", () => {
    const authored = "Read [details](#m0) and [m0](#m0) \uE000m0\uE001";
    render(
      <StreamMarkdown slots={chipSlots()}>{composeMarkdownSlots(authored, [])}</StreamMarkdown>
    );
    expect(screen.queryByTestId("slot-m0")).toBeNull();
    expect(screen.getByRole("link", { name: "details" })).toHaveAttribute("href", "#m0");
    expect(screen.getByRole("link", { name: "m0" })).toHaveAttribute("href", "#m0");
    // Authored sentinel characters read as replacement characters.
    expect(screen.getByText(/\uFFFDm0\uFFFD/)).toBeInTheDocument();
  });

  it("Should render a slot placed inside any code as its literal text", () => {
    const { container } = render(
      <StreamMarkdown slots={chipSlots()}>
        {slotted([
          "Intro",
          "",
          `    indented ${TOKEN} here`,
          "",
          `Run \`inline ${TOKEN}\` now`,
          "",
          "```",
          `fenced ${TOKEN}`,
          "```",
        ])}
      </StreamMarkdown>
    );
    const code = Array.from(container.querySelectorAll("code"), node => node.textContent);
    expect(code).toEqual([`indented ${TOKEN} here`, `inline ${TOKEN}`, `fenced ${TOKEN}`]);
    expect(screen.queryByTestId("slot-m0")).toBeNull();
    expect(container.textContent).not.toMatch(/[\uE000-\uE002]/);
  });

  it("Should render a slot inside a link label or raw HTML as its literal text", () => {
    const { container } = render(
      <StreamMarkdown slots={chipSlots()}>
        {slotted([
          `See [the file ${TOKEN}](https://example.com/a) here.`,
          "",
          "<div>",
          `raw ${TOKEN}`,
          "</div>",
        ])}
      </StreamMarkdown>
    );
    expect(screen.getByRole("link", { name: `the file ${TOKEN}` })).toHaveAttribute(
      "href",
      "https://example.com/a"
    );
    expect(container).toHaveTextContent(`raw ${TOKEN}`);
    expect(screen.queryByTestId("slot-m0")).toBeNull();
    expect(container.textContent).not.toMatch(/[\uE000-\uE002]/);
  });

  it("Should keep slot nodes mounted when the slots object is rebuilt with the same content", () => {
    let mounts = 0;
    function Chip() {
      React.useEffect(() => {
        mounts += 1;
      }, []);
      return <span data-testid="slot-m0">chip</span>;
    }
    const source = slotted([`Read ${TOKEN}`]);
    const slots = () => ({ m0: { node: <Chip />, text: TOKEN } });
    const { rerender } = render(<StreamMarkdown slots={slots()}>{source}</StreamMarkdown>);
    rerender(<StreamMarkdown slots={slots()}>{source}</StreamMarkdown>);
    expect(screen.getByTestId("slot-m0")).toBeInTheDocument();
    expect(mounts).toBe(1);
  });
});

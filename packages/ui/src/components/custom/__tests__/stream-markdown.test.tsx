import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import * as React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { INLINE_CODE_CLASS } from "../markdown-prose-constants";
import { composeMarkdownSlots, type StreamMarkdownSlots } from "../markdown-slots";
import { StreamMarkdown } from "../stream-markdown";
import { MarkdownMermaid } from "../markdown-mermaid";

const renderMermaid = vi.hoisted(() => vi.fn());
vi.mock("../../../lib/mermaid-renderer", () => ({ renderMermaidImage: renderMermaid }));

describe("MarkdownMermaid", () => {
  const code = "flowchart LR\n A --> B";
  beforeEach(() => {
    renderMermaid.mockReset();
  });

  it("Should retain readable source while streaming without loading the renderer", () => {
    const { container } = render(<MarkdownMermaid code={code} streaming />);
    expect(container.querySelector("code")).toHaveTextContent("A --> B");
    expect(renderMermaid).not.toHaveBeenCalled();
  });

  it("Should render the diagram and keep its source accessible", async () => {
    renderMermaid.mockResolvedValue("data:image/svg+xml,test");
    const { container } = render(<MarkdownMermaid code={code} streaming={false} />);
    expect(await screen.findByRole("img", { name: "Mermaid diagram" })).toHaveAttribute(
      "src",
      "data:image/svg+xml,test"
    );
    fireEvent.click(screen.getByRole("button", { name: "View source" }));
    expect(container.querySelector("code")).toHaveTextContent("A --> B");
  });

  it("Should retain readable source when rendering fails", async () => {
    renderMermaid.mockRejectedValue(new Error("invalid diagram"));
    const { container } = render(<MarkdownMermaid code={code} streaming={false} />);
    await waitFor(() => expect(renderMermaid).toHaveBeenCalledOnce());
    expect(container.querySelector("code")).toHaveTextContent("A --> B");
    expect(screen.queryByRole("img")).toBeNull();
  });

  // Invariant: the fullscreen viewer supports zoom/reset and dismissal without
  // replacing the inline diagram or source. Owner: MarkdownMermaid; canonical suite here.
  it("Should zoom and reset the fullscreen diagram", async () => {
    renderMermaid.mockResolvedValue("data:image/svg+xml,test");
    render(<MarkdownMermaid code={code} streaming={false} />);
    await screen.findByRole("img", { name: "Mermaid diagram" });
    fireEvent.click(screen.getByRole("button", { name: "Fullscreen" }));
    const dialog = within(await screen.findByRole("dialog"));
    fireEvent.click(dialog.getByRole("button", { name: "Zoom in" }));
    expect(dialog.getByLabelText("Zoom")).toHaveTextContent("125%");
    fireEvent.click(dialog.getByRole("button", { name: "Zoom out" }));
    expect(dialog.getByLabelText("Zoom")).toHaveTextContent("100%");
    fireEvent.click(dialog.getByRole("button", { name: "Zoom in" }));
    fireEvent.click(dialog.getByRole("button", { name: "Reset zoom" }));
    expect(dialog.getByLabelText("Zoom")).toHaveTextContent("100%");
    fireEvent.click(dialog.getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("img", { name: "Mermaid diagram" })).toBeInTheDocument();
  });

  // Invariant: modified wheel/pinch and plain keys zoom the diagram within 25%–400% and claim
  // the event from page zoom; plain wheel and modified keys keep their browser defaults.
  // Browser geometry (cursor anchoring) is owned by the MermaidSizing story.
  it("Should zoom the fullscreen diagram with modified wheel and keyboard shortcuts", async () => {
    renderMermaid.mockResolvedValue("data:image/svg+xml,test");
    render(<MarkdownMermaid code={code} streaming={false} />);
    await screen.findByRole("img", { name: "Mermaid diagram" });
    fireEvent.click(screen.getByRole("button", { name: "Fullscreen" }));
    const dialog = within(await screen.findByRole("dialog"));
    const viewer = dialog.getByRole("region", { name: "Diagram viewer" });
    const zoom = dialog.getByLabelText("Zoom");

    expect(fireEvent.wheel(viewer, { deltaY: -100 })).toBe(true);
    expect(zoom).toHaveTextContent("100%");
    expect(fireEvent.wheel(viewer, { deltaY: -100, ctrlKey: true })).toBe(false);
    expect(zoom).toHaveTextContent("128%");
    expect(fireEvent.wheel(viewer, { deltaY: 200, metaKey: true })).toBe(false);
    expect(zoom).toHaveTextContent("100%");
    for (let step = 0; step < 200; step++) fireEvent.wheel(viewer, { deltaY: 5, ctrlKey: true });
    expect(zoom).toHaveTextContent("25%");
    expect(dialog.getByRole("button", { name: "Zoom out" })).toBeDisabled();
    for (let step = 0; step < 20; step++) fireEvent.keyDown(viewer, { key: "=" });
    expect(zoom).toHaveTextContent("400%");
    expect(dialog.getByRole("button", { name: "Zoom in" })).toBeDisabled();

    fireEvent.keyDown(viewer, { key: "0" });
    expect(zoom).toHaveTextContent("100%");
    fireEvent.keyDown(viewer, { key: "=" });
    fireEvent.keyDown(viewer, { key: "+" });
    expect(zoom).toHaveTextContent("150%");
    fireEvent.keyDown(viewer, { key: "-" });
    expect(zoom).toHaveTextContent("125%");
    expect(fireEvent.keyDown(viewer, { key: "0", metaKey: true })).toBe(true);
    expect(zoom).toHaveTextContent("125%");
    expect(fireEvent.keyDown(viewer, { key: " " })).toBe(false);
  });

  it("Should restore readable source when the diagram image cannot load", async () => {
    renderMermaid.mockResolvedValue("data:image/svg+xml,test");
    const { container } = render(<MarkdownMermaid code={code} streaming={false} />);
    fireEvent.error(await screen.findByRole("img", { name: "Mermaid diagram" }));
    expect(screen.queryByRole("img")).toBeNull();
    expect(container.querySelector("code")).toHaveTextContent("A --> B");
  });
});

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
  it("Should compose a caller image renderer with the shared fenced code renderer", () => {
    const { container } = render(
      <StreamMarkdown components={{ img: ({ alt }) => <span>{alt}</span> }}>
        {"![Reference](/local.png)\n\n```text\nhello\n```"}
      </StreamMarkdown>
    );
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByText("Reference")).toBeInTheDocument();
    expect(container.querySelector('[data-slot="code-block"]')).not.toBeNull();
  });

  it("Should render the reading density by default", () => {
    const { container } = render(<StreamMarkdown># Title</StreamMarkdown>);
    expect(proseRoot(container)).not.toHaveAttribute("data-compact");
    expect(proseRoot(container)).not.toHaveAttribute("data-rhythm");
  });

  it("Should leave the reading measure to the caller", () => {
    const { container, rerender } = render(<StreamMarkdown># Title</StreamMarkdown>);
    expect(proseRoot(container)?.className).not.toMatch(/(^|\s)max-w-/);
    rerender(<StreamMarkdown className="max-w-prose"># Title</StreamMarkdown>);
    expect(proseRoot(container)).toHaveClass("max-w-prose");
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

  // Invariant: typed chat input keeps each authored line break as a line break, its
  // block structure, and literal code; model output keeps Markdown's soft breaks.
  // Owner: StreamMarkdown `authored` mode; canonical suite: this file.
  it("Should render authored line breaks as breaks without changing code or block structure", () => {
    const source = [
      "first line",
      "second line",
      "",
      "- alpha",
      "- beta",
      "  - nested",
      "",
      "```",
      "a",
      "b",
      "```",
    ].join("\n");
    const { container } = render(<StreamMarkdown authored>{source}</StreamMarkdown>);
    const paragraph = container.querySelector("p")!;
    expect(paragraph.querySelectorAll("br")).toHaveLength(1);
    expect(paragraph.textContent?.replace(/\n/g, "")).toBe("first linesecond line");
    expect(
      Array.from(container.querySelectorAll("li"), li => li.firstChild?.textContent?.trim())
    ).toEqual(["alpha", "beta", "nested"]);
    expect(
      Array.from(
        container.querySelectorAll('[data-slot="code-block-line"]'),
        line => line.textContent
      )
    ).toEqual(["a", "b"]);

    const model = render(<StreamMarkdown>{source}</StreamMarkdown>);
    expect(model.container.querySelector("p")!.querySelectorAll("br")).toHaveLength(0);
  });

  it("Should keep authored line breaks around slots", () => {
    const { container } = render(
      <StreamMarkdown authored slots={chipSlots()}>
        {slotted([`${TOKEN} first`, "second"])}
      </StreamMarkdown>
    );
    const paragraph = container.querySelector("p")!;
    expect(within(paragraph).getByTestId("slot-m0")).toBeInTheDocument();
    expect(paragraph.querySelectorAll("br")).toHaveLength(1);
  });

  // Invariant: authored links open only safe external destinations, in a new tab
  // without opener/referrer, with the full address available while truncated.
  it("Should open only safe authored links behind the external-link affordance", () => {
    const longUrl = `https://example.com/${"segment/".repeat(20)}end`;
    const { container } = render(
      <StreamMarkdown authored>
        {`${longUrl} [Email](mailto:hello@example.com) [unsafe](javascript:alert(1))`}
      </StreamMarkdown>
    );
    const links = screen.getAllByRole("link");
    expect(links.map(link => link.getAttribute("href"))).toEqual([
      longUrl,
      "mailto:hello@example.com",
    ]);
    for (const link of links) {
      expect(link).toHaveAttribute("target", "_blank");
      expect(link).toHaveAttribute("rel", "noopener noreferrer");
      expect(link).toHaveAttribute("title", link.getAttribute("href"));
      expect(link.querySelector("svg[aria-hidden='true']")).not.toBeNull();
      expect(link.querySelector(".truncate")).not.toBeNull();
    }
    expect(container).toHaveTextContent("unsafe");
    expect(container.querySelector("a[href^='javascript']")).toBeNull();
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

  // UT-050. Invariant: the inline-code decorator sees slot-restored literal text and the
  // default element; without it inline code renders the shared recipe unchanged.
  it("Should decorate inline code with slot-restored text and the default element", () => {
    const seen: { text: string; element: React.ReactElement }[] = [];
    render(
      <StreamMarkdown
        slots={chipSlots()}
        inlineCodeDecorator={code => {
          seen.push(code);
          return <mark data-testid="decorated">{code.element}</mark>;
        }}
      >
        {slotted([`Open \`${TOKEN}:12\` and \`plain\`.`, "", "```", `fenced ${TOKEN}`, "```"])}
      </StreamMarkdown>
    );
    expect(seen.map(code => code.text)).toEqual([`${TOKEN}:12`, "plain"]);
    for (const { text } of seen) expect(text).not.toMatch(/[\uE000-\uE002]/);
    const decorated = screen.getAllByTestId("decorated");
    expect(decorated.map(node => node.querySelector("code")?.textContent)).toEqual([
      `${TOKEN}:12`,
      "plain",
    ]);
    expect(decorated[0]!.querySelector("code")).toHaveClass(...INLINE_CODE_CLASS.split(" "));

    const plain = render(<StreamMarkdown>{"Open `a.ts:12` now."}</StreamMarkdown>);
    expect(plain.container.querySelector("p code")?.outerHTML).toBe(
      `<code class="${INLINE_CODE_CLASS}">a.ts:12</code>`
    );
  });

  // Invariant: with a link base, document-relative links and images resolve against the
  // document's directory before safe mode, and links leaving the root stay blocked.
  it("Should resolve document-relative links and images against the link base", () => {
    function LocalLink({ href, children }: { href: string; children: React.ReactNode }) {
      return (
        <button type="button" data-href={href}>
          {children}
        </button>
      );
    }
    const { container } = render(
      <StreamMarkdown localLinkComponent={LocalLink} linkBase="docs/guide">
        {"[a](setup.md#install) [b](../api/x.md) [c](#top) [d](../../../out.md) ![i](img/p.png)"}
      </StreamMarkdown>
    );
    expect(screen.getByRole("link", { name: "a" })).toHaveAttribute(
      "href",
      "/docs/guide/setup.md#install"
    );
    expect(screen.getByRole("link", { name: "b" })).toHaveAttribute("href", "/docs/api/x.md");
    expect(screen.getByRole("link", { name: "c" })).toHaveAttribute("href", "#top");
    expect(container).toHaveTextContent("d [blocked]");
    expect(screen.getByRole("img", { name: "i" })).toHaveAttribute("src", "/docs/guide/img/p.png");

    // A `#` or `?` in a directory name stays part of the path.
    render(<StreamMarkdown linkBase="docs/topic#1?">{"[next](next.md)"}</StreamMarkdown>);
    expect(screen.getByRole("link", { name: "next" })).toHaveAttribute(
      "href",
      "/docs/topic%231%3F/next.md"
    );
  });

  // UT-051. Invariant: links the safe mode blocks route to the caller's local-link component
  // with their authored href; external and script-capable destinations never do, and without
  // the component the blocked rendering is unchanged.
  it("Should route only safe-mode-blocked local links to the local-link component", () => {
    const source = [
      "[a](internal/x.go:42) [b](~/c.toml) [c](file:///Users/d/a.md) [d](C:/x/y.ts)",
      "[e](https://x.dev/) [f](javascript:alert(1)) [g](<java\tscript:alert(1)>)",
      "[h](vbscript:msgbox(1)) [i](data:text/html,x) [j](blob:https://x.dev/1)",
    ].join(" ");
    function LocalLink({ href, children }: { href: string; children: React.ReactNode }) {
      return (
        <button type="button" data-href={href}>
          {children}
        </button>
      );
    }
    const { container } = render(
      <StreamMarkdown localLinkComponent={LocalLink}>{source}</StreamMarkdown>
    );
    expect(
      screen.getAllByRole("button").map(node => [node.textContent, node.getAttribute("data-href")])
    ).toEqual([
      ["a", "internal/x.go:42"],
      ["b", "~/c.toml"],
      ["c", "file:///Users/d/a.md"],
      ["d", "C:/x/y.ts"],
    ]);
    expect(screen.getByRole("link", { name: "e" })).toHaveAttribute("href", "https://x.dev/");
    expect(container).toHaveTextContent("f [blocked]");
    expect(container).toHaveTextContent("g [blocked]");
    for (const name of ["h", "i", "j"]) {
      expect(container).toHaveTextContent(`${name} [blocked]`);
    }

    const fallback = render(<StreamMarkdown>{source}</StreamMarkdown>);
    expect(
      Array.from(fallback.container.querySelectorAll("span[title^='Blocked URL']"), node => [
        node.getAttribute("title"),
        node.textContent,
      ])
    ).toEqual([
      ["Blocked URL: internal/x.go:42", "a [blocked]"],
      ["Blocked URL: ~/c.toml", "b [blocked]"],
      ["Blocked URL: undefined", "c [blocked]"],
      ["Blocked URL: undefined", "d [blocked]"],
      ["Blocked URL: undefined", "f [blocked]"],
      ["Blocked URL: undefined", "g [blocked]"],
      ["Blocked URL: undefined", "h [blocked]"],
      ["Blocked URL: undefined", "i [blocked]"],
      ["Blocked URL: undefined", "j [blocked]"],
    ]);
  });
});

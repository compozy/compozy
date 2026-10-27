import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { useState } from "react";
import { Button } from "../../button";

import {
  composeMarkdownSlots,
  type MarkdownSlotPlacement,
  type StreamMarkdownSlots,
} from "../markdown-slots";
import { StreamMarkdown } from "../stream-markdown";

const meta: Meta<typeof StreamMarkdown> = {
  title: "components/custom/StreamMarkdown",
  component: StreamMarkdown,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Chat-thread wrapper around `Markdown` with the Compozy `CodeBlock` for code. `slots` places inline nodes where a `composeMarkdownSlots` source places them: the slot is part of the parsed document, so bold, lists, and headings form around it as written. Inside code, a link label, or raw HTML a slot reads as its literal `text`.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-full max-w-3xl bg-background p-6">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const MERMAID =
  "```mermaid\nflowchart LR\n  A[Spec] --> B[Implement] --> C[Review]\n```\n\n## Next steps\n\nReview the result.";

export const MermaidDiagram: Story = {
  args: { children: MERMAID },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      await canvas.findByRole("img", { name: "Mermaid diagram" }, { timeout: 15000 })
    ).toBeVisible();
    await userEvent.click(canvas.getByRole("button", { name: "View source" }));
    await expect(canvas.getByText(/flowchart LR/)).toBeVisible();
    await expect(canvas.getByRole("heading", { name: "Next steps" })).toBeVisible();
  },
};

export const MermaidStreaming: Story = {
  args: { children: MERMAID, streaming: true },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByText(/flowchart LR/)).toBeVisible();
    await expect(canvas.queryByRole("img")).toBeNull();
  },
};

export const MermaidInvalid: Story = {
  args: { children: "```mermaid\nnot a valid diagram\n```\n\nThe source remains readable." },
};

const MULTIPLE_DIAGRAMS = [
  "flowchart TD\n A[Request] --> B[Validate] --> C[Plan] --> D[Implement] --> E[Test] --> F[Review] --> G[Deliver]",
  "sequenceDiagram\n participant User\n participant Agent\n User->>Agent: Explain the change\n Agent-->>User: Plan\n User->>Agent: Continue\n Agent-->>User: Result",
  "flowchart LR\n A[Input] --> B[Output]",
]
  .map(code => `\`\`\`mermaid\n${code}\n\`\`\``)
  .join("\n\nFollowing prose.\n\n");

function MermaidTransition() {
  const [streaming, setStreaming] = useState(true);
  return (
    <>
      <Button onClick={() => setStreaming(false)}>Finish streaming</Button>
      <StreamMarkdown streaming={streaming}>{MULTIPLE_DIAGRAMS}</StreamMarkdown>
    </>
  );
}

// Invariant: real Mermaid rendering never changes reserved transcript geometry,
// and inline images fit half the column without exceeding their natural size.
export const MermaidSizing: Story = {
  render: () => <MermaidTransition />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const blocks = [...canvasElement.querySelectorAll('[data-slot="markdown-mermaid"]')];
    const heights = blocks.map(block => block.getBoundingClientRect().height);
    await userEvent.click(canvas.getByRole("button", { name: "Finish streaming" }));
    await waitFor(
      () => expect(canvas.getAllByRole("img", { name: "Mermaid diagram" })).toHaveLength(3),
      { timeout: 15000 }
    );
    const images = canvas.getAllByRole("img", { name: "Mermaid diagram" });
    for (const [index, image] of images.entries()) {
      await (image as HTMLImageElement).decode();
      const bounds = image.getBoundingClientRect();
      await expect(blocks[index]!.getBoundingClientRect().height).toBe(heights[index]);
      await expect(bounds.width).toBeLessThanOrEqual(
        blocks[index]!.getBoundingClientRect().width / 2
      );
      await expect(bounds.width).toBeLessThanOrEqual((image as HTMLImageElement).naturalWidth);
      await expect(bounds.height).toBeLessThanOrEqual(window.innerHeight * 0.4);
    }
    // Browser geometry owns the keyboard-pan invariant; jsdom cannot measure transformed images.
    await userEvent.click(canvas.getAllByRole("button", { name: "Fullscreen" })[0]!);
    const overlay = within(canvasElement.ownerDocument.body);
    const viewer = overlay.getByRole("region", { name: "Diagram viewer" });
    const expandedImage = within(viewer).getByRole("img", { name: "Mermaid diagram" });
    await (expandedImage as HTMLImageElement).decode();
    await waitFor(() =>
      expect(
        overlay
          .getByRole("dialog")
          .getAnimations({ subtree: true })
          .filter(animation => animation.playState === "running")
      ).toHaveLength(0)
    );
    viewer.focus();
    const initial = expandedImage.getBoundingClientRect();
    await userEvent.keyboard("{ArrowRight}{ArrowDown}");
    await expect(expandedImage.getBoundingClientRect().left).toBeGreaterThan(initial.left);
    await expect(expandedImage.getBoundingClientRect().top).toBeGreaterThan(initial.top);
    await userEvent.click(overlay.getByRole("button", { name: "Reset zoom" }));
    await expect(expandedImage.getBoundingClientRect().left).toBeCloseTo(initial.left);
    await expect(expandedImage.getBoundingClientRect().top).toBeCloseTo(initial.top);
    await userEvent.keyboard("{Escape}");
  },
};

// Invariant: diagram-authored directives cannot relax the image renderer's security policy.
// This browser story owns the real Mermaid/SVG boundary; no renderer mock is used.
export const MermaidSecurity: Story = {
  args: {
    children:
      '```mermaid\n%%{init: {"securityLevel":"loose","htmlLabels":true,"flowchart":{"htmlLabels":true}}}%%\nflowchart LR\n  A["<b>Source label</b>"] --> B[Safe image]\n  click A "javascript:alert(1)"\n```',
  },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const diagram = await canvas.findByRole("img", { name: "Mermaid diagram" }, { timeout: 15000 });
    await expect(diagram).toBeVisible();
    const svg = decodeURIComponent(diagram.getAttribute("src") ?? "");
    await expect(svg).not.toContain("foreignObject");
    await expect(svg).not.toContain("javascript:");
    await expect(canvasElement.querySelector("svg[id^='mermaid-']")).toBeNull();
  },
};

function Chip({ label }: { label: string }) {
  return (
    <span
      data-testid="story-slot-chip"
      className="inline-flex max-w-full rounded-xs border border-line px-1 py-px align-baseline font-mono text-form-label text-info"
    >
      {label}
    </span>
  );
}

const SLOTS: StreamMarkdownSlots = {
  spec: { node: <Chip label="_spec.md" />, text: "@.compozy/tasks/mentions/_spec.md" },
  store: { node: <Chip label="internal/store" />, text: "@internal/store/" },
  review: { node: <Chip label="Review session" />, text: "@session:sess_review" },
};

// Places a slot at every occurrence of a slot's literal text, as verified mentions would.
function withSlots(lines: readonly string[]): string {
  const text = lines.join("\n");
  const placements: MarkdownSlotPlacement[] = [];
  for (const [id, slot] of Object.entries(SLOTS)) {
    for (let at = text.indexOf(slot.text); at !== -1; at = text.indexOf(slot.text, at + 1)) {
      placements.push({ id, start: at, end: at + slot.text.length });
    }
  }
  return composeMarkdownSlots(
    text,
    placements.sort((a, b) => a.start - b.start)
  );
}

const SLOTTED = withSlots([
  "## Plan for @.compozy/tasks/mentions/_spec.md",
  "",
  "**Start with @internal/store/** and keep the migration append-only.",
  "",
  "- Compare against @session:sess_review before shipping.",
  "- Leave _@.compozy/tasks/mentions/_spec.md_ untouched.",
]);

export const InlineSlots: Story = {
  render: () => <StreamMarkdown slots={SLOTS}>{SLOTTED}</StreamMarkdown>,
};

const IN_CODE = withSlots([
  "Inline: `cat @internal/store/`",
  "",
  "    indented @.compozy/tasks/mentions/_spec.md",
  "",
  "```sh",
  "rg mention @internal/store/",
  "```",
  "",
  "See [the store @internal/store/](https://example.com/store).",
]);

export const SlotsAsLiteralText: Story = {
  tags: ["play-fn"],
  parameters: {
    docs: {
      description: {
        story:
          "A slot inside code or a link label reads as its literal text, never as its node or its sentinel.",
      },
    },
  },
  render: () => <StreamMarkdown slots={SLOTS}>{IN_CODE}</StreamMarkdown>,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.queryByTestId("story-slot-chip")).toBeNull();
    await expect(canvasElement).toHaveTextContent("cat @internal/store/");
    await expect(canvas.getByRole("link", { name: "the store @internal/store/" })).toBeVisible();
  },
};

export const ImageReference: Story = {
  args: {
    children: "![Preview](/tmp/render/preview.png)",
    components: { img: ({ src }) => <code>{typeof src === "string" ? src : "image"}</code> },
  },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByText("/tmp/render/preview.png")).toBeVisible();
    await expect(canvas.queryByRole("img")).toBeNull();
  },
};

const AUTHORED = [
  "first line",
  "second line, see https://example.com/docs/session-transcript/whitespace-and-links-in-authored-messages",
  "",
  "- alpha",
  "- beta",
  "  - nested",
  "",
  "[unsafe](javascript:alert(1)) stays text",
].join("\n");

export const AuthoredInput: Story = {
  tags: ["play-fn"],
  parameters: {
    docs: {
      description: {
        story:
          "Typed chat input: each authored line break stays a line break, and only http(s)/mailto links open, externally, behind the globe with long URLs truncated.",
      },
    },
  },
  render: () => (
    <div className="max-w-80">
      <StreamMarkdown authored>{AUTHORED}</StreamMarkdown>
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvasElement.querySelector("p br")).not.toBeNull();
    const link = canvas.getByRole("link");
    await expect(link).toHaveAttribute("target", "_blank");
    await expect(link).toHaveAttribute("rel", "noopener noreferrer");
    await expect(canvasElement).toHaveTextContent("unsafe stays text");
  },
};

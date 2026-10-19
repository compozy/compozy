import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { markdownSlotSource, type StreamMarkdownSlots } from "../markdown-slots";
import { StreamMarkdown } from "../stream-markdown";

const meta: Meta<typeof StreamMarkdown> = {
  title: "components/custom/StreamMarkdown",
  component: StreamMarkdown,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Chat-thread wrapper around `Markdown` with the Compozy `CodeBlock` for code. `slots` places inline nodes where the source writes `markdownSlotSource(id)`: the slot is part of the parsed document, so bold, lists, and headings form around it as written. Inside inline, fenced, or indented code a slot reads as its literal `text`.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[720px] bg-background p-6">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

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

const SLOTTED = [
  `## Plan for ${markdownSlotSource("spec")}`,
  "",
  `**Start with ${markdownSlotSource("store")}** and keep the migration append-only.`,
  "",
  `- Compare against ${markdownSlotSource("review")} before shipping.`,
  `- Leave _${markdownSlotSource("spec")}_ untouched.`,
].join("\n");

export const InlineSlots: Story = {
  render: () => <StreamMarkdown slots={SLOTS}>{SLOTTED}</StreamMarkdown>,
};

const IN_CODE = [
  `Inline: \`cat ${markdownSlotSource("store")}\``,
  "",
  `    indented ${markdownSlotSource("spec")}`,
  "",
  "```sh",
  `rg mention ${markdownSlotSource("store")}`,
  "```",
].join("\n");

export const SlotsInsideCode: Story = {
  tags: ["play-fn"],
  parameters: {
    docs: {
      description: {
        story:
          "A slot inside any code reads as its literal text, never as its node or its placeholder source.",
      },
    },
  },
  render: () => <StreamMarkdown slots={SLOTS}>{IN_CODE}</StreamMarkdown>,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.queryByTestId("story-slot-chip")).toBeNull();
    await expect(canvasElement).toHaveTextContent("cat @internal/store/");
    await expect(canvasElement).not.toHaveTextContent("](#");
  },
};

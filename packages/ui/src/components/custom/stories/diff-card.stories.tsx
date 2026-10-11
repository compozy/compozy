import type { Meta, StoryObj } from "@storybook/react-vite";

import { DiffCard, type DiffCardLine } from "../diff-card";

const meta: Meta<typeof DiffCard> = {
  title: "components/custom/DiffCard",
  component: DiffCard,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "A file change as one quiet card: path and colored +a −d stat in the header, then added/removed lines on a faint tint with a leading bar and sign column; context lines stay in the subtle ink.",
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

const EDIT_LINES: DiffCardLine[] = [
  {
    id: "c0",
    kind: "context",
    text: "export function resolveToolDisplay(input: ToolDisplayInput) {",
  },
  { id: "d1", kind: "del", text: '  const name = input.toolName ?? "tool";' },
  { id: "a1", kind: "add", text: "  const name = resolveRegisteredToolName(input.toolName);" },
  { id: "a2", kind: "add", text: "  const registered = registeredDisplay(name, input.args);" },
  { id: "c2", kind: "context", text: "  if (registered) return registered;" },
];

export const Edit: Story = {
  args: { path: "…/session/lib/tool-display.ts", additions: 2, deletions: 1, lines: EDIT_LINES },
};

export const WrittenFile: Story = {
  args: {
    path: "docs/notes.md",
    additions: 3,
    deletions: 0,
    lines: [
      { id: "a0", kind: "add", text: "# Notes" },
      { id: "a1", kind: "add", text: "" },
      { id: "a2", kind: "add", text: "Tool rows read as a verb and its object." },
    ],
  },
};

export const WithFooter: Story = {
  args: {
    path: "web/src/styles.css",
    additions: 60,
    deletions: 0,
    lines: Array.from({ length: 8 }, (_, index) => ({
      id: `a${index}`,
      kind: "add" as const,
      text: `.session-rail-step-${index} { position: relative; }`,
    })),
    footer: <span className="text-transcript-meta text-subtle">Show full diff (60 lines)</span>,
  },
};

import type { Meta, StoryObj } from "@storybook/react-vite";

import { DescriptionCard } from "../description-card";

const meta: Meta<typeof DescriptionCard> = {
  title: "components/custom/DescriptionCard",
  component: DescriptionCard,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Operator-authored markdown rendered through the `streamdown` safe-mode contract. Strips raw HTML, blocks dangerous URL schemes, swaps external images for `[image: alt]` fallbacks.",
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

const PROSE = [
  "# Refactor auth flow",
  "",
  "## Acceptance criteria",
  "",
  "- Session tokens rotate every 24h.",
  "- Refresh endpoint returns `401` on revoked sessions.",
  "- Audit log writes happen inside the same transaction.",
  "",
  "## Constraints",
  "",
  "1. No breaking schema migration.",
  "2. Backwards-compatible with existing webhook signatures.",
  "",
  "Inline `kbd` shortcut sample: press `Cmd+K` to open the command palette.",
  "",
  "```ts",
  "function rotateSession(id: string) {",
  "  return supabase.rpc('rotate_session', { id });",
  "}",
  "```",
  "",
  "| step | owner | status |",
  "|------|-------|--------|",
  "| spec | planner | done |",
  "| impl | scout   | in-progress |",
].join("\n");

/** Markdown sample exercising headings, lists, fenced code, inline code, and tables. */
export const Default: Story = {
  args: { children: PROSE },
};

/** Inline code + bold + italic stress. */
export const InlineEmphasis: Story = {
  args: {
    children:
      "**Important**: the `agh-network` protocol expects `_method` to stay snake_case. _Avoid_ camelCase rewrites.",
  },
};

/** XSS payload corpus — every sample neutralised by the safe-mode contract. */
export const XssResilience: Story = {
  args: {
    children: [
      "Operator markdown with hostile payloads:",
      "",
      "`<script>alert(1)</script>` renders as text.",
      "",
      "[Bad link](javascript:alert(1)) — href stripped.",
      "",
      "External image `![banner](https://example.com/banner.png)` collapses to a textual fallback.",
      "",
      "Inline `<iframe>` / `<style>` / `<form>` tags are dropped.",
    ].join("\n"),
  },
};

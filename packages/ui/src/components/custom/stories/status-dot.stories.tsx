import type { Meta, StoryObj } from "@storybook/react-vite";

import { Eyebrow } from "../eyebrow";
import { StatusDot } from "../status-dot";

const meta: Meta<typeof StatusDot> = {
  title: "components/custom/StatusDot",
  component: StatusDot,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Five-tone glyph inbox vocabulary — warning solid (Needs review), danger solid (Blocked), warning ring (Stuck), accent solid (Mentions), faint ring (Updates). Composes next to an Eyebrow group label.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Inbox group label vocabulary — the canonical lineup. */
export const InboxVocabulary: Story = {
  args: { tone: "warning" },
  render: () => (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <StatusDot tone="warning" variant="solid" label="Needs review" />
        <Eyebrow>Needs review</Eyebrow>
      </div>
      <div className="flex items-center gap-2">
        <StatusDot tone="danger" variant="solid" label="Blocked" />
        <Eyebrow>Blocked</Eyebrow>
      </div>
      <div className="flex items-center gap-2">
        <StatusDot tone="warning" variant="ring" label="Stuck" />
        <Eyebrow>Stuck</Eyebrow>
      </div>
      <div className="flex items-center gap-2">
        <StatusDot tone="accent" variant="solid" label="Mentions" />
        <Eyebrow>Mentions</Eyebrow>
      </div>
      <div className="flex items-center gap-2">
        <StatusDot tone="faint" variant="ring" label="Updates" />
        <Eyebrow>Updates</Eyebrow>
      </div>
    </div>
  ),
};

/** Size — default (6 px) vs sm (5 px). */
export const Sizes: Story = {
  args: { tone: "accent" },
  render: () => (
    <div className="flex items-center gap-4">
      <StatusDot tone="accent" />
      <StatusDot tone="accent" size="sm" />
    </div>
  ),
};

/** Bare dot — no aria-label; aria-hidden defaults to true. */
export const Decorative: Story = {
  args: { tone: "danger" },
};

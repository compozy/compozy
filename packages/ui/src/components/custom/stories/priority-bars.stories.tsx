import type { Meta, StoryObj } from "@storybook/react-vite";

import { PriorityBars } from "../priority-bars";

const meta: Meta<typeof PriorityBars> = {
  title: "components/custom/PriorityBars",
  component: PriorityBars,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Three-bar priority glyph Heights are always 4 / 8 / 12 px ascending; the `level` prop drives bar color (low → `--faint`, medium → `--fg`, high → `--warning`, urgent → `--danger`) — never bar count. There is no `tone` prop.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="flex items-end gap-6 bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** All four levels rendered side by side; bars are recolored, never recounted. */
export const Levels: Story = {
  args: {},
  render: () => (
    <>
      <PriorityBars level="low" />
      <PriorityBars level="medium" />
      <PriorityBars level="high" />
      <PriorityBars level="urgent" />
    </>
  ),
};

export const Low: Story = { args: { level: "low" } };
export const Medium: Story = { args: { level: "medium" } };
export const High: Story = { args: { level: "high" } };
export const Urgent: Story = { args: { level: "urgent" } };

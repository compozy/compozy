import type { Meta, StoryObj } from "@storybook/react-vite";

import { StatusBadge } from "../status-badge";

const meta: Meta<typeof StatusBadge> = {
  title: "ui/StatusBadge",
  component: StatusBadge,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Compact tone badge for health, run, and review status metadata.",
      },
    },
  },
  tags: ["autodocs"],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Neutral metadata badge.
 */
export const Neutral: Story = {
  args: {
    children: "idle",
    tone: "neutral",
  },
};

/**
 * Representative daemon operator tones shown together.
 */
export const ToneMatrix: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap gap-3">
      <StatusBadge tone="accent">running</StatusBadge>
      <StatusBadge tone="success">ready</StatusBadge>
      <StatusBadge tone="warning">degraded</StatusBadge>
      <StatusBadge tone="info">pending</StatusBadge>
      <StatusBadge tone="danger">failed</StatusBadge>
      <StatusBadge tone="neutral">idle</StatusBadge>
    </div>
  ),
};

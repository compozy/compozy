import type { Meta, StoryObj } from "@storybook/react-vite";

import { PillDot } from "../pill";
import { StatusBreakdown } from "../status-breakdown";

const meta: Meta<typeof StatusBreakdown> = {
  title: "components/custom/StatusBreakdown",
  component: StatusBreakdown,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Row-per-bucket breakdown using a flat 6px track on `--canvas`, tone-mapped fills, and a mono tabular value column. Use for queue/run summaries inside detail panels.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[480px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Run status summary with the desaturated signal palette.
 */
export const RunStatus: Story = {
  args: {},
  render: () => (
    <StatusBreakdown
      items={[
        { label: "Succeeded", value: 124, tone: "success" },
        { label: "Failed", value: 3, tone: "danger" },
        { label: "Skipped", value: 7, tone: "neutral" },
        { label: "Pending", value: 14, tone: "warning" },
      ]}
    />
  ),
};

/**
 * Single-bucket breakdown — the bar still renders quietly under the row.
 */
export const SingleBucket: Story = {
  args: {},
  render: () => <StatusBreakdown items={[{ label: "Completed", value: 12, tone: "accent" }]} />,
};

export const FormattedCount: Story = {
  args: {
    total: 256_000,
    items: [{ label: "Context", value: 12_400, formattedValue: "≈ 12.4K", tone: "accent" }],
  },
};

export const LabelAndValue: Story = {
  args: {
    items: [
      {
        label: "Agent & conversation",
        swatch: <PillDot tone="neutral" />,
        value: 77_300,
        formattedValue: "77.3K",
        showBar: false,
      },
      {
        label: "Free",
        swatch: <PillDot color="var(--color-canvas-tint)" />,
        value: 166_300,
        formattedValue: "166.3K",
        showBar: false,
      },
    ],
  },
};

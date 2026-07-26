import type { Meta, StoryObj } from "@storybook/react-vite";

import { QueueHealthSparkline, type QueueHealthSparklineBucket } from "../queue-health-sparkline";

const meta: Meta<typeof QueueHealthSparkline> = {
  title: "components/custom/QueueHealthSparkline",
  component: QueueHealthSparkline,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Thin `recharts` `<BarChart>` adapter. Applies AGH tokens: default bar `var(--color-bar-fill)`; `stuck` buckets render with `var(--color-accent-tint-strong)`. Consumed by the Tasks dashboard queue-health panel.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[420px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const BASE: QueueHealthSparklineBucket[] = Array.from({ length: 24 }, (_, index) => ({
  label: `${24 - index}h`,
  value: 2 + Math.round(Math.sin((index / 24) * Math.PI * 2) * 3 + 4),
}));

const STUCK: QueueHealthSparklineBucket[] = BASE.map((bucket, index) => ({
  ...bucket,
  stuck: index >= BASE.length - 2 ? true : bucket.stuck,
}));

/** Default 24-bucket queue depth across the last day. */
export const Default: Story = {
  args: {
    data: BASE,
    ariaLabel: "Queue depth last 24 hours",
  },
};

/** Stuck — the most recent two buckets paint with `--accent-tint-strong`. */
export const StuckTail: Story = {
  args: {
    data: STUCK,
    ariaLabel: "Queue depth with stuck buckets",
  },
};

/** Compact — shorter height tuned for inline use. */
export const Compact: Story = {
  args: {
    data: BASE,
    height: 48,
    ariaLabel: "Queue depth last 24 hours, compact",
  },
};

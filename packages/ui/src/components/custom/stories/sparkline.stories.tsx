import type { Meta, StoryObj } from "@storybook/react-vite";

import { Sparkline } from "../sparkline";

const meta: Meta<typeof Sparkline> = {
  title: "components/custom/Sparkline",
  component: Sparkline,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Tiny bar sparkline tracking a single signal series. Bars fill the container by default; max defaults to the largest value (or 1). Bar color is `--accent-tint-strong` so the sparkline reads as the operator-orange brand without screaming. Pair with `Metric` for inline trend visualisation.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[280px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default 14-bucket trend.
 */
export const Default: Story = {
  args: {},
  render: () => (
    <Sparkline
      ariaLabel="Sessions per hour, last 14 hours"
      values={[3, 4, 6, 5, 8, 9, 7, 12, 14, 11, 13, 15, 12, 10]}
    />
  ),
};

/**
 * Tall variant with explicit `max` so the trend reads against a fixed ceiling.
 */
export const FixedMax: Story = {
  args: {},
  render: () => (
    <Sparkline
      ariaLabel="Provider requests vs quota"
      height={48}
      max={100}
      values={[12, 18, 24, 30, 22, 35, 41, 38]}
    />
  ),
};

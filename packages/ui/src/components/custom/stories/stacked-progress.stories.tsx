import type { Meta, StoryObj } from "@storybook/react-vite";

import { StackedProgress } from "../stacked-progress";

const meta: Meta<typeof StackedProgress> = {
  title: "components/custom/StackedProgress",
  component: StackedProgress,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Flat 6px stacked-progress track on `--canvas`. Each segment adopts a signal tone; segments with `value <= 0` are dropped from the render. Use for queue health, run distribution, or capacity meters.",
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
 * Three-tone distribution sized as fractions of the inferred total.
 */
export const Distribution: Story = {
  args: {},
  render: () => (
    <StackedProgress
      ariaLabel="Run distribution"
      segments={[
        { value: 12, tone: "success" },
        { value: 4, tone: "warning" },
        { value: 2, tone: "danger" },
      ]}
    />
  ),
};

/**
 * Explicit `total` lets the track render unfilled remaining capacity.
 */
export const PartialCapacity: Story = {
  args: {},
  render: () => (
    <StackedProgress
      ariaLabel="Provider quota"
      total={100}
      segments={[
        { value: 60, tone: "accent" },
        { value: 12, tone: "info" },
      ]}
    />
  ),
};

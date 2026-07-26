import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorySurface } from "@/storybook/story-layout";

import { LoopDeleteAction } from "../detail/loop-delete-action";

const meta: Meta<typeof LoopDeleteAction> = {
  title: "systems/loops/components/LoopDeleteAction",
  component: LoopDeleteAction,
  parameters: { layout: "fullscreen" },
  render: args => (
    <StorySurface className="flex min-h-[640px] items-center justify-center">
      <LoopDeleteAction {...args} />
    </StorySurface>
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Confirmation: Story = {
  args: {
    loopName: "reviews-watch",
    isPending: false,
    error: null,
    defaultOpen: true,
    onConfirm: async () => undefined,
    onReset: () => undefined,
  },
};

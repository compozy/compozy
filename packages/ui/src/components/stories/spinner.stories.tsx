import type { Meta, StoryObj } from "@storybook/react-vite";

import { Spinner } from "../spinner";

const meta: Meta<typeof Spinner> = {
  title: "components/ui/Spinner",
  component: Spinner,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: 'Loading indicator based on lucide\'s Loader2 icon with role="status" built in.',
      },
    },
  },
  decorators: [
    Story => (
      <div className="bg-background p-4 text-foreground">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
};

export const Sizes: Story = {
  args: {},
  render: () => (
    <div className="flex items-center gap-3 text-muted-foreground">
      <Spinner className="size-3" />
      <Spinner className="size-4" />
      <Spinner className="size-6" />
      <Spinner className="size-8" />
    </div>
  ),
};

import type { Meta, StoryObj } from "@storybook/react-vite";

import { Skeleton, SkeletonRows } from "../skeleton";

const meta: Meta<typeof Skeleton> = {
  title: "components/ui/Skeleton",
  component: Skeleton,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Animated placeholder block used while async content is loading.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[360px] bg-background p-4 text-foreground">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { className: "h-5 w-48" },
};

export const ListRow: Story = {
  args: {},
  render: () => (
    <div className="flex items-center gap-3">
      <Skeleton className="size-10 rounded-full" />
      <div className="flex flex-col gap-2">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-3 w-24" />
      </div>
    </div>
  ),
};

export const Rows: Story = {
  args: {},
  render: () => (
    <SkeletonRows count={4} rowClassName="border-b border-border px-4 py-3">
      <Skeleton className="h-3.5 w-2/3" />
      <Skeleton className="h-3 w-full" />
      <Skeleton className="h-3 w-3/4" />
    </SkeletonRows>
  ),
};

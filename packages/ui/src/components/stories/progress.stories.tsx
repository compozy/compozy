import type { Meta, StoryObj } from "@storybook/react-vite";

import { Progress, ProgressLabel, ProgressValue } from "../progress";

const meta: Meta<typeof Progress> = {
  title: "components/ui/Progress",
  component: Progress,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Determinate progress bar composed with label, value, and the primary track/indicator.",
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
  args: { value: 42 },
  render: args => (
    <Progress {...args}>
      <ProgressLabel>Uploading dataset</ProgressLabel>
      <ProgressValue />
    </Progress>
  ),
};

export const Complete: Story = {
  args: { value: 100 },
  render: args => (
    <Progress {...args}>
      <ProgressLabel>Sync finished</ProgressLabel>
      <ProgressValue />
    </Progress>
  ),
};

export const Indeterminate: Story = {
  args: { value: null },
  render: args => (
    <Progress {...args}>
      <ProgressLabel>Resolving agents</ProgressLabel>
    </Progress>
  ),
};

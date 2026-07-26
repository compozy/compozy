import type { Meta, StoryObj } from "@storybook/react-vite";

import { Kbd, KbdGroup } from "../kbd";

const meta: Meta<typeof Kbd> = {
  title: "components/ui/Kbd",
  component: Kbd,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Inline keyboard key indicator. Compose multiple keys with KbdGroup.",
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
  args: { children: "⌘K" },
};

export const Group: Story = {
  args: {},
  render: () => (
    <KbdGroup>
      <Kbd>⌘</Kbd>
      <Kbd>Shift</Kbd>
      <Kbd>P</Kbd>
    </KbdGroup>
  ),
};

import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "../button";

const meta: Meta<typeof Button> = {
  title: "ui/Button",
  component: Button,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Primary operator action button used across the daemon web UI.",
      },
    },
  },
  tags: ["autodocs"],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default primary action for shell-level commands like sync or dispatch.
 */
export const Primary: Story = {
  args: {
    children: "Sync all workflows",
    size: "md",
    variant: "primary",
  },
};

/**
 * Compact secondary action used in dense route and card layouts.
 */
export const SecondarySmall: Story = {
  args: {
    children: "Archive",
    size: "sm",
    variant: "secondary",
  },
};

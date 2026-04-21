import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "../button";
import { SectionHeading } from "../section-heading";

const meta: Meta<typeof SectionHeading> = {
  title: "ui/SectionHeading",
  component: SectionHeading,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Page-level heading block used by dashboard, workflow, and run surfaces.",
      },
    },
  },
  tags: ["autodocs"],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Full heading with description and trailing action.
 */
export const Full: Story = {
  args: {
    eyebrow: "Overview",
    title: "Operator dashboard",
    description: "Inspect queue health, runs, and review pressure from one workspace.",
    actions: <Button size="sm">Sync all</Button>,
  },
};

/**
 * Minimal heading when a route only needs the title.
 */
export const Minimal: Story = {
  args: {
    title: "Workflow memory",
  },
};

import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Badge } from "./badge";

const meta = {
  title: "UI/Badge",
  component: Badge,
  parameters: {
    layout: "centered",
  },
  tags: ["autodocs"],
  argTypes: {
    variant: {
      control: "select",
      options: ["default", "secondary", "destructive", "outline", "success", "warning", "info"],
      description: "Visual variant of the badge",
    },
    size: {
      control: "select",
      options: ["default", "sm", "lg"],
      description: "Size variant of the badge",
    },
    children: {
      control: "text",
      description: "Badge content",
    },
    className: {
      control: "text",
      description: "Additional CSS classes to apply",
    },
  },
} satisfies Meta<typeof Badge>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    children: "Badge",
    variant: "default",
    size: "default",
  },
};

export const Secondary: Story = {
  args: {
    children: "Secondary",
    variant: "secondary",
    size: "default",
  },
};

export const Destructive: Story = {
  args: {
    children: "Error",
    variant: "destructive",
    size: "default",
  },
};

export const Outline: Story = {
  args: {
    children: "Outline",
    variant: "outline",
    size: "default",
  },
};

export const Success: Story = {
  args: {
    children: "Success",
    variant: "success",
    size: "default",
  },
};

export const Warning: Story = {
  args: {
    children: "Warning",
    variant: "warning",
    size: "default",
  },
};

export const Info: Story = {
  args: {
    children: "Info",
    variant: "info",
    size: "default",
  },
};

export const SizeVariants: Story = {
  render: () => (
    <div className="flex items-center gap-4">
      <Badge size="sm">Small</Badge>
      <Badge size="default">Default</Badge>
      <Badge size="lg">Large</Badge>
    </div>
  ),
};

export const AllVariants: Story = {
  render: () => (
    <div className="flex flex-wrap gap-2">
      <Badge variant="default">Default</Badge>
      <Badge variant="secondary">Secondary</Badge>
      <Badge variant="destructive">Destructive</Badge>
      <Badge variant="outline">Outline</Badge>
      <Badge variant="success">Success</Badge>
      <Badge variant="warning">Warning</Badge>
      <Badge variant="info">Info</Badge>
    </div>
  ),
};

export const StatusBadges: Story = {
  render: () => (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium">Status:</span>
        <Badge variant="success">Active</Badge>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium">Priority:</span>
        <Badge variant="destructive">High</Badge>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium">Environment:</span>
        <Badge variant="outline">Production</Badge>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium">Version:</span>
        <Badge variant="secondary">v2.1.0</Badge>
      </div>
    </div>
  ),
};

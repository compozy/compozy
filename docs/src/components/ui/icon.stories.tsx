import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Icon } from "./icon";

const meta = {
  title: "UI/Icon",
  component: Icon,
  parameters: {
    layout: "centered",
  },
  tags: ["autodocs"],
  argTypes: {
    name: {
      control: "text",
      description: "Icon name (PascalCase, camelCase, or kebab-case)",
    },
    size: {
      control: { type: "range", min: 12, max: 64, step: 4 },
      description: "Icon size in pixels",
    },
    strokeWidth: {
      control: { type: "range", min: 0.5, max: 4, step: 0.5 },
      description: "Stroke width",
    },
    className: {
      control: "text",
      description: "Additional CSS classes to apply",
    },
  },
} satisfies Meta<typeof Icon>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    name: "AlertTriangle",
    size: 24,
    strokeWidth: 2,
  },
};

export const Home: Story = {
  args: {
    name: "Home",
    size: 24,
    strokeWidth: 2,
  },
};

export const Settings: Story = {
  args: {
    name: "Settings",
    size: 24,
    strokeWidth: 2,
  },
};

export const User: Story = {
  args: {
    name: "User",
    size: 24,
    strokeWidth: 2,
  },
};

export const Mail: Story = {
  args: {
    name: "Mail",
    size: 24,
    strokeWidth: 2,
  },
};

export const Search: Story = {
  args: {
    name: "Search",
    size: 24,
    strokeWidth: 2,
  },
};

export const Heart: Story = {
  args: {
    name: "Heart",
    size: 24,
    strokeWidth: 2,
  },
};

export const Star: Story = {
  args: {
    name: "Star",
    size: 24,
    strokeWidth: 2,
  },
};

export const Sizes: Story = {
  args: { name: "AlertTriangle" },
  render: () => (
    <div className="flex items-center gap-4">
      <Icon name="AlertTriangle" size={16} />
      <Icon name="AlertTriangle" size={20} />
      <Icon name="AlertTriangle" size={24} />
      <Icon name="AlertTriangle" size={32} />
      <Icon name="AlertTriangle" size={48} />
    </div>
  ),
};

export const StrokeWidths: Story = {
  args: { name: "Heart" },
  render: () => (
    <div className="flex items-center gap-4">
      <Icon name="Heart" strokeWidth={0.5} />
      <Icon name="Heart" strokeWidth={1} />
      <Icon name="Heart" strokeWidth={2} />
      <Icon name="Heart" strokeWidth={3} />
      <Icon name="Heart" strokeWidth={4} />
    </div>
  ),
};

export const CommonIcons: Story = {
  args: { name: "Home" },
  render: () => (
    <div className="grid grid-cols-6 gap-4">
      <div className="flex flex-col items-center gap-2">
        <Icon name="Home" />
        <span className="text-sm">Home</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="Settings" />
        <span className="text-sm">Settings</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="User" />
        <span className="text-sm">User</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="Mail" />
        <span className="text-sm">Mail</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="Search" />
        <span className="text-sm">Search</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="Heart" />
        <span className="text-sm">Heart</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="Star" />
        <span className="text-sm">Star</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="AlertTriangle" />
        <span className="text-sm">Alert</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="CheckCircle" />
        <span className="text-sm">Check</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="X" />
        <span className="text-sm">Close</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="Plus" />
        <span className="text-sm">Add</span>
      </div>
      <div className="flex flex-col items-center gap-2">
        <Icon name="Minus" />
        <span className="text-sm">Remove</span>
      </div>
    </div>
  ),
};

export const NamingConventions: Story = {
  args: { name: "AlertTriangle" },
  render: () => (
    <div className="space-y-4">
      <div className="text-sm text-muted-foreground">
        The Icon component supports multiple naming conventions:
      </div>
      <div className="grid grid-cols-3 gap-4">
        <div className="flex flex-col items-center gap-2">
          <Icon name="AlertTriangle" />
          <span className="text-sm">PascalCase</span>
          <code className="text-xs bg-gray-100 px-2 py-1 rounded">AlertTriangle</code>
        </div>
        <div className="flex flex-col items-center gap-2">
          <Icon name="alertTriangle" />
          <span className="text-sm">camelCase</span>
          <code className="text-xs bg-gray-100 px-2 py-1 rounded">alertTriangle</code>
        </div>
        <div className="flex flex-col items-center gap-2">
          <Icon name="alert-triangle" />
          <span className="text-sm">kebab-case</span>
          <code className="text-xs bg-gray-100 px-2 py-1 rounded">alert-triangle</code>
        </div>
      </div>
    </div>
  ),
};

export const WithColors: Story = {
  args: { name: "Heart" },
  render: () => (
    <div className="flex items-center gap-4">
      <Icon name="Heart" className="text-red-500" />
      <Icon name="Star" className="text-yellow-500" />
      <Icon name="CheckCircle" className="text-green-500" />
      <Icon name="AlertTriangle" className="text-orange-500" />
      <Icon name="X" className="text-red-500" />
    </div>
  ),
};

export const Fallback: Story = {
  args: {
    name: "NonExistentIcon",
    size: 24,
    strokeWidth: 2,
  },
  parameters: {
    docs: {
      description: {
        story: "Shows the fallback icon when the specified icon is not found in lucide-static.",
      },
    },
  },
};

import type { Meta, StoryObj } from "@storybook/react";
import { ScrollArea } from "./scroll-area";

const meta: Meta<typeof ScrollArea> = {
  title: "UI/ScrollArea",
  component: ScrollArea,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component: "A scrollable area component based on Radix UI ScrollArea primitive.",
      },
    },
  },
  tags: ["autodocs"],
  decorators: [
    Story => (
      <div style={{ maxWidth: 400 }}>
        <Story />
      </div>
    ),
  ],
  argTypes: {
    className: {
      control: { type: "text" },
      description: "Additional CSS classes",
    },
    viewportClassName: {
      control: { type: "text" },
      description: "Additional CSS classes for the viewport",
    },
  },
};

export default meta;
type Story = StoryObj<typeof ScrollArea>;

export const Default: Story = {
  render: args => (
    <ScrollArea className="h-[200px] w-full border border-border rounded-md p-4" {...args}>
      <div className="space-y-4">
        {Array.from({ length: 20 }, (_, i) => (
          <div key={i} className="p-3 bg-muted rounded border">
            <h4 className="font-medium">Item {i + 1}</h4>
            <p className="text-sm text-muted-foreground">
              This is item {i + 1} content. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
            </p>
          </div>
        ))}
      </div>
    </ScrollArea>
  ),
};

export const LargeContent: Story = {
  render: args => (
    <ScrollArea className="h-[300px] w-full border border-border rounded-md p-4" {...args}>
      <div className="space-y-4">
        {Array.from({ length: 50 }, (_, i) => (
          <div key={i} className="p-4 bg-card border border-border rounded">
            <h3 className="font-semibold text-lg">Section {i + 1}</h3>
            <p className="text-sm text-muted-foreground mt-2">
              This is a longer content section {i + 1}. Lorem ipsum dolor sit amet, consectetur
              adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut
              enim ad minim veniam, quis nostrud exercitation ullamco laboris.
            </p>
            <div className="mt-3 flex gap-2">
              <span className="px-2 py-1 text-xs bg-primary/10 text-primary rounded">
                Tag {i + 1}
              </span>
              <span className="px-2 py-1 text-xs bg-secondary text-secondary-foreground rounded">
                Category
              </span>
            </div>
          </div>
        ))}
      </div>
    </ScrollArea>
  ),
};

export const HorizontalScroll: Story = {
  render: args => (
    <ScrollArea className="h-[150px] w-full border border-border rounded-md p-4" {...args}>
      <div className="flex gap-4" style={{ width: "150%" }}>
        {Array.from({ length: 10 }, (_, i) => (
          <div key={i} className="flex-shrink-0 w-48 p-4 bg-muted rounded border">
            <h4 className="font-medium">Card {i + 1}</h4>
            <p className="text-sm text-muted-foreground mt-2">
              This is a horizontal scrolling card with some content that extends beyond the
              viewport.
            </p>
          </div>
        ))}
      </div>
    </ScrollArea>
  ),
};

export const MinimalHeight: Story = {
  render: args => (
    <ScrollArea className="h-[100px] w-full border border-border rounded-md" {...args}>
      <div className="p-4 space-y-2">
        {Array.from({ length: 15 }, (_, i) => (
          <div key={i} className="py-2 px-3 bg-accent rounded text-sm">
            Short item {i + 1} - This should scroll smoothly
          </div>
        ))}
      </div>
    </ScrollArea>
  ),
};

export const WithCustomViewport: Story = {
  render: args => (
    <ScrollArea
      className="h-[250px] w-full border border-border rounded-md"
      viewportClassName="p-6"
      {...args}
    >
      <div className="space-y-4">
        <h2 className="text-xl font-bold">Custom Viewport Padding</h2>
        {Array.from({ length: 25 }, (_, i) => (
          <div key={i} className="p-3 bg-muted/50 rounded border border-dashed">
            <p className="text-sm">
              List item {i + 1} with custom viewport padding applied via viewportClassName prop.
              This demonstrates how the viewport can be customized independently.
            </p>
          </div>
        ))}
      </div>
    </ScrollArea>
  ),
};

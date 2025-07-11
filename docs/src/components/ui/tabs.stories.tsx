import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Tab, Tabs } from "./tabs";

const meta: Meta<typeof Tabs> = {
  title: "UI/Tabs",
  component: Tabs,
  parameters: {
    layout: "padded",
  },
  tags: ["autodocs"],
  argTypes: {
    items: {
      control: "multi-select",
      description: "Array of tab labels",
    },
    defaultValue: {
      control: "text",
      description: "Default active tab",
    },
    updateAnchor: {
      control: "boolean",
      description: "Update URL anchor when tab changes",
    },
    className: {
      control: "text",
      description: "Additional CSS classes",
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    items: ["Javascript", "Rust", "C++"],
  },
  render: args => (
    <Tabs {...args}>
      <Tab>
        <div className="p-4">
          <h3 className="text-lg font-semibold mb-2">Javascript</h3>
          <p>Javascript is a versatile programming language used for web development.</p>
        </div>
      </Tab>
      <Tab>
        <div className="p-4">
          <h3 className="text-lg font-semibold mb-2">Rust</h3>
          <p>Rust is a systems programming language focused on safety and performance.</p>
        </div>
      </Tab>
      <Tab>
        <div className="p-4">
          <h3 className="text-lg font-semibold mb-2">C++</h3>
          <p>C++ is a powerful language used for system programming and game development.</p>
        </div>
      </Tab>
    </Tabs>
  ),
};

export const SimpleTest: Story = {
  args: {
    items: ["Tab 1", "Tab 2"],
  },
  render: args => (
    <Tabs {...args}>
      <Tab>
        <p>Content for Tab 1</p>
      </Tab>
      <Tab>
        <p>Content for Tab 2</p>
      </Tab>
    </Tabs>
  ),
};

export const WithUpdateAnchor: Story = {
  args: {
    items: ["Overview", "Installation", "Usage"],
    updateAnchor: true,
  },
  render: args => (
    <Tabs {...args}>
      <Tab>
        <div className="p-4">
          <h3 className="text-lg font-semibold mb-2">Overview</h3>
          <p>This tab updates the URL anchor when selected. Check your browser's address bar!</p>
        </div>
      </Tab>
      <Tab>
        <div className="p-4">
          <h3 className="text-lg font-semibold mb-2">Installation</h3>
          <pre className="bg-muted p-3 rounded-md mt-2">
            <code>npm install @compozy/ui</code>
          </pre>
        </div>
      </Tab>
      <Tab>
        <div className="p-4">
          <h3 className="text-lg font-semibold mb-2">Usage</h3>
          <pre className="bg-muted p-3 rounded-md mt-2">
            <code>{`import { Tabs, Tab } from '@compozy/ui';`}</code>
          </pre>
        </div>
      </Tab>
    </Tabs>
  ),
};

export const WithDefaultValue: Story = {
  args: {
    items: ["First", "Second", "Third"],
    defaultValue: "Second",
  },
  render: args => (
    <Tabs {...args}>
      <Tab>
        <div className="p-4">
          <p>This is the first tab content.</p>
        </div>
      </Tab>
      <Tab>
        <div className="p-4">
          <p>This tab is selected by default!</p>
        </div>
      </Tab>
      <Tab>
        <div className="p-4">
          <p>This is the third tab content.</p>
        </div>
      </Tab>
    </Tabs>
  ),
};

export const LongContent: Story = {
  args: {
    items: ["Short", "Medium", "Long"],
  },
  render: args => (
    <Tabs {...args}>
      <Tab>
        <div className="p-4">
          <p>Short content.</p>
        </div>
      </Tab>
      <Tab>
        <div className="p-4">
          <p>This tab contains a medium amount of content.</p>
          <p className="mt-2">
            It has multiple paragraphs to demonstrate how the tab content area adjusts to different
            content sizes.
          </p>
          <ul className="list-disc list-inside mt-2">
            <li>First item</li>
            <li>Second item</li>
            <li>Third item</li>
          </ul>
        </div>
      </Tab>
      <Tab>
        <div className="p-4 space-y-4">
          <p>
            This tab contains a lot of content to demonstrate how tabs handle longer content areas.
          </p>
          <p>
            Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor
            incididunt ut labore et dolore magna aliqua.
          </p>
          <p>
            Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea
            commodo consequat.
          </p>
          <p>
            Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat
            nulla pariatur.
          </p>
          <p>
            Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt
            mollit anim id est laborum.
          </p>
        </div>
      </Tab>
    </Tabs>
  ),
};

export const ManyTabs: Story = {
  args: {
    items: ["Tab 1", "Tab 2", "Tab 3", "Tab 4", "Tab 5", "Tab 6"],
  },
  render: args => (
    <Tabs {...args}>
      {args.items.map((item, index) => (
        <Tab key={index}>
          <div className="p-4">
            <p>Content for {item}</p>
          </div>
        </Tab>
      ))}
    </Tabs>
  ),
};

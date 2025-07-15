import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { TreeViewElement } from "./magicui/file-tree";
import ProjectStructure from "./project-structure";

const meta = {
  title: "UI/ProjectStructure",
  component: ProjectStructure,
  parameters: {
    layout: "fullscreen",
  },
  tags: ["autodocs"],
  argTypes: {
    structure: {
      control: "object",
      description: "The project structure data as a TreeViewElement array",
    },
    title: {
      control: "text",
      description: "Title to display above the tree",
    },
    initialSelectedId: {
      control: "text",
      description: "Initially selected item ID",
    },
    initialExpandedItems: {
      control: "object",
      description: "Initially expanded items array",
    },
    className: {
      control: "text",
      description: "Additional CSS classes",
    },
  },
} satisfies Meta<typeof ProjectStructure>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    title: "Project Structure",
    initialSelectedId: "project",
    initialExpandedItems: ["compozy", "engine", "project", "pkg"],
  },
};

export const Compact: Story = {
  args: {
    title: "Compact View",
    initialSelectedId: "project",
    initialExpandedItems: ["compozy", "engine"],
  },
};

export const SimpleProject: Story = {
  args: {
    title: "Simple AI Project",
    initialSelectedId: "config",
    initialExpandedItems: ["my-project", "workflows", "agents"],
    structure: [
      {
        id: "my-project",
        name: "my-ai-project/",
        children: [
          { id: "config", name: "compozy.yaml" },
          { id: "env", name: ".env" },
          {
            id: "workflows",
            name: "workflows/",
            children: [
              { id: "analysis", name: "data-analysis.yaml" },
              { id: "content", name: "content-generation.yaml" },
            ],
          },
          {
            id: "agents",
            name: "agents/",
            children: [
              { id: "researcher", name: "researcher.yaml" },
              { id: "writer", name: "writer.yaml" },
            ],
          },
          { id: "tools", name: "tools.ts" },
          {
            id: "schemas",
            name: "schemas/",
            children: [{ id: "user-input", name: "user-input.yaml" }],
          },
          {
            id: "memory",
            name: "memory/",
            children: [{ id: "conversation", name: "conversation.yaml" }],
          },
        ],
      },
    ] as TreeViewElement[],
  },
};

export const ToolsFocused: Story = {
  args: {
    title: "Tools Directory Structure",
    initialSelectedId: "tools",
    initialExpandedItems: ["project", "tools", "scripts"],
    structure: [
      {
        id: "project",
        name: "project/",
        children: [
          {
            id: "tools",
            name: "tools/",
            children: [
              { id: "weather", name: "weather.ts" },
              { id: "database", name: "database.ts" },
              { id: "email", name: "email.ts" },
              { id: "file-ops", name: "file-operations.ts" },
            ],
          },
          {
            id: "scripts",
            name: "scripts/",
            children: [
              { id: "setup", name: "setup.sh" },
              { id: "deploy", name: "deploy.sh" },
              { id: "test", name: "test.sh" },
            ],
          },
          { id: "config", name: "compozy.yaml" },
          { id: "package", name: "package.json" },
        ],
      },
    ] as TreeViewElement[],
  },
};

export const WorkflowStructure: Story = {
  args: {
    title: "Multi-Workflow Project",
    initialSelectedId: "customer-support",
    initialExpandedItems: ["enterprise-project", "workflows", "agents"],
    structure: [
      {
        id: "enterprise-project",
        name: "enterprise-ai-system/",
        children: [
          { id: "config", name: "compozy.yaml" },
          { id: "env", name: ".env" },
          {
            id: "workflows",
            name: "workflows/",
            children: [
              { id: "customer-support", name: "customer-support.yaml" },
              { id: "data-pipeline", name: "data-pipeline.yaml" },
              { id: "content-gen", name: "content-generation.yaml" },
            ],
          },
          {
            id: "agents",
            name: "agents/",
            children: [
              { id: "support-agent", name: "support-agent.yaml" },
              { id: "analyst", name: "data-analyst.yaml" },
              { id: "writer", name: "content-writer.yaml" },
            ],
          },
          { id: "tools", name: "tools.ts" },
          { id: "readme", name: "README.md" },
        ],
      },
    ] as TreeViewElement[],
  },
};

export const MinimalStructure: Story = {
  args: {
    title: "Minimal Project",
    initialSelectedId: "workflow",
    initialExpandedItems: ["minimal-project"],
    structure: [
      {
        id: "minimal-project",
        name: "minimal-ai-project/",
        children: [
          { id: "config", name: "compozy.yaml" },
          { id: "workflow", name: "workflow.yaml" },
          { id: "tools", name: "tools.ts" },
          { id: "env", name: ".env" },
        ],
      },
    ] as TreeViewElement[],
  },
};

export const WithCustomClass: Story = {
  args: {
    title: "Custom Styled",
    className: "shadow-lg border-2 border-blue-200",
    initialSelectedId: "config",
    initialExpandedItems: ["styled-project"],
    structure: [
      {
        id: "styled-project",
        name: "styled-project/",
        children: [
          { id: "config", name: "compozy.yaml" },
          { id: "workflow", name: "workflow.yaml" },
          { id: "tools", name: "tools.ts" },
        ],
      },
    ] as TreeViewElement[],
  },
};

export const AllVariants: Story = {
  render: () => (
    <div className="space-y-8 p-4">
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ProjectStructure
          title="Default Compozy Structure"
          initialSelectedId="project"
          initialExpandedItems={["compozy", "engine"]}
        />
        <ProjectStructure
          title="Simple Project"
          initialSelectedId="config"
          initialExpandedItems={["simple-project"]}
          structure={[
            {
              id: "simple-project",
              name: "simple-project/",
              children: [
                { id: "config", name: "compozy.yaml" },
                { id: "workflow", name: "workflow.yaml" },
                { id: "tools", name: "tools.ts" },
                { id: "env", name: ".env" },
              ],
            },
          ]}
        />
      </div>
      <ProjectStructure
        title="Tools Focus"
        initialSelectedId="tools"
        initialExpandedItems={["project", "tools"]}
        structure={[
          {
            id: "project",
            name: "project/",
            children: [
              {
                id: "tools",
                name: "tools/",
                children: [
                  { id: "weather", name: "weather.ts" },
                  { id: "database", name: "database.ts" },
                  { id: "email", name: "email.ts" },
                ],
              },
              { id: "config", name: "compozy.yaml" },
            ],
          },
        ]}
      />
    </div>
  ),
};

import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { BookOpen, Code, Rocket, Settings, Users, Zap } from "lucide-react";
import { ReferenceCard, ReferenceCardList } from "./reference-card";

const meta: Meta<typeof ReferenceCard> = {
  title: "UI/ReferenceCard",
  component: ReferenceCard,
  parameters: {
    layout: "padded",
  },
  tags: ["autodocs"],
  argTypes: {
    title: {
      control: "text",
      description: "The title of the reference card",
    },
    description: {
      control: "text",
      description: "The description text below the title",
    },
    icon: {
      control: false,
      description: "Optional Lucide icon component",
    },
    href: {
      control: "text",
      description: "Optional URL to navigate to when clicked",
    },
    onClick: {
      action: "clicked",
      description: "Optional click handler function",
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
    title: "Getting Started",
    description: "Learn the basics of using our platform with this comprehensive guide.",
  },
};

export const WithIcon: Story = {
  args: {
    title: "API Documentation",
    description: "Complete reference for all available API endpoints and methods.",
    icon: BookOpen,
    href: "/api-docs",
  },
};

export const WithLink: Story = {
  args: {
    title: "API Documentation",
    description: "Complete reference for all available API endpoints and methods.",
    href: "/api-docs",
  },
};

export const WithClickHandler: Story = {
  args: {
    title: "Configuration Guide",
    description: "Step-by-step instructions for configuring your environment.",
    onClick: () => alert("Configuration guide clicked!"),
  },
};

export const LongContent: Story = {
  args: {
    title: "Advanced Workflow Orchestration Patterns",
    description:
      "Discover advanced patterns for building complex workflow orchestrations with multiple agents, parallel execution, and sophisticated error handling strategies.",
  },
};

export const ShortContent: Story = {
  args: {
    title: "Quick Start",
    description: "Get up and running in minutes.",
  },
};

// Card List Stories
const CardListMeta: Meta<typeof ReferenceCardList> = {
  title: "UI/ReferenceCardList",
  component: ReferenceCardList,
  parameters: {
    layout: "padded",
  },
  tags: ["autodocs"],
  argTypes: {
    className: {
      control: "text",
      description: "Additional CSS classes",
    },
  },
};

export const DefaultList: StoryObj<typeof ReferenceCardList> = {
  render: args => (
    <ReferenceCardList {...args}>
      <ReferenceCard
        title="Getting Started"
        description="Learn the basics of using our platform with this comprehensive guide."
        icon={Rocket}
        href="/getting-started"
      />
      <ReferenceCard
        title="API Documentation"
        description="Complete reference for all available API endpoints and methods."
        icon={Code}
        href="/api-docs"
      />
      <ReferenceCard
        title="Configuration Guide"
        description="Step-by-step instructions for configuring your environment."
        icon={Settings}
        href="/config"
      />
      <ReferenceCard
        title="Best Practices"
        description="Recommended patterns and practices for optimal performance."
        icon={Zap}
        href="/best-practices"
      />
    </ReferenceCardList>
  ),
  parameters: {
    docs: {
      description: {
        story: "A default list of reference cards in a single column layout.",
      },
    },
  },
};

export const ExtendedList: StoryObj<typeof ReferenceCardList> = {
  render: args => (
    <ReferenceCardList {...args}>
      <ReferenceCard
        title="Workflows"
        description="Create and manage complex workflow orchestrations."
        href="/workflows"
      />
      <ReferenceCard
        title="Agents"
        description="Configure AI agents with custom instructions and tools."
        href="/agents"
      />
      <ReferenceCard
        title="Tools"
        description="Build and integrate custom tools for your workflows."
        href="/tools"
      />
      <ReferenceCard
        title="MCP Integration"
        description="Connect external services using Model Context Protocol."
        href="/mcp"
      />
      <ReferenceCard
        title="Templates"
        description="Use template expressions for dynamic configurations."
        href="/templates"
      />
      <ReferenceCard
        title="Deployment"
        description="Deploy your workflows to production environments."
        href="/deployment"
      />
    </ReferenceCardList>
  ),
  parameters: {
    docs: {
      description: {
        story: "An extended list of reference cards showcasing multiple items.",
      },
    },
  },
};

export const CompactList: StoryObj<typeof ReferenceCardList> = {
  render: args => (
    <ReferenceCardList {...args}>
      <ReferenceCard
        title="Installation Guide"
        description="Complete installation instructions for all supported platforms and environments."
        href="/installation"
      />
      <ReferenceCard
        title="Configuration Reference"
        description="Detailed reference for all configuration options, environment variables, and settings."
        href="/configuration"
      />
      <ReferenceCard
        title="Troubleshooting"
        description="Common issues and their solutions, debugging tips, and support resources."
        href="/troubleshooting"
      />
    </ReferenceCardList>
  ),
  parameters: {
    docs: {
      description: {
        story: "A compact list with fewer items, ideal for focused navigation sections.",
      },
    },
  },
};

export const MixedInteractions: StoryObj<typeof ReferenceCardList> = {
  render: args => (
    <ReferenceCardList {...args}>
      <ReferenceCard
        title="External Link"
        description="This card navigates to an external URL."
        icon={BookOpen}
        href="https://example.com"
      />
      <ReferenceCard
        title="Click Handler"
        description="This card triggers a custom click handler."
        onClick={() => alert("Custom action triggered!")}
      />
      <ReferenceCard
        title="Static Card"
        description="This card has no interaction and is purely informational."
        icon={Users}
      />
      <ReferenceCard
        title="Internal Link"
        description="This card navigates to an internal route."
        href="/internal-page"
      />
    </ReferenceCardList>
  ),
  parameters: {
    docs: {
      description: {
        story:
          "Demonstrates different interaction types: external links, click handlers, static cards, and internal links.",
      },
    },
  },
};

// Export the card list meta separately
export { CardListMeta };

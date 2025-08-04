import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { CheckCircle2, Rocket, Star, Target, Zap } from "lucide-react";
import { List, ListItem } from "./list";

// Custom args that are not part of component props
type CustomArgs = {
  withTitle: boolean;
};

// Combine component props with custom args using intersection type
const meta: Meta<React.ComponentProps<typeof List> & CustomArgs> = {
  title: "UI/List",
  component: List,
  parameters: {
    layout: "centered",
  },
  decorators: [
    Story => (
      <div className="max-w-2xl mx-auto p-8">
        <Story />
      </div>
    ),
  ],
  argTypes: {
    withTitle: {
      control: { type: "boolean" },
      description: "Show or hide titles in list items",
      defaultValue: true,
      table: {
        category: "Story Controls",
        defaultValue: { summary: "true" },
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    withTitle: true,
  },
  render: ({ withTitle }) => (
    <List>
      <ListItem title={withTitle ? "First Item" : undefined}>
        This is the description for the first item in the list. It provides additional context and
        details.
      </ListItem>
      <ListItem title={withTitle ? "Second Item" : undefined}>
        Here's another item with its own description. Lists help organize information clearly.
      </ListItem>
      <ListItem title={withTitle ? "Third Item" : undefined}>
        The third item continues the pattern, each with its own numbered indicator.
      </ListItem>
    </List>
  ),
};

export const WithGlobalIcon: Story = {
  args: {
    withTitle: true,
  },
  render: ({ withTitle }) => (
    <List icon={CheckCircle2}>
      <ListItem title={withTitle ? "Completed Task" : undefined}>
        All items in this list use the same check icon instead of numbers.
      </ListItem>
      <ListItem title={withTitle ? "Another Completed Task" : undefined}>
        The icon is set at the List level and applied to all ListItems.
      </ListItem>
      <ListItem title={withTitle ? "Final Completed Task" : undefined}>
        This creates a consistent visual style across all items.
      </ListItem>
    </List>
  ),
};

export const MixedIcons: Story = {
  args: {
    withTitle: true,
  },
  render: ({ withTitle }) => (
    <List>
      <ListItem title={withTitle ? "Default Numbered Item" : undefined}>
        This item uses the default number indicator.
      </ListItem>
      <ListItem title={withTitle ? "Custom Icon Item" : undefined} icon={Star}>
        This item has its own custom icon that overrides the number.
      </ListItem>
      <ListItem title={withTitle ? "Another Custom Icon" : undefined} icon={Zap}>
        Each item can have a different icon if needed.
      </ListItem>
      <ListItem title={withTitle ? "Back to Numbers" : undefined}>
        Items without icons fall back to numbered indicators.
      </ListItem>
    </List>
  ),
};

export const WithoutTitles: Story = {
  args: {
    withTitle: false,
  },
  render: () => (
    <List>
      <ListItem>
        Sometimes you just need simple list items without titles. The content speaks for itself.
      </ListItem>
      <ListItem>
        This creates a cleaner, more minimal appearance when titles aren't necessary.
      </ListItem>
      <ListItem>Perfect for simple bullet points or brief statements.</ListItem>
    </List>
  ),
};

export const FeatureList: Story = {
  args: {
    withTitle: true,
  },
  render: ({ withTitle }) => (
    <List icon={Target}>
      <ListItem title={withTitle ? "Goal-Oriented Design" : undefined}>
        Our platform focuses on helping you achieve your objectives with clear, actionable steps.
      </ListItem>
      <ListItem title={withTitle ? "Performance Optimization" : undefined}>
        Built for speed and efficiency, ensuring your workflows run smoothly at scale.
      </ListItem>
      <ListItem title={withTitle ? "Seamless Integration" : undefined}>
        Connect with your existing tools and services through our comprehensive API.
      </ListItem>
      <ListItem title={withTitle ? "Advanced Analytics" : undefined}>
        Gain insights into your processes with detailed metrics and reporting.
      </ListItem>
    </List>
  ),
};

export const ProcessSteps: Story = {
  args: {
    withTitle: true,
  },
  render: ({ withTitle }) => (
    <List>
      <ListItem title={withTitle ? "Setup Your Environment" : undefined}>
        Begin by installing the necessary dependencies and configuring your development environment
        according to the documentation.
      </ListItem>
      <ListItem title={withTitle ? "Create Your First Workflow" : undefined}>
        Use our intuitive workflow builder to design your automation process step by step.
      </ListItem>
      <ListItem title={withTitle ? "Test and Iterate" : undefined}>
        Run your workflow in test mode to ensure everything works as expected before deployment.
      </ListItem>
      <ListItem title={withTitle ? "Deploy to Production" : undefined}>
        Once satisfied with your workflow, deploy it to production with a single command.
      </ListItem>
      <ListItem title={withTitle ? "Monitor and Scale" : undefined}>
        Track performance metrics and scale your workflows based on demand.
      </ListItem>
    </List>
  ),
};

export const WithLongContent: Story = {
  args: {
    withTitle: true,
  },
  render: ({ withTitle }) => (
    <List>
      <ListItem title={withTitle ? "Comprehensive Documentation" : undefined}>
        Our documentation covers every aspect of the platform, from basic concepts to advanced
        features. You'll find detailed guides, API references, code examples, and best practices to
        help you make the most of our tools. Whether you're just getting started or looking to
        optimize your existing workflows, our docs have you covered.
      </ListItem>
      <ListItem title={withTitle ? "Community Support" : undefined} icon={Rocket}>
        Join our vibrant community of developers and users who are building amazing things with our
        platform. Get help, share ideas, and collaborate on projects. Our community forums, Discord
        server, and GitHub discussions are great places to connect with others and get your
        questions answered.
      </ListItem>
    </List>
  ),
};

export const SmallSize: Story = {
  args: {
    withTitle: true,
  },
  render: ({ withTitle }) => (
    <List size="sm" icon={CheckCircle2}>
      <ListItem title={withTitle ? "Compact Design" : undefined}>
        Small size variant for space-constrained layouts.
      </ListItem>
      <ListItem title={withTitle ? "Reduced Spacing" : undefined}>
        Less padding and smaller text for dense information display.
      </ListItem>
      <ListItem title={withTitle ? "Perfect for Guidelines" : undefined}>
        Ideal for performance guidelines and quick reference lists.
      </ListItem>
    </List>
  ),
};

export const MediumSize: Story = {
  args: {
    withTitle: true,
  },
  render: ({ withTitle }) => (
    <List size="md" icon={Star}>
      <ListItem title={withTitle ? "Default Size" : undefined}>
        The standard medium size provides balanced spacing and readability.
      </ListItem>
      <ListItem title={withTitle ? "Comfortable Reading" : undefined}>
        Appropriate padding and text size for most use cases.
      </ListItem>
      <ListItem title={withTitle ? "Versatile Usage" : undefined}>
        Works well for feature lists, process steps, and general content.
      </ListItem>
    </List>
  ),
};

export const LargeSize: Story = {
  args: {
    withTitle: true,
  },
  render: ({ withTitle }) => (
    <List size="lg" icon={Zap}>
      <ListItem title={withTitle ? "Prominent Display" : undefined}>
        Large size variant for emphasis and visual impact.
      </ListItem>
      <ListItem title={withTitle ? "Increased Visibility" : undefined}>
        Larger icons, text, and spacing draw attention to important content.
      </ListItem>
      <ListItem title={withTitle ? "Hero Sections" : undefined}>
        Perfect for landing pages and feature highlights.
      </ListItem>
    </List>
  ),
};

export const SizeComparison: Story = {
  args: {
    withTitle: true,
  },
  render: ({ withTitle }) => (
    <div className="space-y-8">
      <div>
        <h3 className="text-lg font-semibold mb-4">Small Size</h3>
        <List size="sm" icon={CheckCircle2}>
          <ListItem title={withTitle ? "Memory Management" : undefined}>
            Use batch processing for datasets larger than 1000 items
          </ListItem>
          <ListItem title={withTitle ? "Performance Optimization" : undefined}>
            Monitor memory usage in production environments
          </ListItem>
          <ListItem title={withTitle ? "Scalability Patterns" : undefined}>
            Consider streaming patterns for extremely large datasets
          </ListItem>
        </List>
      </div>

      <div>
        <h3 className="text-lg font-semibold mb-4">Medium Size (Default)</h3>
        <List size="md" icon={CheckCircle2}>
          <ListItem title={withTitle ? "Memory Management" : undefined}>
            Use batch processing for datasets larger than 1000 items
          </ListItem>
          <ListItem title={withTitle ? "Performance Optimization" : undefined}>
            Monitor memory usage in production environments
          </ListItem>
          <ListItem title={withTitle ? "Scalability Patterns" : undefined}>
            Consider streaming patterns for extremely large datasets
          </ListItem>
        </List>
      </div>

      <div>
        <h3 className="text-lg font-semibold mb-4">Large Size</h3>
        <List size="lg" icon={CheckCircle2}>
          <ListItem title={withTitle ? "Memory Management" : undefined}>
            Use batch processing for datasets larger than 1000 items
          </ListItem>
          <ListItem title={withTitle ? "Performance Optimization" : undefined}>
            Monitor memory usage in production environments
          </ListItem>
          <ListItem title={withTitle ? "Scalability Patterns" : undefined}>
            Consider streaming patterns for extremely large datasets
          </ListItem>
        </List>
      </div>
    </div>
  ),
};

import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { CheckCircle2, Rocket, Star, Target, Zap } from "lucide-react";
import { List, ListItem } from "./list";

const meta = {
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
} satisfies Meta<typeof List>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    children: <div>Placeholder</div>,
  },
  render: () => (
    <List>
      <ListItem title="First Item">
        This is the description for the first item in the list. It provides additional context and
        details.
      </ListItem>
      <ListItem title="Second Item">
        Here's another item with its own description. Lists help organize information clearly.
      </ListItem>
      <ListItem title="Third Item">
        The third item continues the pattern, each with its own numbered indicator.
      </ListItem>
    </List>
  ),
};

export const WithGlobalIcon: Story = {
  args: {
    children: <div>Placeholder</div>,
  },
  render: () => (
    <List icon={CheckCircle2}>
      <ListItem title="Completed Task">
        All items in this list use the same check icon instead of numbers.
      </ListItem>
      <ListItem title="Another Completed Task">
        The icon is set at the List level and applied to all ListItems.
      </ListItem>
      <ListItem title="Final Completed Task">
        This creates a consistent visual style across all items.
      </ListItem>
    </List>
  ),
};

export const MixedIcons: Story = {
  args: {
    children: <div>Placeholder</div>,
  },
  render: () => (
    <List>
      <ListItem title="Default Numbered Item">
        This item uses the default number indicator.
      </ListItem>
      <ListItem title="Custom Icon Item" icon={Star}>
        This item has its own custom icon that overrides the number.
      </ListItem>
      <ListItem title="Another Custom Icon" icon={Zap}>
        Each item can have a different icon if needed.
      </ListItem>
      <ListItem title="Back to Numbers">
        Items without icons fall back to numbered indicators.
      </ListItem>
    </List>
  ),
};

export const WithoutTitles: Story = {
  args: {
    children: <div>Placeholder</div>,
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
    children: <div>Placeholder</div>,
  },
  render: () => (
    <List icon={Target}>
      <ListItem title="Goal-Oriented Design">
        Our platform focuses on helping you achieve your objectives with clear, actionable steps.
      </ListItem>
      <ListItem title="Performance Optimization">
        Built for speed and efficiency, ensuring your workflows run smoothly at scale.
      </ListItem>
      <ListItem title="Seamless Integration">
        Connect with your existing tools and services through our comprehensive API.
      </ListItem>
      <ListItem title="Advanced Analytics">
        Gain insights into your processes with detailed metrics and reporting.
      </ListItem>
    </List>
  ),
};

export const ProcessSteps: Story = {
  args: {
    children: <div>Placeholder</div>,
  },
  render: () => (
    <List>
      <ListItem title="Setup Your Environment">
        Begin by installing the necessary dependencies and configuring your development environment
        according to the documentation.
      </ListItem>
      <ListItem title="Create Your First Workflow">
        Use our intuitive workflow builder to design your automation process step by step.
      </ListItem>
      <ListItem title="Test and Iterate">
        Run your workflow in test mode to ensure everything works as expected before deployment.
      </ListItem>
      <ListItem title="Deploy to Production">
        Once satisfied with your workflow, deploy it to production with a single command.
      </ListItem>
      <ListItem title="Monitor and Scale">
        Track performance metrics and scale your workflows based on demand.
      </ListItem>
    </List>
  ),
};

export const WithLongContent: Story = {
  args: {
    children: <div>Placeholder</div>,
  },
  render: () => (
    <List>
      <ListItem title="Comprehensive Documentation">
        Our documentation covers every aspect of the platform, from basic concepts to advanced
        features. You'll find detailed guides, API references, code examples, and best practices to
        help you make the most of our tools. Whether you're just getting started or looking to
        optimize your existing workflows, our docs have you covered.
      </ListItem>
      <ListItem title="Community Support" icon={Rocket}>
        Join our vibrant community of developers and users who are building amazing things with our
        platform. Get help, share ideas, and collaborate on projects. Our community forums, Discord
        server, and GitHub discussions are great places to connect with others and get your
        questions answered.
      </ListItem>
    </List>
  ),
};

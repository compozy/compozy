import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "../button";

const meta: Meta<typeof Button> = {
  title: "components/ui/Button",
  component: Button,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Primary button primitive. Variants — default, primary (semantic alias for default), outline, secondary, ghost, destructive, success, link, neutral (warm `--btn-default-fill` glaze). Sizes — default/xs/sm/lg/cta/cta-lg + icon/icon-xs/icon-sm/icon-lg.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { children: "Action", variant: "default", size: "default" },
};

export const Primary: Story = {
  args: { children: "Primary action", variant: "primary", size: "default" },
  parameters: {
    docs: {
      description: {
        story:
          'Semantic alias for `default` — same accent CTA chrome, expresses caller intent ("primary action"). Pairs with `neutral` for the proposal\'s main CTA / fallback button duo.',
      },
    },
  },
};

export const Neutral: Story = {
  args: { children: "Neutral", variant: "neutral", size: "default" },
  parameters: {
    docs: {
      description: {
        story:
          "Filled secondary action with `--btn-default-fill` (0.04 glaze) → `--btn-default-hover` (0.07). No border. Use when `secondary` is too quiet and `outline` is too noisy.",
      },
    },
  },
};

export const Variants: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2 bg-background p-4 text-foreground">
      <Button variant="default">Default</Button>
      <Button variant="primary">Primary</Button>
      <Button variant="neutral">Neutral</Button>
      <Button variant="outline">Outline</Button>
      <Button variant="secondary">Secondary</Button>
      <Button variant="ghost">Ghost</Button>
      <Button variant="destructive">Destructive</Button>
      <Button variant="success">Success</Button>
      <Button variant="link">Link</Button>
    </div>
  ),
};

export const Sizes: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2 bg-background p-4 text-foreground">
      <Button size="xs">XS</Button>
      <Button size="sm">SM</Button>
      <Button size="default">Default</Button>
      <Button size="lg">LG</Button>
      <Button size="cta">CTA</Button>
      <Button size="cta-lg">CTA LG</Button>
    </div>
  ),
};

export const Disabled: Story = {
  args: { children: "Disabled", variant: "default", disabled: true },
};

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
          "Pill action primitive. Variants — default (the single inverted primary), primary (semantic alias for default), neutral/secondary (`surface-2` pill, no border), outline (hairline ghost), ghost, quiet (muted until hovered — toolbar Display/Filter pills), destructive (tinted), destructive-solid (irreversible Kill), success, link. Every size is a pill. `kbd` adds a decorative trailing key hint. Sizes — default/xs/sm/segment (30px, aligns with PillGroup md)/lg/cta/cta-lg + icon/icon-xs/icon-sm/icon-lg.",
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
          'Semantic alias for `default` — the same inverted pill (white on dark, near-black on light), expressing caller intent ("primary action"). One per surface; pairs with `neutral` for the main CTA / fallback duo.',
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
          "Secondary pill on `surface-2`; hover steps to `selected` and adds `shadow-card`. No border, and the label never dims.",
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
      <Button variant="quiet">Quiet</Button>
      <Button variant="destructive">Destructive</Button>
      <Button variant="destructive-solid">Destructive solid</Button>
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
      <Button size="segment">Segment</Button>
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

/** Tinted destructive beside the solid fill reserved for irreversible Kill. */
export const DestructivePair: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2 bg-background p-4 text-foreground">
      <Button variant="destructive">Cancel run</Button>
      <Button variant="destructive-solid">Kill run</Button>
    </div>
  ),
};

/** Approval footer grammar: a quiet secondary beside the single inverted primary with its ↵ hint. */
export const WithKbdHint: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap items-center gap-2 bg-background p-4 text-foreground">
      <Button variant="neutral">Deny</Button>
      <Button variant="primary" kbd="↵" aria-keyshortcuts="Enter">
        Allow once
      </Button>
    </div>
  ),
};

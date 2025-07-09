import type { Meta, StoryObj } from "@storybook/react";
import { Logo } from "./logo";

const meta: Meta<typeof Logo> = {
  title: "UI/Logo",
  component: Logo,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "The main Compozy logo that can display the symbol, lettering, or both. Available in different sizes for various contexts.",
      },
    },
  },
  argTypes: {
    variant: {
      control: { type: "select" },
      options: ["symbol", "lettering", "full"],
      description: "Controls which parts of the logo are displayed",
    },
    size: {
      control: { type: "select" },
      options: ["sm", "md", "lg"],
      description: "Size variant of the logo",
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    variant: "full",
    size: "md",
  },
};

export const Sizes: Story = {
  render: () => (
    <div className="flex flex-col gap-8">
      <div className="flex items-center gap-8">
        <Logo size="sm" variant="full" />
        <Logo size="md" variant="full" />
        <Logo size="lg" variant="full" />
      </div>
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: "All available size variants of the logo.",
      },
    },
  },
};

export const Variants: Story = {
  render: () => (
    <div className="flex flex-col gap-8">
      <div className="flex items-center gap-8">
        <Logo variant="symbol" size="md" />
        <Logo variant="lettering" size="md" />
        <Logo variant="full" size="md" />
      </div>
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: "Different display variants: symbol only, lettering only, or both.",
      },
    },
  },
};

export const AllCombinations: Story = {
  render: () => (
    <div className="flex flex-col gap-8">
      <div className="text-sm font-medium text-muted-foreground">Symbol Only</div>
      <div className="flex items-center gap-8">
        <Logo variant="symbol" size="sm" />
        <Logo variant="symbol" size="md" />
        <Logo variant="symbol" size="lg" />
      </div>

      <div className="text-sm font-medium text-muted-foreground mt-4">Lettering Only</div>
      <div className="flex items-center gap-8">
        <Logo variant="lettering" size="sm" />
        <Logo variant="lettering" size="md" />
        <Logo variant="lettering" size="lg" />
      </div>

      <div className="text-sm font-medium text-muted-foreground mt-4">Full Logo</div>
      <div className="flex items-center gap-8">
        <Logo variant="full" size="sm" />
        <Logo variant="full" size="md" />
        <Logo variant="full" size="lg" />
      </div>
    </div>
  ),
  parameters: {
    docs: {
      description: {
        story: "All possible combinations of variants and sizes.",
      },
    },
  },
};

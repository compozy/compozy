import type { Meta, StoryObj } from "@storybook/react-vite";

import { Logo, type LogoVariant } from "../logo";

const meta: Meta<typeof Logo> = {
  title: "components/custom/Logo",
  component: Logo,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Compozy brand mark. Use `logo` for full lockups, `symbol` for square app surfaces (including OS-shell menubar chrome at a smaller size class), and `lettering` only where the symbol is already present nearby.",
      },
    },
  },
  argTypes: {
    variant: {
      control: "select",
      options: ["logo", "symbol", "lettering"],
    },
    decorative: {
      control: "boolean",
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const VARIANTS: LogoVariant[] = ["logo", "symbol", "lettering"];

export const Default: Story = {
  args: {
    variant: "logo",
    label: "Compozy",
    className: "h-12 w-auto",
  },
};

export const Symbol: Story = {
  args: {
    variant: "symbol",
    label: "Compozy symbol",
    className: "size-16",
  },
};

export const Lettering: Story = {
  args: {
    variant: "lettering",
    label: "Compozy lettering",
    className: "h-12 w-auto",
  },
};

export const Variants: Story = {
  render: () => (
    <div className="grid min-w-[520px] gap-6 rounded-lg border border-line bg-canvas-soft p-6">
      {VARIANTS.map(variant => (
        <div key={variant} className="grid grid-cols-[7rem_1fr] items-center gap-6">
          <span className="font-mono text-eyebrow font-medium uppercase tracking-badge text-subtle">
            {variant}
          </span>
          <Logo
            variant={variant}
            label={`Compozy ${variant}`}
            className={variant === "symbol" ? "size-14" : "h-12 w-auto"}
          />
        </div>
      ))}
    </div>
  ),
};

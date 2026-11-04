import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";

import { Pill } from "@compozy/ui";
import { Eye, Lock, PencilLine, Zap } from "../../../icons";
import { RadioCardGroup, type RadioCardGroupOption } from "../radio-card-group";

type Level = "ask" | "read" | "edit" | "all";

const OPTIONS: RadioCardGroupOption<Level>[] = [
  {
    value: "ask",
    title: "Ask before every action",
    description: "Every action waits.",
    icon: Lock,
  },
  { value: "read", title: "Ask before changes", description: "Reading is automatic.", icon: Eye },
  {
    value: "edit",
    title: "Accept edits",
    description: "Riskier actions still ask.",
    icon: PencilLine,
    badge: (
      <Pill size="sm" tone="neutral">
        Default
      </Pill>
    ),
  },
  {
    value: "all",
    title: "Full access",
    description: "Acts without asking.",
    icon: Zap,
    disabled: true,
  },
];

const meta: Meta<typeof RadioCardGroup> = {
  title: "components/custom/RadioCardGroup",
  component: RadioCardGroup,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "`RadioCard`s in one radio group: one tab stop on the chosen card, Arrow keys move and select (skipping disabled cards, wrapping), Home and End jump to the ends. Layout comes from `className`.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[480px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

function Group({ className }: { className?: string }) {
  const [value, setValue] = useState<Level | null>("edit");
  return (
    <RadioCardGroup
      aria-label="Permission level"
      className={className}
      onValueChange={setValue}
      options={OPTIONS}
      value={value}
    />
  );
}

export const Column: Story = {
  render: () => <Group />,
};

export const Grid: Story = {
  render: () => <Group className="grid-cols-2" />,
};

export const KeyboardNavigation: Story = {
  tags: ["play-fn"],
  render: () => <Group />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("radio", { name: /Accept edits/ }));
    await userEvent.keyboard("{ArrowDown}");
    await expect(canvas.getByRole("radio", { name: /Ask before every action/ })).toHaveAttribute(
      "aria-checked",
      "true"
    );
  },
};

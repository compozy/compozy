import type { Meta, StoryObj } from "@storybook/react-vite";

import { Input } from "../input";

const meta: Meta<typeof Input> = {
  title: "components/ui/Input",
  component: Input,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Single-line text input with focus ring, disabled, and aria-invalid states.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[320px] bg-background p-4 text-foreground">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { placeholder: "session name" },
};

export const Disabled: Story = {
  args: { placeholder: "read only", disabled: true },
};

export const Invalid: Story = {
  args: { placeholder: "invalid", "aria-invalid": true, defaultValue: "bad value" },
};

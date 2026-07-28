import type { Meta, StoryObj } from "@storybook/react-vite";
import { BellIcon, BoldIcon, ItalicIcon, UnderlineIcon } from "lucide-react";
import { expect, userEvent, within } from "storybook/test";

import { Toggle } from "../toggle";

const meta: Meta<typeof Toggle> = {
  title: "components/ui/Toggle",
  component: Toggle,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Two-state button backed by Base UI. Compose with icons or text, use `ToggleGroup` for mutually exclusive or multi-select clusters.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <Toggle aria-label="Bold">
      <BoldIcon />
    </Toggle>
  ),
};

export const DefaultPressed: Story = {
  render: () => (
    <Toggle defaultPressed aria-label="Italic">
      <ItalicIcon />
    </Toggle>
  ),
};

export const Outline: Story = {
  render: () => (
    <Toggle variant="outline" aria-label="Underline">
      <UnderlineIcon />
    </Toggle>
  ),
};

export const WithLabel: Story = {
  render: () => (
    <Toggle variant="outline">
      <BellIcon />
      Notifications
    </Toggle>
  ),
};

export const TogglesAriaPressed: Story = {
  render: () => (
    <Toggle aria-label="Bold">
      <BoldIcon />
    </Toggle>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const toggle = canvas.getByRole("button", { name: "Bold" });
    await expect(toggle).toHaveAttribute("aria-pressed", "false");
    await userEvent.click(toggle);
    await expect(toggle).toHaveAttribute("aria-pressed", "true");
  },
};

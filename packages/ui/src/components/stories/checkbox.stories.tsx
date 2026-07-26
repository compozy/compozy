import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { Checkbox } from "../checkbox";
import { Label } from "../label";

const meta: Meta<typeof Checkbox> = {
  title: "components/ui/Checkbox",
  component: Checkbox,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Boolean selection control. The checked state fills with the accent; pair with a `Label` for form rows.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <div className="flex items-center gap-2">
      <Checkbox id="cb-default" />
      <Label htmlFor="cb-default">Notify on completion</Label>
    </div>
  ),
};

export const Checked: Story = {
  render: () => (
    <div className="flex items-center gap-2">
      <Checkbox id="cb-checked" defaultChecked />
      <Label htmlFor="cb-checked">Include archived items</Label>
    </div>
  ),
};

export const Disabled: Story = {
  render: () => (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <Checkbox id="cb-disabled-off" disabled />
        <Label htmlFor="cb-disabled-off">Managed by policy</Label>
      </div>
      <div className="flex items-center gap-2">
        <Checkbox id="cb-disabled-on" disabled defaultChecked />
        <Label htmlFor="cb-disabled-on">Managed by policy</Label>
      </div>
    </div>
  ),
};

export const TogglesOnClick: Story = {
  render: () => <Checkbox aria-label="toggle" />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const box = canvas.getByRole("checkbox", { name: "toggle" });
    await expect(box).toHaveAttribute("aria-checked", "false");
    await userEvent.click(box);
    await expect(box).toHaveAttribute("aria-checked", "true");
  },
};

import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { Label } from "../label";
import { Switch } from "../switch";

const meta: Meta<typeof Switch> = {
  title: "components/ui/Switch",
  component: Switch,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Binary on/off control. Pair with a `Label` for settings rows. Supports `sm` and `default` sizes plus disabled state.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <Switch id="switch-default" defaultChecked />
      <Label htmlFor="switch-default">Stream responses</Label>
    </div>
  ),
};

export const Small: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <Switch id="switch-sm" size="sm" />
      <Label htmlFor="switch-sm" className="text-sm">
        Compact mode
      </Label>
    </div>
  ),
};

export const Disabled: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <Switch id="switch-disabled" disabled defaultChecked />
      <Label htmlFor="switch-disabled">Workspace lock (set by admin)</Label>
    </div>
  ),
};

export const TogglesOnClick: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <Switch id="switch-play" aria-label="toggle" />
      <Label htmlFor="switch-play">Enable telemetry</Label>
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const toggle = canvas.getByRole("switch", { name: "toggle" });
    await expect(toggle).toHaveAttribute("aria-checked", "false");
    await userEvent.click(toggle);
    await expect(toggle).toHaveAttribute("aria-checked", "true");
  },
};

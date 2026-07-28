import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";
import { useState } from "react";

import { Slider } from "../slider";

const meta: Meta<typeof Slider> = {
  title: "components/ui/Slider",
  component: Slider,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "One bounded scalar, dragged or stepped with the keyboard. Used where a number input would ask someone to type a value they can only feel.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <div className="w-64">
      <Slider aria-label="History steps" defaultValue={50} max={500} min={1} />
    </div>
  ),
};

export const WithReadout: Story = {
  render: function WithReadoutStory() {
    const [value, setValue] = useState(50);
    return (
      <div className="flex w-64 items-center gap-3">
        <Slider
          aria-label="History steps"
          max={500}
          min={1}
          value={value}
          onValueChange={setValue}
        />
        <span className="w-14 shrink-0 font-mono text-small-body tabular-nums text-fg">
          {value} steps
        </span>
      </div>
    );
  },
};

export const Disabled: Story = {
  render: () => (
    <div className="w-64">
      <Slider aria-label="History steps" defaultValue={120} disabled max={500} min={1} />
    </div>
  ),
};

export const StepsWithKeyboard: Story = {
  tags: ["play-fn"],
  render: () => (
    <div className="w-64">
      <Slider aria-label="History steps" defaultValue={50} max={500} min={1} />
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const thumb = canvas.getByRole("slider", { name: "History steps" });
    thumb.focus();
    await userEvent.keyboard("{ArrowRight}");
    await expect(thumb).toHaveAttribute("aria-valuenow", "51");
  },
};

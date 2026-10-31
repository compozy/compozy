import type { Meta, StoryObj } from "@storybook/react-vite";
import * as React from "react";
import { expect, userEvent, within } from "storybook/test";

import { Calendar } from "../calendar";

const meta: Meta = {
  title: "components/ui/Calendar",
  component: Calendar,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Month grid on react-day-picker (shadcn base-nova recipe via ReUI) on Compozy tokens. Every DayPicker mode passes through; compose it in a Popover or `DateTimePicker` for form fields.",
      },
    },
  },
};

export default meta;
type Story = StoryObj;

const MONTH = new Date(2027, 2, 1);

function SingleCalendar() {
  const [selected, setSelected] = React.useState<Date | undefined>(new Date(2027, 2, 12));
  return (
    <Calendar
      className="rounded-lg border border-line bg-canvas-soft"
      defaultMonth={MONTH}
      mode="single"
      onSelect={setSelected}
      selected={selected}
    />
  );
}

export const Single: Story = {
  tags: ["play-fn"],
  render: () => <SingleCalendar />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const day = canvas.getByRole("button", { name: /March 18th, 2027/ });
    await userEvent.click(day);
    await expect(day).toHaveAttribute("data-selected-single", "true");
  },
};

export const DisabledPast: Story = {
  render: () => (
    <Calendar
      className="rounded-lg border border-line bg-canvas-soft"
      defaultMonth={MONTH}
      disabled={{ before: new Date(2027, 2, 15) }}
      mode="single"
    />
  ),
};

export const Range: Story = {
  render: () => (
    <Calendar
      className="rounded-lg border border-line bg-canvas-soft"
      defaultMonth={MONTH}
      mode="range"
      selected={{ from: new Date(2027, 2, 8), to: new Date(2027, 2, 12) }}
    />
  ),
};

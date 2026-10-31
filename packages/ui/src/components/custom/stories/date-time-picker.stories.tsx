import type { Meta, StoryObj } from "@storybook/react-vite";
import * as React from "react";
import { expect, screen, userEvent, within } from "storybook/test";

import { Field, FieldDescription, FieldLabel } from "../../field";
import { DateTimePicker, type DateTimePickerPreset } from "../date-time-picker";

const meta: Meta<typeof DateTimePicker> = {
  title: "components/custom/DateTimePicker",
  component: DateTimePicker,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Date and time in one popover: optional presets, a month `Calendar`, and a time list. `min` closes earlier days and times; the value stays clearable from the trigger row.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function at(daysFromNow: number, hours: number): Date {
  const date = new Date();
  date.setDate(date.getDate() + daysFromNow);
  date.setHours(hours, 0, 0, 0);
  return date;
}

const PRESETS: DateTimePickerPreset[] = [
  { label: "Tomorrow", value: () => at(1, 9) },
  { label: "In 3 days", value: () => at(3, 9) },
  { label: "Next week", value: () => at(7, 9) },
];

function PickerField({
  initial = null,
  presets,
}: {
  initial?: Date | null;
  presets?: DateTimePickerPreset[];
}) {
  const [value, setValue] = React.useState<Date | null>(initial);
  const [min] = React.useState(() => new Date());
  return (
    <Field className="w-80">
      <FieldLabel id="story-remind-label">Remind me on</FieldLabel>
      <DateTimePicker
        aria-labelledby="story-remind-label"
        min={min}
        onValueChange={setValue}
        presets={presets}
        value={value}
      />
      <FieldDescription>Optional.</FieldDescription>
    </Field>
  );
}

export const Empty: Story = {
  render: () => <PickerField presets={PRESETS} />,
};

export const WithValue: Story = {
  render: () => <PickerField initial={at(2, 14)} presets={PRESETS} />,
};

export const WithoutPresets: Story = {
  render: () => <PickerField initial={at(2, 14)} />,
};

export const PickPreset: Story = {
  tags: ["play-fn"],
  render: () => <PickerField presets={PRESETS} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: /Pick a date/ }));
    await userEvent.click(await screen.findByRole("button", { name: "Tomorrow" }));
    await expect(canvas.getByRole("button", { name: "Clear date" })).toBeInTheDocument();
  },
};

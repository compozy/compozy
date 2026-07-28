import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { Label } from "../label";
import { RadioGroup, RadioGroupItem } from "../radio-group";

const meta: Meta<typeof RadioGroup> = {
  title: "components/ui/RadioGroup",
  component: RadioGroup,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Single-choice control. The selected item fills with the accent; pair each item with a `Label`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const OPTIONS = [
  { value: "fast", label: "Fastest" },
  { value: "balanced", label: "Balanced" },
  { value: "thorough", label: "Most thorough" },
];

export const Default: Story = {
  render: () => (
    <RadioGroup defaultValue="balanced" aria-label="Routing mode">
      {OPTIONS.map(option => (
        <div key={option.value} className="flex items-center gap-2">
          <RadioGroupItem id={`rg-${option.value}`} value={option.value} />
          <Label htmlFor={`rg-${option.value}`}>{option.label}</Label>
        </div>
      ))}
    </RadioGroup>
  ),
};

export const Disabled: Story = {
  render: () => (
    <RadioGroup defaultValue="on" aria-label="Managed" disabled>
      <div className="flex items-center gap-2">
        <RadioGroupItem id="rg-on" value="on" />
        <Label htmlFor="rg-on">Enabled (set by admin)</Label>
      </div>
      <div className="flex items-center gap-2">
        <RadioGroupItem id="rg-off" value="off" />
        <Label htmlFor="rg-off">Disabled</Label>
      </div>
    </RadioGroup>
  ),
};

export const SelectsOnClick: Story = {
  render: () => (
    <RadioGroup aria-label="mode">
      <RadioGroupItem aria-label="fast" value="fast" />
      <RadioGroupItem aria-label="slow" value="slow" />
    </RadioGroup>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const fast = canvas.getByRole("radio", { name: "fast" });
    await expect(fast).toHaveAttribute("aria-checked", "false");
    await userEvent.click(fast);
    await expect(fast).toHaveAttribute("aria-checked", "true");
  },
};

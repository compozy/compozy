import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";

import { PanelSurface } from "@/storybook/story-layout";

import { SettingsChoiceGroup } from "../settings-choice-group";

const options = [
  { value: "ask", name: "Ask first", description: "Every action waits for approval." },
  { value: "read", name: "Allow reading", description: "Reads run without waiting." },
  { value: "all", name: "Allow everything", description: "Trusted agents act freely." },
] as const;

const meta = {
  title: "systems/settings/components/SettingsChoiceGroup",
  component: SettingsChoiceGroup,
  parameters: { layout: "centered" },
} satisfies Meta<typeof SettingsChoiceGroup>;

export default meta;
type Story = StoryObj<typeof meta>;

export const KeyboardNavigation: Story = {
  args: {
    ariaLabel: "Permission mode",
    onChange: () => undefined,
    options,
    value: "ask",
  },
  render: function Render() {
    const [value, setValue] = useState<(typeof options)[number]["value"]>("ask");
    return (
      <PanelSurface className="w-full max-w-3xl p-6">
        <SettingsChoiceGroup
          ariaLabel="Permission mode"
          onChange={setValue}
          options={options}
          value={value}
        />
      </PanelSurface>
    );
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const ask = canvas.getByRole("radio", { name: /Ask first/u });
    const read = canvas.getByRole("radio", { name: /Allow reading/u });
    ask.focus();
    await userEvent.keyboard("{ArrowRight}");
    await expect(read).toHaveFocus();
    await expect(read).toHaveAttribute("aria-checked", "true");
  },
};

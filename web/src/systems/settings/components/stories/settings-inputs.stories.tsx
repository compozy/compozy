import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { SettingsDecimalInput } from "../settings-decimal-input";
import { SettingsNumberInput } from "../settings-number-input";

function SettingsInputHarness() {
  const [whole, setWhole] = useState(3);
  const [decimal, setDecimal] = useState(0.75);
  const [wholeError, setWholeError] = useState<string | null>(null);
  const [decimalError, setDecimalError] = useState<string | null>(null);

  return (
    <div className="grid w-full max-w-lg gap-5 rounded-lg border border-line bg-canvas-soft p-5">
      <label className="grid gap-2">
        <span className="text-sm font-medium text-fg">Max retries</span>
        <SettingsNumberInput
          value={whole}
          min={1}
          onValueChange={setWhole}
          onValidityChange={setWholeError}
        />
        <span className="text-xs text-subtle">{wholeError ?? `Current value: ${whole}`}</span>
      </label>
      <label className="grid gap-2">
        <span className="text-sm font-medium text-fg">Temperature</span>
        <SettingsDecimalInput
          value={decimal}
          min={0}
          max={2}
          precision={2}
          onValueChange={setDecimal}
          onValidityChange={setDecimalError}
        />
        <span className="text-xs text-subtle">
          {decimalError ?? `Current value: ${decimal.toFixed(2)}`}
        </span>
      </label>
    </div>
  );
}

const meta: Meta<typeof SettingsNumberInput> = {
  title: "systems/settings/components/SettingsInputs",
  component: SettingsNumberInput,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Numeric settings inputs with local validation for integer and decimal values.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Integer and decimal controls surface validation without committing invalid values.
 */
export const Default: Story = {
  args: {},
  render: () => <SettingsInputHarness />,
};

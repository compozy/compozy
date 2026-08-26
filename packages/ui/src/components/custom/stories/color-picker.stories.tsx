import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { ColorPicker } from "../color-picker";

const meta: Meta<typeof ColorPicker> = {
  title: "components/custom/ColorPicker",
  component: ColorPicker,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Saturation area plus hue slider for free color choice. Hex input, swatches, and validation stay with the composing surface.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-72">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

function Harness({ initial }: { initial: string }) {
  const [value, setValue] = useState(initial);
  return (
    <div className="flex flex-col gap-2">
      <ColorPicker value={value} onChange={setValue} />
      <span className="font-mono text-badge text-muted">{value}</span>
    </div>
  );
}

/** Dragging the area or hue slider emits normalized `#rrggbb` values. */
export const Default: Story = {
  args: { value: "#4ea7fc", onChange: () => {} },
  render: () => <Harness initial="#4ea7fc" />,
};

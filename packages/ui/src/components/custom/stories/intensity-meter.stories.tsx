import type { Meta, StoryObj } from "@storybook/react-vite";

import { IntensityMeter } from "../intensity-meter";

const meta: Meta<typeof IntensityMeter> = {
  title: "components/custom/IntensityMeter",
  component: IntensityMeter,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Seven-bar discrete intensity meter. `position` fills bars with the accent; `hollow` is the faint default state.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Positions: Story = {
  render: () => (
    <div className="flex items-end gap-4">
      {[0, 1, 3, 5, 7].map(position => (
        <div key={position} className="flex flex-col items-center gap-2">
          <IntensityMeter position={position} />
          <span className="font-mono text-mono-id text-muted">{position}</span>
        </div>
      ))}
    </div>
  ),
};

export const Hollow: Story = {
  render: () => <IntensityMeter position={4} hollow />,
};

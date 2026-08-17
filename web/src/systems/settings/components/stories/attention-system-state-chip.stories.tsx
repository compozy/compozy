import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { AttentionSystemStateChip } from "../attention-system-state-chip";

const meta: Meta<typeof AttentionSystemStateChip> = {
  title: "systems/settings/components/AttentionSystemStateChip",
  component: AttentionSystemStateChip,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const DeliveryStates: Story = {
  args: { state: "granted" },
  render: () => (
    <CenteredSurface>
      <div className="flex items-center gap-5 rounded-lg border border-line bg-canvas-soft px-5 py-4">
        <AttentionSystemStateChip state="granted" />
        <AttentionSystemStateChip state="denied" />
        <AttentionSystemStateChip state="unsupported" />
      </div>
    </CenteredSurface>
  ),
};

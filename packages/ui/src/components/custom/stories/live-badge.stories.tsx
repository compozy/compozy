import type { Meta, StoryObj } from "@storybook/react-vite";

import { LiveBadge } from "../live-badge";

const meta: Meta<typeof LiveBadge> = {
  title: "components/custom/LiveBadge",
  component: LiveBadge,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Single live-state vocabulary: pulsing signal dot + short label. One per surface, owned by the section that owns the stream.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default healthy stream indicator (success tone, pulsing).
 */
export const Default: Story = {
  args: {},
};

/**
 * Tones for degraded stream states; pulse disabled on the stale variant.
 */
export const Tones: Story = {
  args: {},
  render: () => (
    <div className="flex items-center gap-4">
      <LiveBadge />
      <LiveBadge label="Reconnecting" tone="warning" />
      <LiveBadge label="Stale" pulse={false} tone="neutral" />
    </div>
  ),
};

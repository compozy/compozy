import type { Meta, StoryObj } from "@storybook/react-vite";

import { usageTray } from "../../mocks";
import { ProviderUsageTrayMeter } from "../provider-usage-tray-meter";

const NOW = Date.now();

const meta: Meta<typeof ProviderUsageTrayMeter> = {
  component: ProviderUsageTrayMeter,
  title: "systems/provider-usage/components/ProviderUsageTrayMeter",
  args: { tray: usageTray({}, NOW) },
  decorators: [
    Story => (
      <div className="flex min-h-24 items-start justify-end gap-0.5 bg-rail p-2 text-fg">
        <Story />
      </div>
    ),
  ],
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** US-001.AC-1: 0–79 % reads neutral ink. */
export const Neutral: Story = {};

/** US-001.EC-1: no known window — the glyph alone; tooltip "Provider usage". */
export const NoMeter: Story = { args: { tray: null } };

/** US-001.AC-3: 80–99 % uses the warning tone. */
export const Warning: Story = {
  args: {
    tray: usageTray(
      {
        provider_id: "claude",
        window_id: "weekly_opus",
        label: "Claude · Weekly · Opus",
        used_percent: 88,
        severity: "warning",
      },
      NOW
    ),
  },
};

/** US-001.AC-3/EC-4: at or above 100 the meter is full, danger tone, "limit reached". */
export const Exhausted: Story = {
  args: { tray: usageTray({ used_percent: 112, severity: "exhausted" }, NOW) },
};

/** US-001.EC-3: stale data dims the bar and adds a ring marker; the label says how old. */
export const Stale: Story = {
  args: {
    tray: usageTray({ stale: true }, NOW),
    measuredAt: new Date(NOW - 25 * 60_000).toISOString(),
  },
};

/** The open state as the popover trigger renders it. */
export const Open: Story = {
  render: args => <ProviderUsageTrayMeter {...args} aria-expanded data-popup-open="" />,
};

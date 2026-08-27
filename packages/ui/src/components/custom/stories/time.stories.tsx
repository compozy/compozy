import type { Meta, StoryObj } from "@storybook/react-vite";

import { Time } from "../time";

const meta: Meta<typeof Time> = {
  title: "components/custom/Time",
  component: Time,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Relative (default), compact, or absolute timestamp. Renders `<time dateTime title>` with the alternate format in the title attribute. Refreshes every 30 s for relative and compact modes.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Relative — recent (30 s ago). */
export const RelativeJustNow: Story = {
  args: { iso: new Date(Date.now() - 5_000).toISOString() },
};

/** Relative — minutes ago. */
export const RelativeMinutes: Story = {
  args: { iso: new Date(Date.now() - 5 * 60_000).toISOString() },
};

/** Relative — hours ago. */
export const RelativeHours: Story = {
  args: { iso: new Date(Date.now() - 3 * 3_600_000).toISOString() },
};

/** Compact rail age — `5m`, no ago suffix. */
export const Compact: Story = {
  args: { iso: new Date(Date.now() - 5 * 60_000).toISOString(), mode: "compact" },
};

/** Absolute timestamp. */
export const Absolute: Story = {
  args: { iso: new Date(Date.now() - 5 * 60_000).toISOString(), mode: "absolute" },
};

/** Invalid ISO — renders the `—` sentinel. */
export const Invalid: Story = {
  args: { iso: "not-an-iso" },
};

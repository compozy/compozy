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
          "Relative (default) or absolute timestamp. Renders `<time dateTime title>` with the alternate format in the title attribute. Refreshes every 30 s for relative mode.",
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

/** Absolute timestamp. */
export const Absolute: Story = {
  args: { iso: new Date(Date.now() - 5 * 60_000).toISOString(), mode: "absolute" },
};

/** Invalid ISO — renders the `—` sentinel. */
export const Invalid: Story = {
  args: { iso: "not-an-iso" },
};

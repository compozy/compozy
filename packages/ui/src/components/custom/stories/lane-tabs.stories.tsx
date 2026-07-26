import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { LaneTabs } from "../lane-tabs";

const meta: Meta<typeof LaneTabs> = {
  title: "components/custom/LaneTabs",
  component: LaneTabs,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Dense lane tab strip — TechSpec composite. Bottom-rule on `--line`, active lane gets a 1.5px `--accent` underline anchored to the rule. Counts render in a flat `--canvas-soft` mono-badge. Arrow / Home / End keyboard navigation is wired natively.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[640px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const LANES = [
  { value: "all", label: "All", count: 124 },
  { value: "active", label: "Active", count: 8 },
  { value: "review", label: "Review", count: 3 },
  { value: "done", label: "Done", count: 113 },
] as const;

/**
 * Tasks-style lane row with counts.
 */
export const WithCounts: Story = {
  args: {},
  render: () => {
    type LaneValue = (typeof LANES)[number]["value"];
    const [value, setValue] = useState<LaneValue>("active");
    return (
      <LaneTabs<LaneValue>
        ariaLabel="Tasks lanes"
        items={LANES.map(l => ({ value: l.value, label: l.label, count: l.count }))}
        value={value}
        onChange={setValue}
      />
    );
  },
};

/**
 * Lanes without counts — confirms the count badge gracefully omits.
 */
export const NoCounts: Story = {
  args: {},
  render: () => {
    type Tab = "overview" | "runs" | "settings";
    const [value, setValue] = useState<Tab>("overview");
    return (
      <LaneTabs<Tab>
        ariaLabel="Detail panes"
        items={[
          { value: "overview", label: "Overview" },
          { value: "runs", label: "Runs" },
          { value: "settings", label: "Settings" },
        ]}
        value={value}
        onChange={setValue}
      />
    );
  },
};

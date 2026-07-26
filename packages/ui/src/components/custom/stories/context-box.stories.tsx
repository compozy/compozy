import type { Meta, StoryObj } from "@storybook/react-vite";

import { ContextBox } from "../context-box";

const meta: Meta<typeof ContextBox> = {
  title: "components/custom/ContextBox",
  component: ContextBox,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Two-column metadata grid with mono UPPERCASE labels and `--fg` values, on a flat `--canvas-soft` panel with 1px `--line` ring. Use for dense identifying context (session id, branch, region, owner, model) inside detail headers and inspectors.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[420px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Common operator context — five rows of identifying metadata.
 */
export const Default: Story = {
  args: {},
  render: () => (
    <ContextBox
      entries={[
        { label: "Session", value: "sess_5f3a91" },
        { label: "Workspace", value: "personal" },
        { label: "Branch", value: "redesign" },
        { label: "Provider", value: "anthropic" },
        { label: "Started", value: "04:21:15 UTC" },
      ]}
    />
  ),
};

/**
 * With an optional eyebrow title above the panel.
 */
export const WithTitle: Story = {
  args: {},
  render: () => (
    <ContextBox
      title="Run context"
      entries={[
        { label: "Run id", value: "run_2026-05-10T04:21" },
        { label: "Trigger", value: "manual" },
        { label: "Caller", value: "operator@local" },
      ]}
    />
  ),
};

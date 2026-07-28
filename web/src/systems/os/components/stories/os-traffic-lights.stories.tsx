import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { OsTrafficLights } from "../os-traffic-lights";

const meta: Meta<typeof OsTrafficLights> = {
  title: "systems/os/components/OsTrafficLights",
  component: OsTrafficLights,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "The three OS window controls (close / minimize / zoom). Neutral at rest; each shows its signal color on hover or keyboard focus. Buttons only when a callback is supplied.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Interactive — hover or focus a control to reveal its signal color.
 */
export const Interactive: Story = {
  args: { onSelect: fn() },
  render: args => (
    <div className="rounded-md border border-line bg-canvas p-4">
      <OsTrafficLights {...args} />
    </div>
  ),
};

/**
 * Presentation-only — no callback, so the controls render as inert chrome.
 */
export const PresentationOnly: Story = {
  args: {},
  render: args => (
    <div className="rounded-md border border-line bg-canvas p-4">
      <OsTrafficLights {...args} />
    </div>
  ),
};

/**
 * Compact inert chrome keeps the 15px glyphs visibly separated without
 * pretending the controls are interactive.
 */
export const CompactPresentationOnly: Story = {
  args: { compact: true },
  render: args => (
    <div className="rounded-md border border-line bg-canvas p-4">
      <OsTrafficLights {...args} />
    </div>
  ),
};

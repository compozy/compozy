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
          "Quiet window controls in the order minimize, zoom, close: subtle icons that wash to `surface-2` on hover. They trail the deck row, or the head when there is no deck. Buttons only when a callback is supplied.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Interactive — hover or focus a control to reveal its wash.
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
 * Zoomed — the zoom control reads as pressed and offers Restore.
 */
export const Zoomed: Story = {
  args: { onSelect: fn(), zoomed: true },
  render: args => (
    <div className="rounded-md border border-line bg-canvas p-4">
      <OsTrafficLights {...args} />
    </div>
  ),
};

/**
 * Compact inert chrome drops zoom and keeps 44px cells without pretending the
 * controls are interactive.
 */
export const CompactPresentationOnly: Story = {
  args: { compact: true },
  render: args => (
    <div className="rounded-md border border-line bg-canvas p-4">
      <OsTrafficLights {...args} />
    </div>
  ),
};

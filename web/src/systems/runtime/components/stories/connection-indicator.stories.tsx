import type { Meta, StoryObj } from "@storybook/react-vite";

import { RuntimeConnectionIndicator } from "../connection-indicator";

const meta: Meta<typeof RuntimeConnectionIndicator> = {
  title: "systems/runtime/components/RuntimeConnectionIndicator",
  component: RuntimeConnectionIndicator,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Single owner of the daemon connection indicator. Transport state determines reachability; the canonical daemon health response determines whether a connected runtime is healthy or degraded.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Connected daemon with a healthy runtime response. */
export const Healthy: Story = {
  args: { status: "connected", healthStatus: "ok" },
  parameters: {
    docs: {
      description: { story: "Daemon reachable and reporting healthy runtime state." },
    },
  },
};

/** Connected daemon whose canonical health response reports degradation. */
export const Degraded: Story = {
  args: { status: "connected", healthStatus: "degraded" },
  parameters: {
    docs: {
      description: {
        story: "Daemon reachable but reporting degraded runtime health.",
      },
    },
  },
};

/** Initial status request in flight. */
export const Connecting: Story = {
  args: { status: "connecting", healthStatus: "ok" },
  parameters: {
    docs: {
      description: { story: "Initial connection in flight; pulses until resolved." },
    },
  },
};

/** Daemon transport cannot be reached. */
export const DangerSolid: Story = {
  args: { status: "disconnected", healthStatus: "ok" },
  parameters: {
    docs: {
      description: { story: "Daemon transport is unreachable." },
    },
  },
};

/** Status endpoint returned an unusable response. */
export const Error: Story = {
  args: { status: "error", healthStatus: "ok" },
  parameters: {
    docs: {
      description: { story: "Daemon errored; surfaces a danger tone with the error label." },
    },
  },
};

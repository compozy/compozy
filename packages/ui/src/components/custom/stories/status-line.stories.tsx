import type { Meta, StoryObj } from "@storybook/react-vite";

import { StatusLine } from "../status-line";

const meta: Meta<typeof StatusLine> = {
  title: "components/custom/StatusLine",
  component: StatusLine,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Typed status line + N-013 / N-005. Pairs the daemon `ConnectionIndicator` with a typed `Array<{key, label?, value, tone?}>` item list (replaces the legacy `ReactNode[]` shape). Items render with `·` separators and tone-driven value colors.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Connected — neutral / info / success items rendered next to the daemon LED. */
export const TypedItems: Story = {
  args: {
    status: "connected",
    daemonLabel: "Daemon",
    items: [
      { key: "sessions", label: "Sessions", value: "12", tone: "neutral" },
      { key: "agents", label: "Agents", value: "3", tone: "info" },
      { key: "workspace", label: "Workspace", value: "launch", tone: "success" },
    ],
  },
};

/** Connecting — pulsing LED with a single info-tone status item. */
export const Connecting: Story = {
  args: {
    status: "connecting",
    daemonLabel: "Daemon",
    items: [{ key: "status", label: "Status", value: "Resyncing settings…", tone: "info" }],
  },
};

/** Disconnected — danger-tone LED with a danger message. */
export const Disconnected: Story = {
  args: {
    status: "disconnected",
    daemonLabel: "Daemon",
    items: [
      {
        key: "reconnect",
        value: "Reconnect from the topbar before saving changes.",
        tone: "danger",
      },
    ],
  },
};

import type { Meta, StoryObj } from "@storybook/react-vite";

import { ConnectionIndicator, type ConnectionStatus } from "../custom/connection-indicator";

const STATES: ConnectionStatus[] = ["connected", "connecting", "disconnected", "error"];

const meta: Meta<typeof ConnectionIndicator> = {
  title: "components/custom/ConnectionIndicator",
  component: ConnectionIndicator,
  args: {
    status: "connected",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const AllStates: Story = {
  args: {},
  render: () => (
    <div className="flex flex-col items-start gap-4">
      {STATES.map(status => (
        <ConnectionIndicator key={status} status={status} />
      ))}
    </div>
  ),
};

export const CompoundSlots: Story = {
  args: {},
  render: () => (
    <ConnectionIndicator status="connecting">
      <ConnectionIndicator.Dot />
      <ConnectionIndicator.Label>Daemon handshake</ConnectionIndicator.Label>
    </ConnectionIndicator>
  ),
};

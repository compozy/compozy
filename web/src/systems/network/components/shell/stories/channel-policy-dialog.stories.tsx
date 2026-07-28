import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { storyChannels } from "@/storybook/fintech-scenario";

import { ChannelPolicyDialog } from "../channel-policy-dialog";
import type { ChannelMember } from "../../../hooks/use-channel-members";
import type { NetworkChannel, NetworkChannelSummary } from "../../../types";

const meta: Meta<typeof ChannelPolicyDialog> = {
  title: "systems/network/components/ChannelPolicyDialog",
  component: ChannelPolicyDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Channel edit surface. Name and membership are locked by UpdateNetworkChannelRequest; purpose, fanout policy, and the coordinator peer stay editable.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const members: ReadonlyArray<ChannelMember> = [
  {
    displayName: "risk-triage",
    local: true,
    peerId: "risk-triage.sess-1a2b",
    presenceState: "local",
    role: "agent",
    sessionId: "sess-1a2b",
  },
  {
    displayName: "settlement-desk",
    local: true,
    peerId: "settlement-desk.sess-9f0c",
    presenceState: "local",
    role: "agent",
    sessionId: "sess-9f0c",
  },
];

function summary(overrides: Partial<NetworkChannelSummary> = {}): NetworkChannelSummary {
  return {
    channel: storyChannels.merchantEscalations,
    coordinator_peer_id: "",
    created_at: "2026-07-17T14:00:00Z",
    created_by: "risk-triage",
    fanout_policy: "capability_match",
    peer_count: members.length,
    purpose: "Coordinate VIP merchant escalations between risk and settlement partners.",
    workspace_id: "ws_fintech",
    ...overrides,
  };
}

function detailFor(channel: NetworkChannelSummary): NetworkChannel {
  return { ...channel, kind_counts: [], peers: [] } as NetworkChannel;
}

function harness(channel: NetworkChannelSummary, isSubmitting = false) {
  return (
    <CenteredSurface className="flex-col gap-4">
      <ChannelPolicyDialog
        channel={channel}
        detail={detailFor(channel)}
        isSubmitting={isSubmitting}
        members={members}
        onOpenChange={() => undefined}
        onSubmit={async () => {}}
        open
      />
    </CenteredSurface>
  );
}

/** VC-07 capture target: locked identity above the editable delivery policy. */
export const Default: Story = {
  args: {},
  render: () => harness(summary()),
};

/** Coordinator policy selected — the member-backed coordinator picker is revealed. */
export const CoordinatorSelected: Story = {
  args: {},
  render: () =>
    harness(
      summary({
        coordinator_peer_id: "risk-triage.sess-1a2b",
        fanout_policy: "coordinator",
      })
    ),
};

/** Coordinator policy with no peer chosen yet — the save stays blocked. */
export const CoordinatorUnselected: Story = {
  args: {},
  render: () => harness(summary({ fanout_policy: "coordinator" })),
};

/** Saving: the primary action swaps to a spinner and blocks duplicate submits. */
export const Saving: Story = {
  args: {},
  render: () => harness(summary(), true),
};

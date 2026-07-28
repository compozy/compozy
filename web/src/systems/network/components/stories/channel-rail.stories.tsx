import type { Meta, StoryObj } from "@storybook/react-vite";

import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";
import { PanelSurface } from "@/storybook/story-layout";
import {
  networkChannelsFixture,
  networkDirectRoomsFixture,
  networkThreadsFixture,
} from "@/systems/network/mocks";
import {
  ChannelRail,
  ChannelRailRecents,
  ChannelRailRow,
} from "@/systems/network/components/shell";
import type { NetworkChannelSummary, NetworkRecentEntry, NetworkSurface } from "@/systems/network";

const allChannels: NetworkChannelSummary[] = [...networkChannelsFixture.channels];

const longPreviewRecents: NetworkRecentEntry[] = [
  {
    surface: "thread" satisfies NetworkSurface,
    channel: "design",
    containerId: "thread_design_redesign",
    preview:
      "Kicking off a new thread to coordinate a redesign of the network shell and recents rail.",
    lastActivityAt: "2026-05-26T01:42:00Z",
    hasUnread: true,
    participantLabel: "3 peers",
  },
  {
    surface: "direct" satisfies NetworkSurface,
    channel: "design",
    containerId: "direct_design_review",
    preview: "Can you review the typography tokens before we ship the sidebar density pass?",
    lastActivityAt: "2026-05-26T00:15:00Z",
    hasUnread: false,
    participantLabel: "two-party",
  },
];

const recents: NetworkRecentEntry[] = [
  {
    surface: "thread" satisfies NetworkSurface,
    channel: networkThreadsFixture[0]?.channel ?? "builders",
    containerId: networkThreadsFixture[0]?.thread_id ?? "thread_one",
    preview: "Open both corridors at 18:30 UTC.",
    lastActivityAt: networkThreadsFixture[0]?.last_activity_at ?? null,
    hasUnread: true,
    participantLabel: "6 peers",
  },
  {
    surface: "direct" satisfies NetworkSurface,
    channel: networkDirectRoomsFixture[0]?.channel ?? "builders",
    containerId: networkDirectRoomsFixture[0]?.direct_id ?? "direct_one",
    preview: "Replay finished. BR timeout copy is clear.",
    lastActivityAt: networkDirectRoomsFixture[0]?.last_activity_at ?? null,
    hasUnread: false,
    participantLabel: "two-party",
  },
];

const meta: Meta<typeof ChannelRail> = {
  title: "systems/network/components/ChannelRail",
  component: ChannelRail,
  parameters: {
    layout: "fullscreen",
    router: { kind: "stub" as const },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const pinnedSet = new Set([allChannels[0]?.channel ?? ""]);

export const Default: Story = {
  render: () => (
    <PanelSurface className="min-h-[640px]">
      <ChannelRail
        workspaceId={storyDefaultWorkspaceId}
        activeChannel={allChannels[0]?.channel ?? null}
        activeDirectId={null}
        directs={networkDirectRoomsFixture}
        hasUnread={() => true}
        loading={{ channels: false, directs: false, recents: false }}
        isPinned={channel => pinnedSet.has(channel)}
        onTogglePinned={() => undefined}
        pinnedChannels={allChannels.filter(channel => pinnedSet.has(channel.channel))}
        recents={recents}
        selfSessionId={null}
        unpinnedChannels={allChannels.filter(channel => !pinnedSet.has(channel.channel))}
      />
    </PanelSurface>
  ),
};

export const Loading: Story = {
  render: () => (
    <PanelSurface className="min-h-[640px]">
      <ChannelRail
        workspaceId={storyDefaultWorkspaceId}
        activeChannel={null}
        activeDirectId={null}
        directs={[]}
        hasUnread={() => false}
        loading={{ channels: true, directs: true, recents: true }}
        isPinned={() => false}
        onTogglePinned={() => undefined}
        pinnedChannels={[]}
        recents={[]}
        selfSessionId={null}
        unpinnedChannels={[]}
      />
    </PanelSurface>
  ),
};

export const Empty: Story = {
  render: () => (
    <PanelSurface className="min-h-[640px]">
      <ChannelRail
        workspaceId={storyDefaultWorkspaceId}
        activeChannel={null}
        activeDirectId={null}
        directs={[]}
        hasUnread={() => false}
        loading={{ channels: false, directs: false, recents: false }}
        isPinned={() => false}
        onTogglePinned={() => undefined}
        pinnedChannels={[]}
        recents={[]}
        selfSessionId={null}
        unpinnedChannels={[]}
      />
    </PanelSurface>
  ),
};

/**
 * Individual rail row and recents primitives used inside the channel rail.
 */
export const RailPrimitives: StoryObj<typeof ChannelRailRow> = {
  render: () => (
    <PanelSurface className="min-h-[360px] p-4">
      <div className="grid max-w-sm gap-5">
        <div className="space-y-1">
          <ChannelRailRow
            workspaceId={storyDefaultWorkspaceId}
            active
            channel={allChannels[0]!}
            hasUnread
            isPinned
            onTogglePinned={() => undefined}
          />
          <ChannelRailRow
            workspaceId={storyDefaultWorkspaceId}
            active={false}
            channel={allChannels[1]!}
            hasUnread={false}
            isPinned={false}
            onTogglePinned={() => undefined}
          />
        </div>
        <ChannelRailRecents
          workspaceId={storyDefaultWorkspaceId}
          recents={recents}
          isLoading={false}
        />
      </div>
    </PanelSurface>
  ),
};

/** Recents rows with long preview copy at narrow rail width — verifies preview truncation. */
export const LongPreviewRecents: StoryObj<typeof ChannelRailRecents> = {
  render: () => (
    <PanelSurface className="min-h-[360px] p-4">
      <div className="max-w-[220px]">
        <ChannelRailRecents
          workspaceId={storyDefaultWorkspaceId}
          recents={longPreviewRecents}
          isLoading={false}
        />
      </div>
    </PanelSurface>
  ),
};

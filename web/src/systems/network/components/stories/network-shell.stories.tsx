import type { Meta, StoryObj } from "@storybook/react-vite";

import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";
import { PanelSurface } from "@/storybook/story-layout";
import {
  networkChannelsFixture,
  networkDirectRoomsFixture,
  networkThreadsFixture,
} from "@/systems/network/mocks";
import { NetworkShell, type ChannelTab } from "@/systems/network/components/shell";
import {
  NetworkListFiltersProvider,
  useNetworkListFilters,
  type NetworkChannelSummary,
  type NetworkRecentEntry,
  type NetworkSurface,
} from "@/systems/network";

const recents: NetworkRecentEntry[] = [
  {
    surface: "thread" satisfies NetworkSurface,
    channel: networkThreadsFixture[0]?.channel ?? "builders",
    containerId: networkThreadsFixture[0]?.thread_id ?? "thread_one",
    preview: networkThreadsFixture[0]?.last_message_preview ?? "Open both corridors at 18:30 UTC.",
    lastActivityAt: networkThreadsFixture[0]?.last_activity_at ?? null,
    hasUnread: true,
    participantLabel: "6 peers",
  },
  {
    surface: "direct" satisfies NetworkSurface,
    channel: networkDirectRoomsFixture[0]?.channel ?? "builders",
    containerId: networkDirectRoomsFixture[0]?.direct_id ?? "direct_one",
    preview:
      networkDirectRoomsFixture[0]?.last_message_preview ??
      "Replay finished. BR timeout copy clear.",
    lastActivityAt: networkDirectRoomsFixture[0]?.last_activity_at ?? null,
    hasUnread: false,
    participantLabel: "two-party",
  },
];

const allChannels: NetworkChannelSummary[] = [...networkChannelsFixture.channels];

const meta: Meta<typeof NetworkShell> = {
  title: "systems/network/components/NetworkShell",
  component: NetworkShell,
  parameters: {
    layout: "fullscreen",
    router: { kind: "stub" as const },
    docs: {
      description: {
        component:
          "Channel-pivot shell for the new /network route - left rail with cross-channel Recents, channel header with Threads / Directs / Activity tabs, and a right-rail slot.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

interface NetworkShellHarnessProps {
  activeTab?: ChannelTab;
  rightRailOpen?: boolean;
  channels?: NetworkChannelSummary[];
  recentsList?: NetworkRecentEntry[];
}

function NetworkShellHarness({
  activeTab = "threads",
  rightRailOpen = false,
  channels = allChannels,
  recentsList = recents,
}: NetworkShellHarnessProps) {
  const pinnedSet = new Set(channels.slice(0, 1).map(channel => channel.channel));
  const pinned = channels.filter(channel => pinnedSet.has(channel.channel));
  const unpinned = channels.filter(channel => !pinnedSet.has(channel.channel));
  const filters = useNetworkListFilters({
    workspaceId: storyDefaultWorkspaceId,
    channel: channels[0]?.channel ?? "",
  });

  return (
    <PanelSurface className="min-h-[760px]">
      <NetworkListFiltersProvider value={filters}>
        <NetworkShell
          workspaceId={storyDefaultWorkspaceId}
          activeChannel={channels[0] ?? null}
          activeChannelDetail={null}
          activeDirectId={null}
          activeTab={activeTab}
          directCount={networkDirectRoomsFixture.length}
          directs={networkDirectRoomsFixture}
          hasUnread={() => true}
          inspectorOpen={false}
          loading={{ channels: false, directs: false, recents: false }}
          isPinned={(channel: string) => pinnedSet.has(channel)}
          onInspectorToggle={() => undefined}
          onTogglePinned={() => undefined}
          openWorkCount={2}
          pinnedChannels={pinned}
          recents={recentsList}
          rightRailMode="thread"
          rightRailOpen={rightRailOpen}
          selfSessionId={null}
          threadCount={networkThreadsFixture.length}
          unpinnedChannels={unpinned}
        >
          <div className="px-6 py-4 text-small-body text-muted">
            Tab content placeholder for message rows.
          </div>
        </NetworkShell>
      </NetworkListFiltersProvider>
    </PanelSurface>
  );
}

/**
 * Default shell with Threads tab active and a populated channel rail.
 */
export const Default: Story = {
  render: () => <NetworkShellHarness />,
};

/**
 * Directs tab active.
 */
export const DirectsTab: Story = {
  render: () => <NetworkShellHarness activeTab="directs" />,
};

/**
 * Empty channel rail with no recents - pre-onboarding state.
 */
export const EmptyChannels: Story = {
  render: () => <NetworkShellHarness channels={[]} recentsList={[]} />,
};

/**
 * Right-rail open state - overlay slot reserved for thread overlay and inspectors.
 */
export const RightRailOpen: Story = {
  render: () => <NetworkShellHarness rightRailOpen />,
};

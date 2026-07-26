import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";
import { PanelSurface } from "@/storybook/story-layout";
import {
  networkDirectRoomsFixture,
  networkThreadsFixture,
  networkWorkFixture,
} from "@/systems/network/mocks";
import type { OpenWorkEntry } from "@/systems/network/hooks/use-work";
import type { ChannelMember } from "@/systems/network/hooks/use-channel-members";
import type { NetworkChannelSummary, NetworkThreadSummary } from "@/systems/network/types";

import { ActivityFeed } from "../activity/activity-feed";
import { DirectsList } from "../directs/directs-list";
import { InspectorMembersList } from "../shell/inspector-members-list";
import { NetworkInspector } from "../shell/network-inspector";
import { ThreadsList } from "../threads/threads-list";
import { WorkInspectorRow } from "../work/work-inspector-row";

const channel = "launch-war-room";
const channelSummary: NetworkChannelSummary = {
  channel,
  peer_count: 3,
  purpose: "Coordinate the v0.16 launch across agents and humans.",
  fanout_policy: "capability_match",
  created_by: "ops-human",
  created_at: "2026-07-20T12:00:00Z",
};
const workEntry: OpenWorkEntry = {
  workId: networkWorkFixture.work_id,
  state: networkWorkFixture.state,
  messageId: "msg_launch_work",
  targetPeerId: "partner-settlement",
  openedAt: networkWorkFixture.opened_at ?? null,
  lastActivityAt: networkWorkFixture.last_activity_at ?? null,
};
const members: ChannelMember[] = [
  {
    peerId: "northstar-local",
    sessionId: "sess-northstar-local",
    displayName: "Northstar Local",
    role: "agent",
    local: true,
    presenceState: "local",
  },
  {
    peerId: "partner-settlement",
    sessionId: "sess-partner-settlement",
    displayName: "Partner Settlement",
    role: "agent",
    local: true,
    presenceState: "local",
  },
  {
    peerId: "ops-human",
    sessionId: "",
    displayName: "Ops Human",
    role: "human",
    local: true,
    presenceState: "local",
  },
];

const meta: Meta<typeof ThreadsList> = {
  title: "systems/network/components/NetworkLists",
  component: ThreadsList,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Thread, direct, activity, and open-work list surfaces for network navigation.",
      },
    },
  },
  decorators: [
    Story => (
      <PanelSurface className="min-h-[520px] p-0">
        <Story />
      </PanelSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Threads list with active row and open-work chip.
 */
export const Threads: Story = {
  args: {},
  render: () => (
    <ThreadsList
      workspaceId={storyDefaultWorkspaceId}
      channel={channel}
      threads={networkThreadsFixture}
      activeThreadId={networkThreadsFixture[0]?.thread_id ?? null}
      status="ready"
      onStartThread={fn()}
    />
  ),
};

const longContentThread: NetworkThreadSummary = {
  ...(networkThreadsFixture[0] ?? {
    channel: "design",
    last_activity_at: "2026-05-26T01:42:00Z",
    message_count: 4,
    open_work_count: 0,
    opened_at: "2026-05-26T01:00:00Z",
    opened_by_peer_id: "design-lead",
    opened_session_id: "sess-design",
    participant_count: 3,
    root_message_id: "msg-root",
    thread_id: "thread-long-content",
  }),
  channel: "design",
  thread_id: "thread-long-content",
  title:
    "Kicking off a new thread to coordinate a redesign of the network shell and recents rail with a very long title",
  last_message_preview:
    "Modernize the visual language and component system - Improve information architecture and key user flows - Raise accessibility (WCAG 2.2 AA) and Core Web Vitals across the app - Establish a token-driven design system that engineering can consume directly",
};

/**
 * Worst-case long title and preview to verify truncation without horizontal scroll.
 */
export const ThreadsLongContent: Story = {
  args: {},
  decorators: [
    Story => (
      <PanelSurface className="min-h-[520px] max-w-md p-0">
        <Story />
      </PanelSurface>
    ),
  ],
  render: () => (
    <ThreadsList
      workspaceId={storyDefaultWorkspaceId}
      channel="design"
      threads={[longContentThread]}
      activeThreadId={longContentThread.thread_id}
      status="ready"
      onStartThread={fn()}
    />
  ),
};

/**
 * Direct rooms list resolves the opposite peer and member role.
 */
export const Directs: Story = {
  args: {},
  render: () => (
    <DirectsList
      workspaceId={storyDefaultWorkspaceId}
      channel={channel}
      directs={networkDirectRoomsFixture}
      activeDirectId={networkDirectRoomsFixture[0]?.direct_id ?? null}
      isLoading={false}
      selfSessionId="sess-northstar-local"
      members={[
        {
          peerId: "partner-settlement",
          sessionId: "sess-partner-settlement",
          displayName: "Partner Settlement",
          role: "agent",
          local: true,
          presenceState: "local",
        },
        {
          peerId: "northstar-growth",
          sessionId: "sess-northstar-growth",
          displayName: "Growth Desk",
          role: "human",
          local: true,
          presenceState: "local",
        },
      ]}
      onNewDirect={fn()}
    />
  ),
};

/**
 * Activity feed merges thread and direct activity by last timestamp.
 */
export const Activity: Story = {
  args: {},
  render: () => (
    <ActivityFeed
      workspaceId={storyDefaultWorkspaceId}
      channel={channel}
      threads={networkThreadsFixture}
      directs={networkDirectRoomsFixture}
      status="ready"
    />
  ),
};

/**
 * Inspector lists cover member identity, activity ordering, tabs, and close
 * affordances without mounting the full route shell.
 */
export const Inspector: StoryObj<typeof NetworkInspector> = {
  args: {},
  render: () => (
    <div className="grid min-h-[520px] gap-4 md:grid-cols-[360px_360px]">
      <NetworkInspector
        channel={channelSummary}
        activeTab="members"
        onTabChange={fn()}
        members={members}
        isMembersLoading={false}
        workEntries={[workEntry]}
        isWorkLoading={false}
        workCount={1}
        onWorkJump={fn()}
      />
      <div className="grid min-h-0 gap-4">
        <InspectorMembersList members={members} />
      </div>
    </div>
  ),
};

/**
 * Open-work row shows target, age, state chip, and jump action.
 */
export const WorkRow: Story = {
  args: {},
  render: () => (
    <ul className="w-full max-w-md">
      <WorkInspectorRow entry={workEntry} onJump={fn()} />
    </ul>
  ),
};

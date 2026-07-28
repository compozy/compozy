import type { Meta, StoryObj } from "@storybook/react-vite";

import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";
import { PanelSurface } from "@/storybook/story-layout";
import { networkThreadDetailFixture, networkThreadMessagesFixture } from "@/systems/network/mocks";
import {
  ThreadOverlayHeader,
  ThreadOverlayReplies,
  ThreadOverlayRoot,
} from "@/systems/network/components/thread-overlay";
import type { NetworkConversationMessage } from "@/systems/network";

const root: NetworkConversationMessage | undefined = networkThreadMessagesFixture.find(
  message => message.message_id === "msg_launch_001"
);

const replies = networkThreadMessagesFixture.filter(message => message.kind === "say").slice(0, 4);

const meta: Meta = {
  title: "systems/network/components/ThreadOverlay",
  parameters: {
    layout: "fullscreen",
    router: { kind: "stub" as const },
    docs: {
      description: {
        component:
          "Right-rail thread overlay surfaces - header (with close + open-in-main), root message badge, replies divider, and reply timeline at overlay density per `_design.md` §3.2/§5.5.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Header: Story = {
  render: () => (
    <PanelSurface className="min-h-[160px] p-0">
      <div className="flex w-[420px] flex-col bg-canvas">
        <ThreadOverlayHeader
          workspaceId={storyDefaultWorkspaceId}
          channel="ops"
          detail={networkThreadDetailFixture}
          onClose={() => undefined}
          onOpenMain={() => undefined}
          threadId="thread_launch_command"
        />
      </div>
    </PanelSurface>
  ),
};

export const Root: Story = {
  render: () => (
    <PanelSurface className="min-h-[200px] p-0">
      <div className="flex w-[420px] flex-col bg-canvas">
        <ThreadOverlayRoot isLoading={false} rootMessage={root ?? null} />
      </div>
    </PanelSurface>
  ),
};

export const RepliesPopulated: Story = {
  render: () => (
    <PanelSurface className="min-h-[480px] p-0">
      <div className="flex h-[480px] w-[420px] flex-col bg-canvas">
        <ThreadOverlayReplies isLoading={false} messages={replies} replyCount={replies.length} />
      </div>
    </PanelSurface>
  ),
};

export const RepliesLoading: Story = {
  render: () => (
    <PanelSurface className="min-h-[300px] p-0">
      <div className="flex h-[300px] w-[420px] flex-col bg-canvas">
        <ThreadOverlayReplies isLoading messages={[]} replyCount={0} />
      </div>
    </PanelSurface>
  ),
};

export const RepliesEmpty: Story = {
  render: () => (
    <PanelSurface className="min-h-[260px] p-0">
      <div className="flex h-[260px] w-[420px] flex-col bg-canvas">
        <ThreadOverlayReplies isLoading={false} messages={[]} replyCount={0} />
      </div>
    </PanelSurface>
  ),
};

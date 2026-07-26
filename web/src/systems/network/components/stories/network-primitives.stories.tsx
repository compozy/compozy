import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { PanelSurface } from "@/storybook/story-layout";
import { networkThreadMessagesFixture } from "@/systems/network/mocks";
import type { NetworkConversationMessage } from "@/systems/network/types";

import { KindChip } from "@agh/ui";
import { ConversationError } from "../empty-states/conversation-error";
import { DatePill } from "../timeline/date-pill";
import { HoverToolbar } from "../timeline/hover-toolbar";
import { MessageAvatar } from "../timeline/message-avatar";
import { MessageBodyText } from "../timeline/message-body";
import { MessageRowCollapsed } from "../timeline/message-row-collapsed";
import { MessageRowSystem } from "../timeline/message-row-system";
import { NewDivider } from "../timeline/new-divider";

const message = networkThreadMessagesFixture[0] as unknown as NetworkConversationMessage;
const systemMessage = {
  ...message,
  kind: "trace",
  message_id: "network_story_system_event",
  text: "Trace event recorded the partner-bank replay checkpoint and attached rollout evidence for the launch room.",
} as NetworkConversationMessage;

const meta: Meta<typeof KindChip> = {
  title: "systems/network/components/NetworkPrimitives",
  component: KindChip,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Protocol chips, timeline separators, avatars, body text, collapsed rows, system rows, and hover actions.",
      },
    },
  },
  decorators: [
    Story => (
      <PanelSurface className="min-h-[520px] p-6">
        <Story />
      </PanelSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Protocol kind chips cover known semantic colors and unknown neutral kinds.
 */
export const KindChips: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap gap-2">
      {["say", "direct", "capability", "receipt", "trace", "whois", "custom"].map(kind => (
        <KindChip key={kind} kind={kind} />
      ))}
    </div>
  ),
};

/**
 * Conversation error primitive anchors unavailable thread and direct-room states.
 */
export const ErrorState: StoryObj<typeof ConversationError> = {
  args: {},
  render: () => (
    <ConversationError
      title="Thread unavailable"
      description="Choose an existing thread from #launch-war-room."
    />
  ),
};

/**
 * Timeline separators use mono labels and accent-only new-message affordances.
 */
export const TimelineSeparators: Story = {
  args: {},
  render: () => (
    <div className="grid gap-4">
      <DatePill timestamp="2026-04-17T18:12:00Z" now={new Date("2026-04-17T20:00:00Z")} />
      <NewDivider />
      <NewDivider label="UNREAD" />
    </div>
  ),
};

/**
 * Avatars and body text show the identity palette and message fallback body parsing.
 */
export const MessageIdentity: Story = {
  args: {},
  render: () => (
    <div className="flex items-start gap-3">
      <MessageAvatar
        initialFrom={message.display_name ?? message.peer_from ?? "peer"}
        seed={message.peer_from ?? "peer"}
        sizePx={36}
      />
      <MessageBodyText message={message} />
    </div>
  ),
};

/**
 * Collapsed and system rows keep long activity compact without losing actions.
 */
export const CompactRows: Story = {
  args: {},
  render: () => (
    <div className="relative grid gap-3">
      <MessageRowCollapsed message={message} onCopyLink={fn()} onCopyText={fn()} />
      <MessageRowSystem message={systemMessage} />
      <div className="group relative min-h-10 rounded-lg border border-line p-4">
        <span className="text-small-body text-muted">Hover toolbar container</span>
        <HoverToolbar testIdSuffix="story" onCopyLink={fn()} onCopyText={fn()} />
      </div>
    </div>
  ),
};

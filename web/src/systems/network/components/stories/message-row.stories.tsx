import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import { networkThreadMessagesFixture } from "@/systems/network/mocks";
import {
  MessageRow,
  MessageRowCollapsed,
  MessageRowSystem,
} from "@/systems/network/components/timeline";
import type { NetworkConversationMessage } from "@/systems/network";

const sayMessage =
  networkThreadMessagesFixture.find(message => message.kind === "say") ??
  (networkThreadMessagesFixture[0] as NetworkConversationMessage);

const traceMessage =
  networkThreadMessagesFixture.find(message => message.kind === "trace") ?? sayMessage;

const meta: Meta<typeof MessageRow> = {
  title: "systems/network/components/MessageRow",
  component: MessageRow,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Three message-row variants - full, collapsed continuation, system - per `_design.md` §5.2. Avatar gutter is 36px in the channel timeline and 32px in the thread overlay; never circular.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const FullRow: Story = {
  render: () => (
    <PanelSurface className="min-h-[140px] p-0">
      <MessageRow message={sayMessage} />
    </PanelSurface>
  ),
};

export const CollapsedContinuation: Story = {
  render: () => (
    <PanelSurface className="min-h-[80px] p-0">
      <MessageRowCollapsed message={sayMessage} />
    </PanelSurface>
  ),
};

export const SystemEvent: Story = {
  render: () => (
    <PanelSurface className="min-h-[80px] p-0">
      <MessageRowSystem message={traceMessage} />
    </PanelSurface>
  ),
};

export const ThreadDensity: Story = {
  name: "Thread overlay density (32px gutter)",
  render: () => (
    <PanelSurface className="min-h-[140px] p-0">
      <MessageRow density="overlay" message={sayMessage} />
    </PanelSurface>
  ),
};

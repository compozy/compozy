import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { SessionChatRuntimeProvider } from "../session-chat-runtime-provider";

const meta: Meta<typeof SessionChatRuntimeProvider> = {
  title: "systems/session/components/SessionChatRuntimeProvider",
  component: SessionChatRuntimeProvider,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Assistant runtime provider that wires AGH session transport, tools, and data UIs.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Runtime provider wraps arbitrary thread UI children for a concrete session.
 */
export const Default: Story = {
  args: {
    sessionId: "session_launch_coordination",
    workspaceId: "workspace_hq",
    children: (
      <div className="rounded-lg border border-line bg-canvas-soft p-4 text-sm text-fg">
        Session runtime children render here.
      </div>
    ),
  },
};

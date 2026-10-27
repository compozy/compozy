import { baseArgs } from "./session-thread-story-runtime";
import { primarySessionFixture } from "@/systems/session/mocks";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { SessionThread } from "@/components/assistant-ui/session-thread";
import { SessionThreadSkeleton } from "@/components/assistant-ui/session-thread-skeleton";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { HttpResponse } from "msw";
import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";

import { transcriptPayload } from "./session-thread-story-transcripts";

const storyWorkspaceId = primarySessionFixture.workspace_id ?? "ws_alpha";

/**
 * Loading-pane stories for the assistant-ui session thread shell, under the same runtime provider
 * and transcript handler as the `SessionThread` stories.
 */
const meta: Meta<typeof SessionThread> = {
  title: "systems/session/components/assistant-ui/SessionThreadLoading",
  component: SessionThread,
  parameters: {
    layout: "fullscreen",
    ...storybookMswParameters({
      session: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload([]))
        ),
      ],
    }),
    docs: {
      description: {
        component:
          "Session thread pane before its chunk or session arrives: the transcript viewport over the composer card, or without the composer for a read-only history.",
      },
    },
  },
  decorators: [
    Story => (
      <SessionChatRuntimeProvider
        sessionId={primarySessionFixture.id}
        workspaceId={storyWorkspaceId}
      >
        <div className="flex h-[640px] w-full flex-col bg-background border border-line">
          <Story />
        </div>
      </SessionChatRuntimeProvider>
    ),
  ],
};

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * The thread pane before its chunk or session arrives: the transcript viewport over the composer
 * card, in the pane the live thread fills.
 */
export const PaneLoading: Story = {
  args: baseArgs,
  render: () => (
    <div className="flex h-[640px] w-full flex-col border border-line bg-background">
      <SessionThreadSkeleton />
    </div>
  ),
};

/** A read-only history has no composer to hold a place for. */
export const ReadOnlyPaneLoading: Story = {
  args: baseArgs,
  render: () => (
    <div className="flex h-[640px] w-full flex-col border border-line bg-background">
      <SessionThreadSkeleton composer={false} />
    </div>
  ),
};

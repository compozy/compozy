import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { SessionThread } from "@/components/assistant-ui/session-thread";
import { storybookMswParameters } from "@/storybook/msw";
import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";
import type { SessionPayload } from "@/systems/session/types";

import { liveToolTranscript } from "./session-thread-story-quiet-transcripts";
import {
  subagentGroupRoster,
  subagentGroupTranscript,
  subagentNativeRoster,
  subagentNativeTranscript,
  subagentRow,
  subagentStoryHandlers,
  subagentStoryIdleSession,
  subagentStoryRunningSession,
  subagentToolsTranscript,
  type SubagentStoryScene,
} from "./session-thread-story-subagents";

const storyWorkspaceId = subagentStoryIdleSession.workspace_id ?? "ws_alpha";

function SubagentStoryHost({ session }: { session: SessionPayload }) {
  const running = session.activity !== null;
  return (
    <SessionThread
      sessionId={session.id}
      workspaceId={storyWorkspaceId}
      agentName={session.agent_name}
      sessionState={session.state}
      statusSession={session}
      canPrompt
      isSessionRunning={running}
      onCancelPrompt={() => undefined}
    />
  );
}

function scene(options: SubagentStoryScene) {
  return storybookMswParameters({ session: subagentStoryHandlers(options) });
}

/**
 * Subagents in the session view (`_uiux.md` S1, S2, S5–S8; boards
 * `subagents-transcript.html` and `subagents-composer.html`): the real
 * `SessionThread` over an MSW daemon whose stream sends the transcript and then
 * the parent's subagent roster, so cards, the waiting banner and the agents
 * count read the production path.
 */
const meta: Meta<typeof SubagentStoryHost> = {
  title: "systems/session/components/assistant-ui/SessionThread/Subagents",
  component: SubagentStoryHost,
  parameters: { layout: "centered" },
  decorators: [
    (Story, context) => (
      <SessionChatRuntimeProvider
        sessionId={context.args.session.id}
        workspaceId={storyWorkspaceId}
      >
        <div
          className="flex h-[640px] w-[900px] max-w-full flex-col overflow-hidden rounded-lg border border-line bg-canvas shadow-elevated"
          data-testid="subagent-story-window"
        >
          <Story />
        </div>
      </SessionChatRuntimeProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Transcript VC-01/VC-03 + composer VC-01: three delegations in one group after
 * the capabilities row; the parent's turn is over and two still work, so the
 * composer waits on them.
 */
export const WaitingOnSubagents: Story = {
  args: { session: subagentStoryIdleSession },
  parameters: scene({
    session: subagentStoryIdleSession,
    transcript: subagentGroupTranscript,
    roster: subagentGroupRoster,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    // The roster snapshot follows the transcript; the summary reads it.
    await waitFor(() =>
      expect(canvas.getByText("1 working · 1 needs you · 1 done")).toBeInTheDocument()
    );
    await expect(canvas.getByText("Waiting on 2 subagents")).toBeInTheDocument();
    // Transcript VC-01 shows the group open.
    await userEvent.click(canvas.getByRole("button", { name: /3 subagents/ }));
    await waitFor(() =>
      expect(canvas.getByText("Reading internal/payments/webhook.go")).toBeVisible()
    );
  },
};

/** Transcript VC-06: the subagent tool family as verbs; a failed delegate stays a tool row. */
export const ToolPhrases: Story = {
  args: { session: subagentStoryIdleSession },
  parameters: scene({
    session: subagentStoryIdleSession,
    transcript: subagentToolsTranscript,
    roster: [],
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    // The settled turn folds its tool rows; open the fold to read them (VC-06).
    const fold = await canvas.findByRole("button", {
      name: /Checked subagent capabilities 2 times/,
    });
    await userEvent.click(fold);
    await waitFor(() => expect(canvas.getByText("Read subagent status")).toBeVisible());
  },
};

/** Transcript VC-05: a provider-native card; its inner work renders only inside it. */
export const NativeNesting: Story = {
  args: { session: subagentStoryIdleSession },
  parameters: scene({
    session: subagentStoryIdleSession,
    transcript: subagentNativeTranscript,
    roster: subagentNativeRoster,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    // Transcript VC-05: the native card opens inline to its inner work.
    const card = await waitFor(() => {
      const element = canvasElement.querySelector<HTMLElement>(
        '[data-slot="subagent-card-open"][aria-expanded]'
      );
      if (!element) throw new Error("native card not rendered yet");
      return element;
    });
    await userEvent.click(card);
    await waitFor(() => expect(canvas.getByText("Inspecting the diff.")).toBeVisible());
    // The click left pointer and focus on the card; release both so the hover card closes.
    await userEvent.unhover(card);
    card.blur();
    await waitFor(() => expect(canvas.queryByText("sonnet-5.5")).toBeNull());
  },
};

/** Composer VC-03: during a turn the status line counts live subagents of both origins once. */
export const AgentsRunning: Story = {
  args: { session: subagentStoryRunningSession },
  parameters: scene({
    session: subagentStoryRunningSession,
    transcript: liveToolTranscript,
    roster: [
      subagentRow("sub-live-1", "Audit payment webhooks for retry safety", { elapsed: 120 }),
      subagentRow("sub-live-2", "Review the diff (high effort)", {
        origin: "provider_native",
        child_session_id: null,
        elapsed: 40,
      }),
    ],
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByText("2 agents running")).toBeInTheDocument());
  },
};

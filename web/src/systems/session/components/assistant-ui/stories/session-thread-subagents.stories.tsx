import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { SessionThread } from "@/components/assistant-ui/session-thread";
import { storybookMswParameters } from "@/storybook/msw";
import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";
import { SessionComposerSubagents } from "@/systems/session/components/session-composer-subagents";
import type { SessionPayload } from "@/systems/session/types";

import { liveToolTranscript } from "./session-thread-story-quiet-transcripts";
import { composerToolbarArgs } from "./session-thread-story-runtime";
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

const summary = (live: number, total: number) => ({
  live,
  total,
  failed: 0,
  attention: 0,
  most_urgent: live > 0 ? "running" : "done",
});

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
      {...composerToolbarArgs}
      subagentControl={<SessionComposerSubagents session={session} />}
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

/** Opens the composer's subagents popover once the roster has reached the trigger. */
async function openComposerSubagents(canvasElement: HTMLElement, state: string) {
  const trigger = await waitFor(() => {
    const element = canvasElement.querySelector<HTMLElement>(
      '[data-testid="composer-subagents-button"]'
    );
    if (element?.dataset.state !== state) throw new Error(`subagents trigger not ${state} yet`);
    return element;
  });
  await userEvent.click(trigger);
  await waitFor(() => expect(within(document.body).getByRole("dialog")).toBeVisible());
}

const previousRoster = [
  subagentRow("sub-prev-1", "Draft release notes for v2", {
    elapsed: 420,
    status: "completed",
    result_preview: "Drafted notes: 6 user-facing changes, 2 fixes, 1 migration step.",
  }),
  subagentRow("sub-prev-2", "Review the diff (high effort)", {
    elapsed: 300,
    status: "completed",
    origin: "provider_native",
    child_session_id: null,
    runtime: {
      agent: "",
      provider: "claude",
      model: "sonnet-5.5",
      reasoning_effort: "",
      speed: "",
    },
  }),
  subagentRow("sub-prev-3", "Benchmark the cart totals query", {
    elapsed: 240,
    status: "canceled",
  }),
];

/** Issue #766: no subagents yet — the control stays a quiet icon; its popover says so. */
export const ComposerNoSubagents: Story = {
  args: { session: subagentStoryIdleSession },
  parameters: scene({
    session: subagentStoryIdleSession,
    transcript: liveToolTranscript,
    roster: [],
  }),
  play: async ({ canvasElement }) => {
    await openComposerSubagents(canvasElement, "idle");
    await expect(within(document.body).getByText("Nothing delegated yet.")).toBeVisible();
  },
};

/** Issue #766: only finished subagents — quiet trigger, previous rows open in the popover. */
export const ComposerPreviousOnly: Story = {
  args: { session: { ...subagentStoryIdleSession, subagent_summary: summary(0, 3) } },
  parameters: scene({
    session: { ...subagentStoryIdleSession, subagent_summary: summary(0, 3) },
    transcript: liveToolTranscript,
    roster: previousRoster,
  }),
  play: async ({ canvasElement }) => {
    await openComposerSubagents(canvasElement, "idle");
    await expect(
      within(document.body).getByRole("button", { name: "Previous subagents (3)" })
    ).toHaveAttribute("aria-expanded", "true");
  },
};

/** Issue #766: two subagents work during the parent's turn — glyph and live count. */
export const ComposerActive: Story = {
  args: { session: subagentStoryRunningSession },
  parameters: scene({
    session: subagentStoryRunningSession,
    transcript: liveToolTranscript,
    roster: [
      subagentRow("sub-live-1", "Audit payment webhooks for retry safety", {
        elapsed: 120,
        progress: "Reading internal/payments/webhook.go",
      }),
      subagentRow("sub-live-2", "S1 queue core implementation", {
        origin: "provider_native",
        child_session_id: null,
        elapsed: 2_700,
        runtime: { agent: "", provider: "claude", model: "", reasoning_effort: "", speed: "" },
      }),
    ],
  }),
  play: async ({ canvasElement }) => {
    await openComposerSubagents(canvasElement, "running");
  },
};

/** Issue #766: live, waiting, failed and finished at once — current first, previous folded. */
export const ComposerMixed: Story = {
  args: { session: subagentStoryIdleSession },
  parameters: scene({
    session: subagentStoryIdleSession,
    transcript: liveToolTranscript,
    roster: [
      ...subagentGroupRoster,
      subagentRow("sub-failed", "Migrate the ledger schema", {
        elapsed: 610,
        status: "failed",
        error: "Tests failed: 3 migrations out of order.",
      }),
      ...previousRoster,
    ],
  }),
  play: async ({ canvasElement }) => {
    await openComposerSubagents(canvasElement, "attention");
  },
};

import { primarySessionFixture } from "@/systems/session/mocks";
import { type ComponentProps, type ReactNode, useEffect } from "react";
import { useSessionStore } from "@/systems/session/hooks/use-session-store";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { ScrollToBottomPill, SessionThread } from "@/components/assistant-ui/session-thread";
import { storybookMswParameters } from "@/storybook/msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { HttpResponse } from "msw";
import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";
import { SessionTranscriptThreadProvider } from "@/systems/session/lib/session-transcript-thread-context";
import { expect, userEvent, within } from "storybook/test";

import {
  changedFilesTranscript,
  foldedTurnsTranscript,
  hoverToolbarTranscript,
  mixedStreamingTranscript,
  transcriptPayload,
} from "./session-thread-story-transcripts";

const storyWorkspaceId = primarySessionFixture.workspace_id ?? "ws_alpha";

function withoutSessionValue<T>(values: Record<string, T>, sessionId: string): Record<string, T> {
  return Object.fromEntries(Object.entries(values).filter(([key]) => key !== sessionId));
}

function GoalCommandErrorFixture({ children }: { children: ReactNode }) {
  useEffect(() => {
    useSessionStore.getState().setGoalResult(primarySessionFixture.id, {
      outcome: "error",
      reason_code: "goal_objective_required",
      replaced_run_id: null,
      snapshot: null,
    });
    return () => {
      useSessionStore.setState(state => ({
        goalResults: withoutSessionValue(state.goalResults, primarySessionFixture.id),
        goalResultCommands: withoutSessionValue(state.goalResultCommands, primarySessionFixture.id),
        goalErrorVisible: withoutSessionValue(state.goalErrorVisible, primarySessionFixture.id),
      }));
    };
  }, []);

  return children;
}

/**
 * Storybook stories for the assistant-ui session thread shell.
 *
 * The component depends on `@assistant-ui/react`'s `ThreadPrimitive` /
 * `MessagePrimitive` / `ComposerPrimitive`, which in turn require an active
 * runtime context (`AssistantRuntimeProvider`). These stories use the same
 * session runtime provider as the route shell, while overriding transcript
 * hydration to keep the empty-thread chrome visible.
 */
const meta: Meta<typeof SessionThread> = {
  title: "systems/session/components/assistant-ui/SessionThread",
  component: SessionThread,
  parameters: {
    layout: "fullscreen",
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload([]))
        ),
      ],
    }),
    docs: {
      description: {
        component:
          "Shell wrapping `@assistant-ui/react` ThreadPrimitive + ComposerPrimitive. The composer input box is raised onto `--elevated` with a `--line-strong` ring so it reads distinctly against the `--canvas-soft` shell; focus-within shifts the ring to `--accent`. Idle shows an `--accent` send disc (`--accent-ink` glyph, never raw white); while a turn runs the primary disc becomes a `--danger` Stop, Enter queues the draft (with a visible hint), and queued prompts fuse onto the composer top as steer/edit/remove rows. Clear-conversation lives in the topbar, not the composer.",
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

const baseArgs = {
  sessionId: primarySessionFixture.id,
  agentName: primarySessionFixture.agent_name,
  canPrompt: true,
  onCancelPrompt: () => undefined,
};

function renderWithTranscriptState(
  args: ComponentProps<typeof SessionThread>,
  state: {
    status: "pending" | "error" | "success";
    error?: Error | null;
  }
) {
  return (
    <SessionTranscriptThreadProvider
      messages={[]}
      status={state.status}
      error={state.error ?? null}
      retry={() => undefined}
    >
      <SessionThread {...args} />
    </SessionTranscriptThreadProvider>
  );
}

/**
 * Transcript loading state — shimmer message skeleton, never the empty-state copy.
 */
export const LoadingSkeleton: Story = {
  args: baseArgs,
  render: args => renderWithTranscriptState(args, { status: "pending" }),
};

/**
 * Transcript fetch failure — retryable pane with provider detail surfaced.
 */
export const TranscriptError: Story = {
  args: baseArgs,
  render: args =>
    renderWithTranscriptState(args, {
      status: "error",
      error: new Error("Storybook transcript fetch failed."),
    }),
};

/**
 * Empty thread state — assistant-ui empty slot renders the agent eyebrow + intro copy.
 */
export const Empty: Story = {
  args: baseArgs,
};

/**
 * Running composer with queued follow-ups — the strip fuses onto the composer top,
 * each row exposing steer / edit / remove; the primary disc is the `--danger` Stop
 * and Enter queues (hint visible).
 */
export const QueuedComposer: Story = {
  args: {
    sessionId: primarySessionFixture.id,
    agentName: primarySessionFixture.agent_name,
    canPrompt: true,
    isSessionRunning: true,
    allowBusyInput: true,
    onCancelPrompt: () => undefined,
    onQueuePrompt: () => undefined,
    onInterruptPrompt: () => undefined,
    onSteerPrompt: () => undefined,
    onRemoveQueuedPrompt: () => undefined,
    onSteerQueuedPrompt: () => undefined,
    queuedPrompts: [
      { id: "inq-1", text: "Add a regression test for the reconnect path." },
      { id: "inq-2", text: "Then update the CLI docs for `--frames`." },
    ],
  },
  parameters: {
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload(mixedStreamingTranscript))
        ),
      ],
    }),
  },
};

/**
 * Disabled composer (session not active) — verifies placeholder + disabled state.
 */
export const Disabled: Story = {
  args: {
    sessionId: primarySessionFixture.id,
    agentName: primarySessionFixture.agent_name,
    canPrompt: false,
    onCancelPrompt: () => undefined,
  },
};

/**
 * Durable startup state — the persisted session is already addressable while AGH
 * prepares its runtime, and the composer remains disabled until activation.
 */
export const Starting: Story = {
  args: {
    ...baseArgs,
    sessionState: "starting",
  },
};

/**
 * Durable generic startup failure — the diagnostic replaces transcript chrome
 * without claiming that agent runtime settings are always the recovery path.
 */
export const StartupFailure: Story = {
  args: {
    ...baseArgs,
    sessionState: "stopped",
    failure: {
      kind: "startup_failure",
      summary: "The configured model is unavailable for this provider.",
    },
  },
};

/**
 * Mixed streaming turn — reasoning, unregistered and registered tool calls, then later markdown text.
 */
export const MixedStreaming: Story = {
  args: {
    sessionId: primarySessionFixture.id,
    agentName: primarySessionFixture.agent_name,
    canPrompt: true,
    onCancelPrompt: () => undefined,
  },
  parameters: {
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload(mixedStreamingTranscript))
        ),
      ],
    }),
  },
};

/**
 * Folded work turns — three settled assistant turns collapse to duration buttons while final text remains visible.
 */
export const FoldedTurns: Story = {
  args: {
    sessionId: primarySessionFixture.id,
    agentName: primarySessionFixture.agent_name,
    canPrompt: true,
    onCancelPrompt: () => undefined,
  },
  parameters: {
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload(foldedTurnsTranscript))
        ),
      ],
    }),
  },
};

/**
 * Changed-files roll-up — a settled editing turn closes with a collapsed
 * "Edited N files +a/-d" audit summary at its tail (below the folded work and the
 * terminal answer). Display-only: no Undo/Review, since AGH exposes no checkpoint
 * semantics. Expanding it lists each modified file with its diff stats.
 */
export const ChangedFilesRollup: Story = {
  args: {
    sessionId: primarySessionFixture.id,
    agentName: primarySessionFixture.agent_name,
    canPrompt: true,
    onCancelPrompt: () => undefined,
  },
  parameters: {
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload(changedFilesTranscript))
        ),
      ],
    }),
  },
};

/**
 * Goal prefill — the settled response exposes its Goal action immediately, then
 * pointer and keyboard activation both fill and focus the cancellable local draft.
 */
export const HoverToolbar: Story = {
  args: {
    sessionId: primarySessionFixture.id,
    agentName: primarySessionFixture.agent_name,
    canPrompt: true,
    onCancelPrompt: () => undefined,
  },
  parameters: {
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload(hoverToolbarTranscript))
        ),
      ],
    }),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const action = await canvas.findByRole("button", { name: "Use as Goal" });
    await expect(action).toBeVisible();
    await userEvent.click(action);
    await expect(canvas.getByRole("textbox", { name: "Session prompt" })).toHaveFocus();
    await expect(canvas.getByRole("status")).toHaveTextContent("Goal command draft");
    await userEvent.click(canvas.getByRole("button", { name: "Discard Goal command" }));
    action.focus();
    await userEvent.keyboard("{Enter}");
    await expect(canvas.getByRole("textbox", { name: "Session prompt" })).toHaveFocus();
    await expect(canvas.getByRole("status")).toHaveTextContent("Goal command draft");
  },
};

/**
 * Goal command validation — typed failures stay internal while the session-scoped
 * runtime alert gives the operator a concrete recovery action.
 */
export const GoalCommandError: Story = {
  args: {
    sessionId: primarySessionFixture.id,
    agentName: primarySessionFixture.agent_name,
    canPrompt: true,
    onCancelPrompt: () => undefined,
  },
  parameters: {
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload(hoverToolbarTranscript))
        ),
      ],
    }),
  },
  decorators: [
    Story => (
      <GoalCommandErrorFixture>
        <Story />
      </GoalCommandErrorFixture>
    ),
  ],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByRole("alert")).toHaveTextContent(
      "Add an objective after /goal, then try again."
    );
    await expect(canvas.queryByText("goal_objective_required")).not.toBeInTheDocument();
  },
};

/**
 * Busy turn controls — queue, steer, and interrupt share the running composer surface.
 */
export const BusyInputControls: Story = {
  args: {
    sessionId: primarySessionFixture.id,
    agentName: primarySessionFixture.agent_name,
    canPrompt: true,
    isSessionRunning: true,
    allowBusyInput: true,
    onCancelPrompt: () => undefined,
    onQueuePrompt: () => undefined,
    onInterruptPrompt: () => undefined,
    onSteerPrompt: () => undefined,
    isBusyInputPending: true,
  },
  parameters: {
    ...storybookMswParameters({
      session: [
        aghApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload(mixedStreamingTranscript))
        ),
      ],
    }),
  },
};

/**
 * Wide panel — viewport and composer share the same full-width content rail.
 */
export const WidePanel: Story = {
  ...MixedStreaming,
  decorators: [
    Story => (
      <SessionChatRuntimeProvider
        sessionId={primarySessionFixture.id}
        workspaceId={storyWorkspaceId}
      >
        <div className="flex h-[640px] w-[1200px] max-w-full flex-col border border-line bg-background">
          <Story />
        </div>
      </SessionChatRuntimeProvider>
    ),
  ],
};

/**
 * Onboarding inset — matches wizard header/footer horizontal padding (px-8).
 */
export const OnboardingInset: Story = {
  ...MixedStreaming,
  args: {
    sessionId: primarySessionFixture.id,
    agentName: primarySessionFixture.agent_name,
    canPrompt: true,
    contentInset: "px-8",
    onCancelPrompt: () => undefined,
  },
  decorators: [
    Story => (
      <SessionChatRuntimeProvider
        sessionId={primarySessionFixture.id}
        workspaceId={storyWorkspaceId}
      >
        <div className="flex h-[640px] w-[1200px] max-w-full flex-col border border-line bg-background">
          <Story />
        </div>
      </SessionChatRuntimeProvider>
    ),
  ],
};

/**
 * Scroll-to-bottom pill — the live-follow affordance revealed when the reader
 * scrolls away from the live edge. Neutral `size-8 rounded-full` disc
 * (`bg-canvas-soft` + `border-line`, no glass/backdrop-blur), floating over the
 * transcript above the composer. Interaction-gated in production; rendered here
 * in its visible state over a transcript-like backdrop.
 */
export const ScrollToBottomAffordance: Story = {
  args: baseArgs,
  render: () => (
    <div className="relative flex min-h-0 flex-1 flex-col bg-background">
      <div className="flex-1 space-y-4 overflow-hidden px-4 py-6 text-sm leading-7 text-muted">
        {Array.from({ length: 8 }, (_, index) => (
          <p key={index}>
            Streamed transcript line {index + 1} — the reader has scrolled up while the assistant
            keeps answering below the fold, so the live-follow pill offers a one-click return to the
            newest output.
          </p>
        ))}
      </div>
      <ScrollToBottomPill visible onClick={() => undefined} />
    </div>
  ),
};

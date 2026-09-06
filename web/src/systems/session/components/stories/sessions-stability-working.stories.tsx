import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { SessionBusyInputRefusalError, type SessionSendOutcome } from "@/systems/session";
import { liveToolTranscript } from "../assistant-ui/stories/session-thread-story-quiet-transcripts";

import { storybookMswParameters } from "@/storybook/msw";
import { quietWarningSessionFixture } from "@/systems/session/mocks";
import {
  type SessionQuietWarning,
  sessionQuietWarning,
} from "@/systems/session/lib/session-quiet-warning";

import {
  childrenRunningTranscript,
  escalatedStopTranscript,
  inactivityStoppedSession,
  inactivityStopTranscript,
  runningSession,
  settledTurnsTranscript,
  thinkingTranscript,
} from "./sessions-stability-story-fixtures";
import { stabilityHandlers, discardStabilityDraft } from "./sessions-stability-story-routes";
import { StabilityThreadHost } from "./sessions-stability-story-host";

const MINUTE = 60_000;

/** Re-anchors the daemon's quiet episode on the story's mount time so the live clock reads "31m · 9m". */
function anchoredQuietWarning(quietForMinutes: number): SessionQuietWarning {
  const warning = sessionQuietWarning(quietWarningSessionFixture)!;
  const shift = Date.now() - quietForMinutes * MINUTE - warning.quietSinceMs;
  return {
    quietSinceMs: warning.quietSinceMs + shift,
    warnedAtMs: warning.warnedAtMs + shift,
    stopAtMs: warning.stopAtMs === null ? null : warning.stopAtMs + shift,
  };
}

/**
 * The working board's states that only exist in composition (task_05 VC-01,
 * task_07 VC-10, VC-12): the quiet warning on the notice rail with the quiet
 * clock in the status row, the thinking row with no assistant shell yet, and
 * the frozen stop sentences the daemon's own receipts produce. The transcript,
 * the status row, and the composer are the production thread over MSW.
 */
const meta: Meta<typeof StabilityThreadHost> = {
  title: "systems/session/components/SessionsStability/Working",
  component: StabilityThreadHost,
  loaders: [discardStabilityDraft],
  parameters: {
    layout: "centered",
    ...storybookMswParameters({ session: stabilityHandlers(settledTurnsTranscript) }),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** task_05 VC-01 — "Quiet for 30 minutes." on the notice rail (Stop now), the status row counting "Quiet for 31m · stops in 9m". */
export const QuietWarning: Story = {
  render: () => (
    <StabilityThreadHost
      quietWarning={anchoredQuietWarning(31)}
      statusSession={quietWarningSessionFixture}
    />
  ),
};

/** task_07 VC-10 — the transcript ends at the operator's bubble; the status row reads "Thinking…" with the text shimmer. */
export const Thinking: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(thinkingTranscript) }),
  render: () => (
    <StabilityThreadHost isSessionRunning statusSession={runningSession({ startedAgoMs: 3_000 })} />
  ),
};

/** task_07 VC-12 — the verified escalated receipt: "Stopped after 14s · the agent didn't answer the stop, so it was closed for you"; the cut call reads "stopped". */
export const StoppedEscalated: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(escalatedStopTranscript) }),
};

/** task_07 VC-12 — supervision's inactivity stop: "Stopped after 31m · no work for 40 minutes" from the daemon's `session.supervision_stopped` episode. */
export const StoppedInactivity: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(inactivityStopTranscript) }),
  render: () => <StabilityThreadHost statusSession={inactivityStoppedSession} />,
};

/** task_07 VC-12 / US-009.EC-2 — the stop arrived after the turn finished: the faint transient "Completed" note, the turn folds normally. */
export const StopCompletionNote: Story = {
  render: () => <StabilityThreadHost stopCompletionNote />,
};

/** task_07 VC-11 — the lead reply landed, two spawned agents still run as their own live rows (bot glyph); the status row reads "Working for … · 2 agents running". */
export const ChildrenRunning: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(childrenRunningTranscript) }),
  render: () => (
    <StabilityThreadHost
      isSessionRunning
      statusSession={runningSession({ agents: 2, startedAgoMs: 362_000 })}
    />
  ),
};

// Integrated recaptures for task01/02: send through the production composer,
// with the application callback boundary supplying the daemon outcome.
const composerOutcome = (delivery: SessionSendOutcome["steerDelivery"]): SessionSendOutcome => ({
  disposition: "steering",
  entryId: "inp_4d9",
  idempotencyKey: "idk_1f77",
  messageId: "msg_01k4",
  queuePosition: null,
  replayed: false,
  steerDelivery: delivery,
  turnId: "story-turn-quiet",
});
const composerParameters = storybookMswParameters({
  session: stabilityHandlers(liveToolTranscript),
});
const composerSend: NonNullable<Story["play"]> = async ({ canvasElement }) => {
  const canvas = within(canvasElement);
  const input = await canvas.findByTestId("composer-input");
  const editable = input.querySelector<HTMLElement>('[contenteditable="true"]') ?? input;
  await userEvent.click(editable);
  await userEvent.keyboard("Only touch the lifecycle tests, skip the store package");
  await userEvent.keyboard("{Enter}");
  await waitFor(() => expect(canvas.getByTestId("composer-feedback-note")).toBeVisible());
};
function deliveryScene(delivery: SessionSendOutcome["steerDelivery"]): Story {
  return {
    parameters: composerParameters,
    render: () => (
      <StabilityThreadHost
        isSessionRunning
        statusSession={runningSession({ turnId: "story-turn-quiet" })}
        composerProps={{ onSteerPrompt: async () => composerOutcome(delivery) }}
      />
    ),
    play: composerSend,
  };
}
export const ComposerInjected: Story = deliveryScene("injected");
export const ComposerPending: Story = deliveryScene("pending_injection");
export const ComposerFallback: Story = deliveryScene("interrupt_fallback");
export const ComposerQueued: Story = {
  parameters: composerParameters,
  render: () => (
    <StabilityThreadHost
      isSessionRunning
      statusSession={runningSession({ turnId: "story-turn-quiet" })}
      composerProps={{
        busyInputDefaultMode: "queue",
        onQueuePrompt: async () => ({
          ...composerOutcome(null),
          disposition: "queued",
          entryId: "inp_4d8",
          queuePosition: 2,
        }),
      }}
    />
  ),
  play: composerSend,
};
export const ComposerRefused: Story = {
  parameters: composerParameters,
  render: () => (
    <StabilityThreadHost
      isSessionRunning
      statusSession={runningSession({ turnId: "story-turn-quiet" })}
      composerProps={{
        onSteerPrompt: async () => {
          throw new SessionBusyInputRefusalError({
            code: "active_turn_mismatch",
            currentTurnId: "t_9f3",
          });
        },
      }}
    />
  ),
  play: composerSend,
};
export const ComposerStopping: Story = {
  parameters: composerParameters,
  render: () => (
    <StabilityThreadHost
      isSessionRunning
      statusSession={runningSession({ turnId: "story-turn-quiet" })}
      composerProps={{ stopPhase: "stopping" }}
    />
  ),
};
export const ComposerStopped: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(escalatedStopTranscript) }),
};

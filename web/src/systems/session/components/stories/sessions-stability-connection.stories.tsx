import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";

import { liveToolTranscript } from "../assistant-ui/stories/session-thread-story-quiet-transcripts";
import { runningSession, settledTurnsTranscript } from "./sessions-stability-story-fixtures";
import {
  stabilityHandlers,
  STREAM_LOST_AT,
  transportState,
  discardStabilityDraft,
} from "./sessions-stability-story-routes";
import { StabilityThreadHost } from "./sessions-stability-story-host";

const LIVE_STORY_TURN = "story-turn-quiet";

/**
 * The connection board inside the session window (task_06 VC-01..05): the
 * production transport chip at the head's trailing edge, the end-of-transcript
 * notices, the sync-failed pane, and the composer's disconnected send guard —
 * each over the store's own transport snapshot, injected through the same
 * context the live-tail store provides, with the 2s grace already elapsed.
 */
const meta: Meta<typeof StabilityThreadHost> = {
  title: "systems/session/components/SessionsStability/Connection",
  component: StabilityThreadHost,
  loaders: [discardStabilityDraft],
  parameters: {
    layout: "centered",
    ...storybookMswParameters({ session: stabilityHandlers(liveToolTranscript) }),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-01 — "Reconnecting · 3": info chip with the attempt count; the transcript, the live row, and the composer stay as they were. */
export const Reconnecting: Story = {
  render: () => (
    <StabilityThreadHost
      isSessionRunning
      statusSession={runningSession({ currentTool: "Bash", turnId: LIVE_STORY_TURN })}
      transport={transportState({
        degradedAt: STREAM_LOST_AT,
        phase: "waiting-reconnect",
        reconnectAttempt: 3,
      })}
    />
  ),
};

/** VC-01 (tooltip) — hovering the chip states when the stream was last live and the retry cadence: "Live updates lost at HH:MM · retrying every 4s". */
export const ReconnectingTooltip: Story = {
  render: Reconnecting.render,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.hover(await canvas.findByTestId("session-transport-chip"));
    await waitFor(() => {
      const popup = document.querySelector('[data-slot="tooltip-content"][data-open]');
      expect(popup).not.toBeNull();
      expect(popup).toHaveTextContent(/Live updates lost at .+ · retrying every \d+s/);
    });
  },
};

/** VC-02 — back and replaying the gap: the "Catching up" chip and the one-line history-reset marker at the end of the transcript. */
export const CatchingUp: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(settledTurnsTranscript) }),
  render: () => (
    <StabilityThreadHost
      transport={transportState({
        catchingUp: true,
        historyReset: { at: STREAM_LOST_AT + 60_000, generation: 4, reason: "generation_mismatch" },
        phase: "live",
      })}
    />
  ),
};

/** VC-03 (loaded, then lost) — retries ran out: the only danger chip, the "Live updates stopped at 14:02 — couldn't reconnect after 6 tries" marker, Try again. */
export const DisconnectedLoaded: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(settledTurnsTranscript) }),
  render: () => (
    <StabilityThreadHost
      transport={transportState({
        degradedAt: STREAM_LOST_AT,
        failure: { at: STREAM_LOST_AT + 30_000, attempts: 6 },
        phase: "failed",
      })}
    />
  ),
};

/** VC-03 (never loaded) — the sync-failed pane in the thread's own geometry: "This conversation didn't sync", the count of tries, Try again. */
export const DisconnectedNeverLoaded: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers([]) }),
  render: () => (
    <StabilityThreadHost
      transport={transportState({
        degradedAt: STREAM_LOST_AT,
        failure: { at: STREAM_LOST_AT + 30_000, attempts: 6 },
        phase: "failed",
      })}
    />
  ),
};

/** VC-04 — Send while disconnected: the production guard answers the press with the info note; the draft stays, nothing is disabled. */
export const SendGuard: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(settledTurnsTranscript) }),
  render: () => (
    <StabilityThreadHost
      transport={transportState({
        degradedAt: STREAM_LOST_AT,
        phase: "waiting-reconnect",
        reconnectAttempt: 4,
      })}
    />
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const input = await canvas.findByTestId("composer-input");
    const editable = input.querySelector<HTMLElement>('[contenteditable="true"]') ?? input;
    await userEvent.click(editable);
    await userEvent.keyboard("Only touch the lifecycle tests, skip the store package");
    await userEvent.click(await canvas.findByTestId("composer-send-button"));
    await waitFor(() => {
      expect(canvas.getByTestId("composer-feedback-note")).toHaveTextContent(/Not sent/);
    });
    await expect(editable).toHaveTextContent("Only touch the lifecycle tests");
  },
};

/** VC-05 — a background window keeps its last frame: the neutral "Paused · 4m" chip, the status row reading "as of 14:02". */
export const BackgroundPaused: Story = {
  render: () => (
    <StabilityThreadHost
      isSessionRunning
      liveDataEnabled={false}
      statusSession={runningSession({ currentTool: "Bash", turnId: LIVE_STORY_TURN })}
      transport={transportState({ lastLiveAt: Date.now() - 4 * 60_000, phase: "disabled" })}
      windowLive={false}
    />
  ),
};

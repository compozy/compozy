import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";

import {
  completedGroupTranscript,
  interruptedOpenTranscript,
  liveToolTranscript,
  parallelToolsTranscript,
  partialFailureTranscript,
  steerFallbackTranscript,
  steerMarkersTranscript,
  streamingProseTranscript,
} from "../assistant-ui/stories/session-thread-story-quiet-transcripts";
import { runningSession } from "./sessions-stability-story-fixtures";
import { discardStabilityDraft, stabilityHandlers } from "./sessions-stability-story-routes";
import { StabilityThreadHost } from "./sessions-stability-story-host";

/** The turn id the quiet-timeline fixtures record their live work under. */
const QUIET_TURN = "story-turn-quiet";

function running(currentTool?: string) {
  return runningSession({ currentTool, turnId: QUIET_TURN });
}

/**
 * The quiet-timeline states (task_07 VC-01..04, VC-06, VC-07, VC-09, VC-12)
 * over the exact fixtures `SessionThread › QuietTimeline*` renders, inside the
 * artboard's 860px session-window body instead of that module's full-width
 * 640px shell — so a capture at 1440×900 frames the reference piece.
 */
const meta: Meta<typeof StabilityThreadHost> = {
  title: "systems/session/components/SessionsStability/QuietTimeline",
  component: StabilityThreadHost,
  loaders: [discardStabilityDraft],
  parameters: {
    layout: "centered",
    ...storybookMswParameters({ session: stabilityHandlers(liveToolTranscript) }),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-01 — the settled turn folded, the live turn's prose and completed group, one live row "Running shell — …". */
export const SingleLive: Story = {
  render: () => <StabilityThreadHost isSessionRunning statusSession={running("Bash")} />,
};

/** VC-02 — three calls in flight stay one honest row: "Running 3 tools…" with the layers glyph. */
export const ParallelLive: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(parallelToolsTranscript) }),
  render: () => <StabilityThreadHost isSessionRunning statusSession={running()} />,
};

/** VC-02 (expanded) — the parallel row opened lists the in-flight calls. */
export const ParallelLiveExpanded: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(parallelToolsTranscript) }),
  render: () => <StabilityThreadHost isSessionRunning statusSession={running()} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("live-tool-parallel"));
    await waitFor(() => {
      expect(canvas.getByTestId("live-tool-entries")).toBeVisible();
    });
  },
};

/** VC-03 (closed) — six settled tools of the live turn rest as one sentence; the live row below. */
export const CompletedGroupClosed: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(completedGroupTranscript) }),
  render: () => <StabilityThreadHost isSessionRunning statusSession={running("Bash")} />,
};

/** VC-04 — one absorbed failure: "· 1 failed" in the same ink, no danger; the live row keeps going. */
export const PartialFailure: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(partialFailureTranscript) }),
  render: () => <StabilityThreadHost isSessionRunning statusSession={running("Bash")} />,
};

/** VC-06 (left) — the turn you stopped stays open: the cut call reads "stopped", "You stopped after 1m 40s" in warning ink. */
export const InterruptedOpen: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(interruptedOpenTranscript) }),
};

/** VC-06 (right) — a steer that fell back to interrupt folds normally: "Interrupted after 48s · replaced by your steer · Ran 2 commands". */
export const SteerFallbackFold: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(steerFallbackTranscript) }),
};

/** VC-07 — every steer state under its bubble: injected, pending, superseded (quieter), interrupted-and-replaced, from the queue. */
export const SteerProvenance: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(steerMarkersTranscript) }),
};

/** VC-07 (top of the thread) — the same transcript scrolled to its start: injected ("delivered into the live turn"), pending ("the agent sees it when the current tool finishes"), and the superseded bubble. */
export const SteerProvenanceTop: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(steerMarkersTranscript) }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findAllByTestId("user-message-steer-meta");
    const viewport = await canvas.findByTestId("chat-view");
    viewport.scrollTop = 0;
    viewport.dispatchEvent(new Event("scroll"));
    await waitFor(() => {
      const meta = canvas.getAllByTestId("user-message-steer-meta")[0]!;
      const view = viewport.getBoundingClientRect();
      const rect = meta.getBoundingClientRect();
      expect(rect.top >= view.top && rect.bottom <= view.bottom).toBe(true);
      expect(meta).toHaveTextContent("Steered — delivered into the live turn");
    });
  },
};

/** VC-09 (on) — assistant prose still streaming with the smooth reveal on (default preference). */
export const StreamingSmoothOn: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(streamingProseTranscript) }),
  render: () => <StabilityThreadHost isSessionRunning statusSession={running()} />,
};

/** VC-12 (by you) — the status row's frozen sentence after the operator's stop: "Stopped by you after 1m 40s". */
export const StoppedByUser: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(interruptedOpenTranscript) }),
};

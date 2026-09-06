import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";

import {
  completedGroupTranscript,
  giantPayloadTranscript,
  partialFailureTranscript,
  streamingProseTranscript,
} from "../assistant-ui/stories/session-thread-story-quiet-transcripts";
import {
  giantPayloadLiveTranscript,
  runningSession,
  settledTurnsTranscript,
} from "./sessions-stability-story-fixtures";
import { stabilityHandlers, discardStabilityDraft } from "./sessions-stability-story-routes";
import { SmoothStreamingOff, StabilityThreadHost } from "./sessions-stability-story-host";

const LIVE_STORY_TURN = "story-turn-quiet";

// The giant result belongs to a settled turn: open its fold first, then the row.
async function expandToolRow(canvasElement: HTMLElement) {
  const canvas = within(canvasElement);
  await userEvent.click(await canvas.findByTestId("turn-fold-row"));
  const row = await canvas.findByTestId("tool-call-row");
  const toggle = row.querySelector<HTMLElement>("[aria-expanded]");
  if (!toggle) throw new Error("The tool row has no disclosure toggle.");
  await userEvent.click(toggle);
  await waitFor(() => {
    expect(canvas.getByTestId("detail-payload-truncation")).toBeVisible();
  });
  return canvas;
}

/**
 * Timeline states the quiet-timeline stories reach only through a click
 * (task_07 VC-03 open, VC-05 both cells, VC-08 expanded / Show all) and the
 * Smooth streaming preference turned off (VC-09). The collapsed and live
 * states live in `SessionThread` › QuietTimeline*; this module opens them
 * with the production disclosures.
 */
const meta: Meta<typeof StabilityThreadHost> = {
  title: "systems/session/components/SessionsStability/Timeline",
  component: StabilityThreadHost,
  loaders: [discardStabilityDraft],
  parameters: {
    layout: "centered",
    ...storybookMswParameters({ session: stabilityHandlers(settledTurnsTranscript) }),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// Opens the live turn's completed-tools group and waits until its rows are laid
// out, so a capture taken after the play finds the open state.
async function openWorkGroup(canvasElement: HTMLElement) {
  const canvas = within(canvasElement);
  const group = await canvas.findByTestId("work-summary-row");
  const toggle = within(group).getByRole("button");
  await userEvent.click(toggle);
  await waitFor(() => {
    expect(canvas.getByTestId("work-summary-entries")).toBeVisible();
    expect(canvas.getAllByTestId("tool-call-row").length).toBeGreaterThan(0);
  });
  await new Promise(resolve => requestAnimationFrame(() => resolve(undefined)));
}

/** VC-03 (open) — the completed-tools group of the live turn opened: the production ToolCallRows inside, the live row still below. */
export const CompletedGroupExpanded: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(completedGroupTranscript) }),
  render: () => (
    <StabilityThreadHost
      isSessionRunning
      statusSession={runningSession({ currentTool: "Bash", turnId: LIVE_STORY_TURN })}
    />
  ),
  play: async ({ canvasElement }) => {
    await openWorkGroup(canvasElement);
  },
};

/** VC-04 (open) — the group with one absorbed failure opened: the failed row wears the subtle × and the word "failed" with its own output; the live row keeps going below. */
export const PartialFailureExpanded: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(partialFailureTranscript) }),
  render: () => (
    <StabilityThreadHost
      isSessionRunning
      statusSession={runningSession({ currentTool: "Bash", turnId: LIVE_STORY_TURN })}
    />
  ),
  play: async ({ canvasElement }) => {
    await openWorkGroup(canvasElement);
    const canvas = within(canvasElement);
    await waitFor(() => {
      expect(canvas.getByTestId("tool-call-state-word")).toHaveTextContent("failed");
    });
    const failedRow = canvas
      .getByTestId("tool-call-state-word")
      .closest('[data-testid="tool-call-row"]');
    const toggle = failedRow?.querySelector<HTMLElement>("[aria-expanded]");
    if (!toggle) throw new Error("The failed tool row has no disclosure toggle.");
    await userEvent.click(toggle);
    await waitFor(() => expect(toggle).toHaveAttribute("aria-expanded", "true"));
  },
};

/** VC-05 (closed, default) — "Worked for 4m 12s · Ran 6 commands, edited 2 files, read 1 file" over the answer; the zero-tool turn below has no fold row. */
export const SettledFoldCollapsed: Story = {};

/** VC-05 (open) — the same fold opened: the settled ToolCallRows directly under the line (no second disclosure), the answer still below. */
export const SettledFoldExpanded: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("turn-fold-row"));
    await waitFor(() => {
      expect(canvas.getAllByTestId("tool-call-row").length).toBeGreaterThan(1);
      expect(canvas.queryByTestId("work-summary-row")).not.toBeInTheDocument();
    });
    canvas.getByTestId("turn-fold-row").scrollIntoView?.({ block: "start" });
  },
};

/** VC-08 (row collapsed, on load) — the 12,480-line result as a single settled call of the live turn: no fold, the collapsed ToolCallRow with its bounded preview. */
export const GiantPayloadRowCollapsed: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(giantPayloadLiveTranscript) }),
  render: () => <StabilityThreadHost isSessionRunning statusSession={runningSession()} />,
};

/** VC-08 (expanded) — the 12,480-line result opened: the bounded body shows the head; the strip says how much of how much, Show all, Download. */
export const GiantPayloadExpanded: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(giantPayloadTranscript) }),
  play: async ({ canvasElement }) => {
    await expandToolRow(canvasElement);
  },
};

/** VC-08 (Show all) — the rest renders in the same scrolling body; the control now reads Show less. */
export const GiantPayloadShowAll: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(giantPayloadTranscript) }),
  play: async ({ canvasElement }) => {
    const canvas = await expandToolRow(canvasElement);
    await userEvent.click(canvas.getByTestId("detail-payload-show-all"));
    await waitFor(() => {
      expect(canvas.getByTestId("detail-payload-show-all")).toHaveTextContent("Show less");
    });
  },
};

/** VC-09 (off) — the Smooth streaming setting off: chunks render as they arrive, no reveal timers; the text is the same. */
export const StreamingSmoothOff: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(streamingProseTranscript) }),
  decorators: [
    Story => (
      <SmoothStreamingOff>
        <Story />
      </SmoothStreamingOff>
    ),
  ],
  render: () => (
    <StabilityThreadHost
      isSessionRunning
      statusSession={runningSession({ turnId: LIVE_STORY_TURN })}
    />
  ),
};

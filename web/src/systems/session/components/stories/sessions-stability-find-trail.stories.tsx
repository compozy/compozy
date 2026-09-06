import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fireEvent, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";

import { denseTranscript, settledTurnsTranscript } from "./sessions-stability-story-fixtures";
import { stabilityHandlers, discardStabilityDraft } from "./sessions-stability-story-routes";
import { StabilityThreadHost } from "./sessions-stability-story-host";

/** The trail needs ≥864px of pane beside the transcript column; the window is drawn at 900px here. */
const TRAIL_WINDOW_WIDTH = 900;

async function openFind(canvasElement: HTMLElement, query: string) {
  const canvas = within(canvasElement);
  await canvas.findByText("Only touch the lifecycle tests, skip the store package");
  await userEvent.keyboard("{Meta>}f{/Meta}");
  const field = await canvas.findByTestId("session-find-input");
  const input = field.matches("input") ? field : field.querySelector("input");
  if (!input) throw new Error("The find bar has no text field.");
  await waitFor(() => expect(input).toHaveFocus());
  // The controlled field is typed through the DOM's own value setter: user-event's
  // value interceptor does not reach React's change tracking in the browser.
  fireEvent.input(input, { target: { value: query } });
  return canvas;
}

/**
 * Find and the trail over a real transcript (task_08 VC-01..05): ⌘F docks the
 * production bar, the search route answers over the same fixture the thread
 * renders, the matches are painted by the viewport's own highlight registry,
 * a jump opens the fold it lands in, and the trail's ticks come from the
 * outline route with the hover card on a real tick.
 */
const meta: Meta<typeof StabilityThreadHost> = {
  title: "systems/session/components/SessionsStability/FindTrail",
  component: StabilityThreadHost,
  loaders: [discardStabilityDraft],
  parameters: {
    layout: "centered",
    ...storybookMswParameters({ session: stabilityHandlers(settledTurnsTranscript) }),
  },
  render: () => <StabilityThreadHost width={TRAIL_WINDOW_WIDTH} />,
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The docked search field before a query is entered. */
export const FindOpenEmpty: Story = {
  play: async ({ canvasElement }) => {
    await openFind(canvasElement, "");
  },
};

/** VC-01 — "lifecycle": the bar under the head with "1 of 4", the result list with who / snippet / time and the folded tag, every match glazed in the transcript, the fold noting "1 match inside". */
export const FindMatches: Story = {
  play: async ({ canvasElement }) => {
    const canvas = await openFind(canvasElement, "lifecycle");
    await waitFor(() => {
      expect(canvas.getByTestId("session-find-count")).toHaveTextContent("4 matches");
    });
    await expect(canvas.getAllByTestId("session-find-folded").length).toBeGreaterThan(0);
    // Enter steps to the first match: the active glaze and "1 of 4".
    await userEvent.keyboard("{Enter}");
    await waitFor(() => {
      expect(canvas.getByTestId("session-find-count")).toHaveTextContent("1 of 4");
    });
  },
};

/** VC-02 — "sqlite": the count reads "No matches", the list carries the query back, no step buttons. */
export const FindNoMatches: Story = {
  play: async ({ canvasElement }) => {
    const canvas = await openFind(canvasElement, "sqlite");
    await waitFor(() => {
      expect(canvas.getByTestId("session-find-empty")).toBeVisible();
    });
  },
};

/** VC-03 — picking the match behind the fold opens it: the fold row notes "opened for a match", the active match glazed inside the tool preview. */
export const FindMatchInsideFold: Story = {
  play: async ({ canvasElement }) => {
    const canvas = await openFind(canvasElement, "lifecycle");
    await waitFor(() => {
      expect(canvas.getByTestId("session-find-count")).toHaveTextContent("4 matches");
    });
    const folded = canvas.getAllByTestId("session-find-folded")[0]!;
    const option = folded.closest<HTMLElement>('[role="option"]');
    if (!option) throw new Error("The folded match is not a result row.");
    await userEvent.click(option);
    await waitFor(() => {
      expect(canvas.getByTestId("turn-fold-note")).toHaveTextContent("opened for a match");
    });
  },
};

/** VC-04 — the trail on the scroller's left edge: one tick per sent message, the anchor at 90%; hovering a tick shows the ask, the final reply, and the time. */
export const TrailHover: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const ticks = await canvas.findAllByTestId("session-trail-tick");
    await userEvent.hover(ticks[1]!);
    await waitFor(() => {
      expect(within(document.body).getByText(/the store package is untouched/)).toBeVisible();
    });
  },
};

/** VC-05 — sixty sent messages: the ticks compress to the pane and magnify around the pointer instead of becoming a wall. */
export const TrailDense: Story = {
  parameters: storybookMswParameters({ session: stabilityHandlers(denseTranscript(60)) }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const ticks = await canvas.findAllByTestId("session-trail-tick");
    await expect(ticks.length).toBe(60);
    await userEvent.hover(ticks[30]!);
  },
};

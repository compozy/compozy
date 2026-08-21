import { useState } from "react";
import { MotionConfig } from "motion/react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { StoryTopbarHost } from "@/storybook/story-layout";

import { type LoopNodeSelection, LoopRunPageBody } from "../../index";
import { registerDeepAndWideScenario } from "./loop-run-deep-graph-fixtures";
import { buildScenarioProps } from "./loop-run-scenario-props";
import type { LoopRunStoryScenario } from "./loop-run-scenario-types";
import {
  LONG_STORY_PAGE_SIZE,
  registerLongStoryScenario,
  registerNoStepsScenario,
  registerRetryingScenario,
  registerRunningScenario,
  registerWideFanOutScenario,
} from "./loop-run-register-fixtures";
import {
  VC_PRUNED_SESSION_IDS,
  vcBudgetWarningScenario,
  vcCancelDispositionsScenario,
  vcCrashInterruptedScenario,
  vcFailedAndQuarantinedScenario,
  vcGenerationHistoryScenario,
  vcPrunedSessionScenario,
  vcQueuedScenario,
  vcRequestExpiryScenario,
} from "./loop-run-vc-fixtures";

/**
 * The operator register, staged.
 *
 * Which lane is showing and which node is open are in-page state by design, so
 * the stories that need a specific lane click the real lane tab in `play`
 * instead of reaching past the component with a prop that exists only for
 * Storybook. What gets captured is the register a person would actually be
 * looking at, reached the way they would reach it.
 */
function RegisterPage({
  scenario,
  prunedSessionIds,
}: {
  scenario: LoopRunStoryScenario;
  prunedSessionIds?: ReadonlySet<string>;
}) {
  const [inspectOpen, setInspectOpen] = useState(true);
  const [nodeSelection, setNodeSelection] = useState<LoopNodeSelection | null>(null);
  return (
    <div className="flex h-dvh flex-col bg-canvas">
      <StoryTopbarHost title="Loops">
        <div className="flex min-h-0 flex-1 flex-col bg-canvas">
          <LoopRunPageBody
            {...buildScenarioProps(scenario)}
            inspect={{ open: inspectOpen, onOpenChange: setInspectOpen }}
            nodeSelection={nodeSelection}
            // `use-loop-run-timetravel` wires both on the real page (US-013:
            // "Compare/Fork preserved"), so a story that omitted them captured a
            // generation history the shipped surface does not have.
            onCompareGeneration={() => undefined}
            onForkGeneration={() => undefined}
            onNodeSelectionChange={setNodeSelection}
            prunedSessionIds={prunedSessionIds}
          />
        </div>
      </StoryTopbarHost>
    </div>
  );
}

const meta: Meta<typeof RegisterPage> = {
  title: "systems/loops/components/LoopRunRegisters",
  component: RegisterPage,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const openLane =
  (lane: "graph" | "nodes" | "generations") => async (context: { canvasElement: HTMLElement }) => {
    const canvas = within(context.canvasElement);
    await userEvent.click(canvas.getByTestId(`loop-lane-${lane}`));
    await expect(canvas.getByTestId(`loop-run-inspect-lane-${lane}`)).toBeInTheDocument();
  };

// VC-02 · queued on admission, with the cap named.
export const DefaultQueued: Story = {
  render: () => <RegisterPage scenario={vcQueuedScenario()} />,
};

// VC-12 · budget nearly exhausted; the rail warns before the run stops.
export const DefaultBudgetWarning: Story = {
  render: () => <RegisterPage scenario={vcBudgetWarningScenario()} />,
};

// VC-15 · the expiry is stated, and nothing retries itself.
export const NeedsYouExpiry: Story = {
  render: () => <RegisterPage scenario={vcRequestExpiryScenario()} />,
};

// VC-17 · a live graph with the edge pulsing into what is running.
export const GraphLive: Story = {
  render: () => <RegisterPage scenario={registerRunningScenario()} />,
  play: openLane("graph"),
};

// VC-18 · the same graph, terminal: final states, no last-live frame.
export const GraphTerminal: Story = {
  render: () => <RegisterPage scenario={vcGenerationHistoryScenario()} />,
  play: openLane("graph"),
};

// VC-19 · failed and quarantined chips, side by side.
export const GraphFailedAndQuarantined: Story = {
  render: () => <RegisterPage scenario={vcFailedAndQuarantinedScenario()} />,
  play: openLane("graph"),
};

// VC-22 · deep as well as wide: the tail sits outside the lane's box and the
// lane stays navigable to it. Staging the wide scenario here made this row a
// duplicate of VC-21.
export const GraphDeepAndWide: Story = {
  render: () => <RegisterPage scenario={registerDeepAndWideScenario()} />,
  play: openLane("graph"),
};

// VC-23 · one node opened: attempts, links and the verbs the daemon allows.
export const NodePanelOpen: Story = {
  render: () => <RegisterPage scenario={vcFailedAndQuarantinedScenario()} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("loop-lane-nodes"));
    await userEvent.click(canvas.getByTestId("loop-roster-row-2:fix_batch:0"));
    await expect(canvas.getByTestId("loop-node-panel")).toBeInTheDocument();
  },
};

/**
 * VC-24 · reduced motion: the edge pulse is unmounted, not paused.
 *
 * `MotionConfig` is what actually reduces it. `useReducedMotionConfig` resolves
 * against motion's own context first and the media query second, and the
 * Storybook `globals.reducedMotion` this story used to rely on moves neither —
 * a DOM probe showed the pulse still mounted under it, so the row's evidence was
 * the animated state labelled as the reduced one.
 */
export const GraphReducedMotion: Story = {
  parameters: { reducedMotion: "reduce" },
  globals: { reducedMotion: "reduce" },
  render: () => (
    <MotionConfig reducedMotion="always">
      <RegisterPage scenario={registerRunningScenario()} />
    </MotionConfig>
  ),
  play: openLane("graph"),
};

// VC-25 · the complete roster, healthy rows included.
export const RosterComplete: Story = {
  render: () => <RegisterPage scenario={registerRunningScenario()} />,
  play: openLane("nodes"),
};

// VC-26 · a multi-attempt row naming when its next attempt lands.
export const RosterMultiAttempt: Story = {
  render: () => <RegisterPage scenario={registerRetryingScenario()} />,
  play: openLane("nodes"),
};

// VC-27 · a strategy cancellation reads differently from an operator one.
export const RosterCancelDispositions: Story = {
  render: () => <RegisterPage scenario={vcCancelDispositionsScenario()} />,
  play: openLane("nodes"),
};

// VC-28 · a run that reached no step at all says so.
export const RosterNoSteps: Story = {
  render: () => <RegisterPage scenario={registerNoStepsScenario()} />,
  play: openLane("nodes"),
};

// VC-29 · fan-out workers group under the step that spread them.
export const RosterFanOutGrouped: Story = {
  render: () => <RegisterPage scenario={registerWideFanOutScenario()} />,
  play: openLane("nodes"),
};

// VC-30 · rounds with a score beside rounds the loop never scored.
export const GenerationsScoredAndUnscored: Story = {
  render: () => <RegisterPage scenario={vcGenerationHistoryScenario()} />,
  play: openLane("generations"),
};

// VC-31 · a round the run died inside keeps what it recorded.
export const GenerationsCrashInterrupted: Story = {
  render: () => <RegisterPage scenario={vcCrashInterruptedScenario()} />,
  play: openLane("generations"),
};

// VC-32 · retention took the session; the panel says so instead of 404ing.
export const NodePanelPrunedSession: Story = {
  render: () => (
    <RegisterPage scenario={vcPrunedSessionScenario()} prunedSessionIds={VC_PRUNED_SESSION_IDS} />
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("loop-lane-nodes"));
    await userEvent.click(canvas.getByTestId("loop-roster-row-2:review:0"));
    await expect(canvas.getByTestId("loop-node-panel-degraded-session")).toBeInTheDocument();
  },
};

/**
 * VC-10 · a long run whose story pages back on demand.
 *
 * The scenario stages more than 500 events behind a one-page window, and this
 * wrapper owns the backfill the way the page's query does: each click widens
 * the window rather than re-anchoring it, so paging back never costs the beats
 * already on screen.
 */
function LongStoryPage() {
  const scenario = registerLongStoryScenario();
  const [pageSize, setPageSize] = useState(LONG_STORY_PAGE_SIZE);
  const [inspectOpen, setInspectOpen] = useState(false);
  const [nodeSelection, setNodeSelection] = useState<LoopNodeSelection | null>(null);
  const props = buildScenarioProps({ ...scenario, timelinePageSize: pageSize });
  return (
    <div className="flex h-dvh flex-col bg-canvas">
      <StoryTopbarHost title="Loops">
        <div className="flex min-h-0 flex-1 flex-col bg-canvas">
          <LoopRunPageBody
            {...props}
            inspect={{ open: inspectOpen, onOpenChange: setInspectOpen }}
            nodeSelection={nodeSelection}
            onNodeSelectionChange={setNodeSelection}
            storyPaging={{
              ...props.storyPaging,
              hasOlder: props.storyPaging?.hasOlder ?? false,
              isLoading: false,
              isLoadingOlder: false,
              onLoadOlder: () => setPageSize(current => current + LONG_STORY_PAGE_SIZE),
            }}
          />
        </div>
      </StoryTopbarHost>
    </div>
  );
}

export const StoryPaging: Story = {
  render: () => <LongStoryPage />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const before = canvas.getAllByTestId(/^loop-run-beat-\d+$/).length;
    await userEvent.click(canvas.getByTestId("loop-run-story-load-older"));
    await expect(canvas.getAllByTestId(/^loop-run-beat-\d+$/).length).toBeGreaterThan(before);
  },
};

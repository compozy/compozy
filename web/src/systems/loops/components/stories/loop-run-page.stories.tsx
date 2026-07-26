import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { useTopbarSlot, type TopbarSlotValue } from "@agh/ui";

import { StoryTopbarHost } from "@/storybook/story-layout";

import { LoopRunControls, LoopRunOverflowMenu, LoopRunPageBody, LoopStatusPill } from "../../index";
import {
  buildScenarioProps,
  exhaustedScenario,
  failedScenario,
  needsApprovalScenario,
  noOpScenario,
  pausedScenario,
  runningScenario,
  watchingScenario,
  type LoopRunStoryScenario,
} from "./loop-run-page-fixtures";

/**
 * The redesigned run detail page (LOOP-RUN-REDESIGN-SPEC.md) across its §7
 * states, derived from synthetic frames through the production reducer + libs.
 * These stories are the visual-contract capture targets for the canonical
 * prototypes (`loop-run-detail.html` / `loop-run-detail-states.html`).
 */

function ScenarioPage({
  scenario,
  inspectInitiallyOpen = false,
}: {
  scenario: LoopRunStoryScenario;
  inspectInitiallyOpen?: boolean;
}) {
  const [inspectOpen, setInspectOpen] = useState(inspectInitiallyOpen);
  const props = buildScenarioProps(scenario);
  const topbarIdentity: Pick<TopbarSlotValue, "crumb" | "crumbs" | "onBack"> = {
    onBack: () => {},
    crumbs: [
      { id: "loops", label: "Loops", onSelect: () => {} },
      { id: "runs", label: "Runs", onSelect: () => {} },
    ],
    crumb: props.run.id,
  };

  useTopbarSlot({
    ...topbarIdentity,
    status: <LoopStatusPill status={props.run.status} data-testid="loop-run-status-pill" />,
    actions: (
      <div className="flex items-center gap-2">
        <LoopRunControls
          status={props.run.status}
          pauseRequested={props.run.pause_requested}
          onPause={() => {}}
          onResume={() => {}}
          onStop={() => {}}
        />
        <LoopRunOverflowMenu
          loopName={props.run.loop_name}
          onInspect={() => setInspectOpen(true)}
        />
      </div>
    ),
  });

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-canvas">
      <LoopRunPageBody {...props} inspect={{ open: inspectOpen, onOpenChange: setInspectOpen }} />
    </div>
  );
}

function LoopRunPageStory({
  scenario,
  inspectInitiallyOpen = false,
}: Parameters<typeof ScenarioPage>[0]) {
  return (
    <div className="flex h-dvh flex-col bg-canvas">
      <StoryTopbarHost title="Loops">
        <ScenarioPage scenario={scenario} inspectInitiallyOpen={inspectInitiallyOpen} />
      </StoryTopbarHost>
    </div>
  );
}

const meta: Meta<typeof ScenarioPage> = {
  title: "systems/loops/components/LoopRunPage",
  component: ScenarioPage,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Running: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={runningScenario()} />,
};

export const NeedsApproval: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={needsApprovalScenario()} />,
};

export const Watching: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={watchingScenario()} />,
};

export const Paused: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={pausedScenario()} />,
};

export const Failed: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={failedScenario()} />,
};

export const Exhausted: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={exhaustedScenario()} />,
};

export const NoOp: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={noOpScenario()} />,
};

export const InspectOpen: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={runningScenario()} inspectInitiallyOpen />,
};

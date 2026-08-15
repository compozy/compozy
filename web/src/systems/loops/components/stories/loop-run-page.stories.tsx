import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { useTopbarSlot, type TopbarSlotValue } from "@compozy/ui";

import { StoryTopbarHost } from "@/storybook/story-layout";

import {
  LoopNodeRowActions,
  LoopRunControls,
  LoopRunOverflowMenu,
  LoopRunPageBody,
  LoopStatusPill,
} from "../../index";
import {
  attentionScenario,
  canceledScenario,
  killedScenario,
  parkedProgressScenario,
  pausedByRuleScenario,
  pausedNodeScenario,
  quarantinedScenario,
  retryingScenario,
  waitingScenario,
} from "./loop-run-lifecycle-fixtures";
import {
  buildScenarioProps,
  exhaustedScenario,
  failedScenario,
  needsApprovalScenario,
  noOpScenario,
  pausedScenario,
  ratchetRestoreScenario,
  runningScenario,
  scoredBestScenario,
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
      { id: "loop", label: props.run.loop_name, onSelect: () => {} },
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
          onCancel={() => {}}
        />
        <LoopRunOverflowMenu loopName={props.run.loop_name} onKill={() => {}} />
      </div>
    ),
  });

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-canvas">
      <LoopRunPageBody
        {...props}
        inspect={{ open: inspectOpen, onOpenChange: setInspectOpen }}
        renderNodeActions={node => (
          <LoopNodeRowActions node={node} onVerb={() => {}} runStatus={props.run.status} />
        )}
      />
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

export const ScoredBestGeneration: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={scoredBestScenario()} />,
};

export const RatchetRestore: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={ratchetRestoreScenario()} />,
};

export const NoMetric: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={runningScenario()} />,
};

export const ExhaustedBestGeneration: Story = {
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

// Node lifecycle states (Spec 1). These are the Visual Contract capture targets
// for VC-R1 (Retrying) and VC-R2 (the rest).
export const Retrying: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={retryingScenario()} />,
};

export const NodePaused: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={pausedNodeScenario()} />,
};

export const NodePausedByRule: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={pausedByRuleScenario()} />,
};

export const Waiting: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={waitingScenario()} />,
};

export const Quarantined: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={quarantinedScenario()} />,
};

export const NeedsAttention: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={attentionScenario()} />,
};

export const Canceled: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={canceledScenario()} />,
};

export const Killed: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={killedScenario()} />,
};

export const ParkedProgress: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={parkedProgressScenario()} />,
};

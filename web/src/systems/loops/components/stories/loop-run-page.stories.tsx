import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { useTopbarSlot, type TopbarSlotValue } from "@compozy/ui";

import { StoryTopbarHost } from "@/storybook/story-layout";

import type { LoopRunRequestState } from "../run-page/requests/loop-request-questionnaire";
import {
  type LoopNodeSelection,
  LoopRunControls,
  LoopRunOverflowMenu,
  LoopRunPageBody,
  LoopStatusPill,
} from "../../index";
import {
  forkedRunScenario,
  laneRequestsScenario,
  pendingEnumRequestScenario,
  pendingRequestsScenario,
  redactedRequestScenario,
  resolvedRequestsScenario,
} from "./loop-run-graph-eng-fixtures";
import {
  registerDoneScenario,
  registerFailedScenario,
  registerLongStoryScenario,
  registerNeedsYouScenario,
  registerNoStepsScenario,
  registerPartialOutputsScenario,
  registerPrunedArtifactScenario,
  registerRetryingScenario,
  registerRoutedScenario,
  registerRunningScenario,
  registerWideFanOutScenario,
} from "./loop-run-register-fixtures";
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
import { pendingReviewRequest } from "../../mocks";
import {
  exhaustedScenario,
  ratchetRestoreScenario,
  scoredBestScenario,
} from "./loop-run-metric-fixtures";
import {
  buildScenarioProps,
  failedScenario,
  needsApprovalScenario,
  noOpScenario,
  pausedScenario,
  runningScenario,
  watchingScenario,
  type LoopRunStoryScenario,
} from "./loop-run-page-fixtures";

function ScenarioPage({
  scenario,
  inspectInitiallyOpen = false,
  requestState,
  prunedSessionIds,
}: {
  scenario: LoopRunStoryScenario;
  inspectInitiallyOpen?: boolean;
  requestState?: LoopRunRequestState;
  /** Stages the retention degrade the live page reads from the session store. */
  prunedSessionIds?: ReadonlySet<string>;
}) {
  const [inspectOpen, setInspectOpen] = useState(inspectInitiallyOpen);
  const [nodeSelection, setNodeSelection] = useState<LoopNodeSelection | null>(null);
  const props = buildScenarioProps(scenario);
  const topbarIdentity: Pick<TopbarSlotValue, "crumb" | "crumbs" | "onBack"> = {
    onBack: () => {},
    crumbs: [
      { id: "loops", label: "Loops", onSelect: () => {} },
      { id: "runs", label: "Runs", onSelect: () => {} },
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
        nodeSelection={nodeSelection}
        onNodeSelectionChange={setNodeSelection}
        prunedSessionIds={prunedSessionIds}
        requestState={requestState}
      />
    </div>
  );
}

function LoopRunPageStory(props: Parameters<typeof ScenarioPage>[0]) {
  return (
    <div className="flex h-dvh flex-col bg-canvas">
      <StoryTopbarHost title="Loops">
        <ScenarioPage {...props} />
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

const ASK_KEY = "3:confirm-rollout:0";
const REVIEW_KEY = "3:apply-migration:0";

export const PendingRequests: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={pendingRequestsScenario()} />,
};

export const PendingEnumRequest: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={pendingEnumRequestScenario()} />,
};

export const RequestValidationFailure: Story = {
  args: {},
  render: () => (
    <LoopRunPageStory
      requestState={{
        engagedKey: ASK_KEY,
        fieldErrors: { regions: "At least one region is required." },
      }}
      scenario={pendingRequestsScenario()}
    />
  ),
};

export const RequestAlreadyAnswered: Story = {
  args: {},
  render: () => (
    <LoopRunPageStory
      requestState={{
        engagedKey: REVIEW_KEY,
        refusal: "Someone already answered this request.",
      }}
      scenario={pendingRequestsScenario()}
    />
  ),
};

export const RequestAnswerPending: Story = {
  args: {},
  render: () => (
    <LoopRunPageStory
      requestState={{ engagedKey: ASK_KEY, isAnswerPending: true }}
      scenario={pendingRequestsScenario()}
    />
  ),
};

export const RepeatedGenerationRequests: Story = {
  args: {},
  render: () => {
    const scenario = pendingRequestsScenario();
    return (
      <LoopRunPageStory
        requestState={{
          engagedKey: REVIEW_KEY,
          fullContextError: "Context is temporarily unavailable",
          onRequestFullContext: () => {},
        }}
        scenario={{
          ...scenario,
          requests: [
            { ...pendingReviewRequest, generation: 2 },
            { ...pendingReviewRequest, generation: 3 },
          ],
        }}
      />
    );
  },
};

export const ResolvedRequests: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={resolvedRequestsScenario()} />,
};

export const RedactedRequestContext: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={redactedRequestScenario()} />,
};

export const LaneRequests: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={laneRequestsScenario()} />,
};

/** VC-08: a terminal failure whose partial output is labelled as partial. */
export const PartialCompletion: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerPartialOutputsScenario()} />,
};

export const ForkLineage: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={forkedRunScenario()} inspectInitiallyOpen />,
};

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

/**
 * Visual-contract states for the two-register redesign (task_05 VC rows).
 *
 * Each stages the three run reads and travels the production projection, so the
 * screenshot bundle captured from it is evidence about the real derivation path
 * rather than about a hand-built view model.
 */
export const RegisterRunning: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerRunningScenario()} />,
};

export const RegisterNeedsYou: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerNeedsYouScenario()} />,
};

export const RegisterDone: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerDoneScenario()} />,
};

export const RegisterPrunedArtifact: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerPrunedArtifactScenario()} />,
};

export const RegisterFailed: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerFailedScenario()} />,
};

export const RegisterLongStory: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerLongStoryScenario()} />,
};

export const RegisterGraphOpen: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerRunningScenario()} inspectInitiallyOpen />,
};

export const RegisterRoutedGraph: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerRoutedScenario()} inspectInitiallyOpen />,
};

export const RegisterWideFanOut: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerWideFanOutScenario()} inspectInitiallyOpen />,
};

export const RegisterRetryingRoster: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerRetryingScenario()} inspectInitiallyOpen />,
};

export const RegisterNoSteps: Story = {
  args: {},
  render: () => <LoopRunPageStory scenario={registerNoStepsScenario()} inspectInitiallyOpen />,
};

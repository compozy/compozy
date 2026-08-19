import type { ComponentProps, ReactNode } from "react";
import { Search } from "lucide-react";

import { Button, cn } from "@compozy/ui";

import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import type { LoopNodeNowLine } from "../../lib/loop-node-now-view";
import { LoopRunAttentionPanel, LoopRunWaitingPanel } from "./loop-run-parked-panels";
import { LoopRunWaitsRail } from "./loop-run-waits-rail";

import type {
  LoopApprovalFact,
  LoopApprovalRequest,
  LoopCoordinatorFailure,
  LoopGateDecision,
  LoopGateVerdict,
} from "../../lib/loop-events";
import type { LoopGraph } from "../../lib/loop-graph";
import type { LoopRunInputRow } from "../../lib/loop-run-about";
import type { LoopRequestView } from "../../lib/loop-request-model";
import type { LoopRunProgressModel } from "../../lib/loop-run-progress";
import type { LoopStrategyProgressModel } from "../../lib/loop-run-strategy";
import type { LoopRunStory } from "../../lib/loop-run-story";
import type { LoopRunUsageRow } from "../../lib/loop-run-usage";
import type { GoalTurnTimelineItem } from "../../hooks/use-goal-turns";
import type {
  LoopDefinition,
  LoopContract,
  LoopRunEventFrame,
  LoopRunGeneration,
  LoopRunRecord,
  LoopRunStatus,
  LoopWatchEventsState,
} from "../../types";
import { LoopRunAboutRail } from "./loop-run-about-rail";
import { LoopRunInspectSheet } from "./loop-run-inspect-sheet";
import { LoopRunNeedsYouCard } from "./loop-run-needs-you-card";
import type { LoopRunRequestState } from "./requests/loop-request-questionnaire";
import { LoopRunNextNote } from "./loop-run-next-note";
import { LoopRunNowCard } from "./loop-run-now-card";
import { LoopRunOutcomeCard } from "./loop-run-outcome-card";
import { LoopRunProgressPanel } from "./loop-run-progress-panel";
import { LoopStrategyProgress } from "./loop-strategy-progress";
import { LoopRunStoryTimeline } from "./loop-run-story-timeline";
import { LoopRunTurnsDisclosure } from "./loop-run-turns-disclosure";
import { LoopRunUsageRail } from "./loop-run-usage-rail";

const OUTCOME_STATUSES = new Set<LoopRunStatus>([
  "failed",
  "blocked",
  "exhausted",
  "stalled",
  "no-op",
  // `canceled` is a deliberate ending, not a failure — it renders the same
  // "Why it stopped" slot with calm neutral tone (VC-R2).
  "canceled",
]);

const NO_REQUESTS: readonly LoopRequestView[] = [];
const NO_STRATEGY_PROGRESS: readonly LoopStrategyProgressModel[] = [];

export interface LoopRunGoalTurnsPaging {
  hasMore: boolean;
  isLoading: boolean;
  onLoadMore?: () => void;
}

export interface LoopRunInspectState {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** The one in-flight page action; the matching control disables while set. */
export type LoopRunPendingAction = "approve" | "start-new-run";

export interface LoopRunPageBodyProps extends Omit<ComponentProps<"div">, "children"> {
  run: LoopRunRecord;
  definition?: LoopDefinition;
  materializedContract: LoopContract;
  graph: LoopGraph | null;
  isLive: boolean;
  stepElapsedLabel: string | null;
  progress: LoopRunProgressModel;
  story: LoopRunStory;
  goalIds: ReadonlySet<string>;
  goalTurns: readonly GoalTurnTimelineItem[];
  goalTurnsPaging?: LoopRunGoalTurnsPaging;
  usageRows: LoopRunUsageRow[];
  usageNote: string | null;
  approvalRequest: LoopApprovalRequest | null;
  approvalFallbackFacts: LoopApprovalFact[];
  failure: LoopCoordinatorFailure | null;
  latestVerdict: LoopGateVerdict | null;
  watchEvents?: LoopWatchEventsState;
  watchCadence: string | null;
  generations: readonly LoopRunGeneration[];
  frames: readonly LoopRunEventFrame[];
  inputRows: LoopRunInputRow[];
  startedBy: string;
  workspaceLabel: string;
  workspaceId?: string;
  /** `v3 · pinned` when the run pins its executed definition. */
  versionLabel?: string;
  nextNote: string | null;
  showNowCard: boolean;
  terminalFromStatus?: string;
  terminalAt?: string;
  /** The terminal transition's `cause` — separates a cancel from a kill. */
  terminalCause?: string;
  inspect: LoopRunInspectState;
  pendingAction?: LoopRunPendingAction;
  /** Lifecycle lines rendered inside the Happening-now card. */
  nodeNowLines?: readonly LoopNodeNowLine[];
  /** Nodes holding an open wait cell. */
  waitingNodes?: readonly LoopNodeLifecycle[];
  /** Nodes carrying an attention flag. */
  attentionNodes?: readonly LoopNodeLifecycle[];
  /** Every node with declared lifecycle state; feeds the rail's waits panel. */
  nodeLifecycles?: readonly LoopNodeLifecycle[];
  /** Lifecycle rows by node id, so a line can host its verb menu. */
  nodesById?: ReadonlyMap<string, LoopNodeLifecycle>;
  /** Renders the verb menu for one node; omitted in read-only fixtures. */
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;
  /** ACP session per node; the Happening-now rows link straight into them. */
  nodeSessions?: ReadonlyMap<string, string>;
  /** Opens the quarantine entry sheet for the quarantined node id. */
  onOpenQuarantine?: (nodeId: string) => void;
  onDecision: (decision: LoopGateDecision, gateId: string) => void;
  onStartNewRun: () => void;

  requests?: readonly LoopRequestView[];
  requestFocus?: { generation?: number; nodeId: string; itemIndex: number };

  requestState?: LoopRunRequestState;

  strategyProgress?: readonly LoopStrategyProgressModel[];

  onOpenRun?: (runId: string) => void;

  onCompareGeneration?: (generation: number) => void;

  onForkGeneration?: (generation: number) => void;
}

/** `sha256:4f9c2a1…` → `4f9c2a1` for the rail foot. */
function shortDigest(digest: string): string {
  return digest.replace(/^sha256:/, "").slice(0, 7);
}

export function LoopRunPageBody({
  run,
  definition,
  materializedContract,
  graph,
  isLive,
  stepElapsedLabel,
  progress,
  story,
  goalIds,
  goalTurns,
  goalTurnsPaging,
  usageRows,
  usageNote,
  approvalRequest,
  approvalFallbackFacts,
  failure,
  latestVerdict,
  watchEvents,
  watchCadence,
  generations,
  frames,
  inputRows,
  startedBy,
  workspaceLabel,
  workspaceId = "",
  versionLabel,
  nextNote,
  showNowCard,
  terminalFromStatus,
  terminalAt,
  terminalCause,
  inspect,
  pendingAction,
  nodeNowLines,
  waitingNodes,
  attentionNodes,
  nodeLifecycles,
  nodesById,
  renderNodeActions,
  nodeSessions,
  onOpenQuarantine,
  onDecision,
  onStartNewRun,
  requests = NO_REQUESTS,
  requestFocus,
  requestState,
  strategyProgress = NO_STRATEGY_PROGRESS,
  onOpenRun,
  onCompareGeneration,
  onForkGeneration,
  className,
  ...divProps
}: LoopRunPageBodyProps) {
  const status = run.status;
  const contract = materializedContract;
  const quarantinedNodes = (nodeLifecycles ?? []).filter(node => node.quarantined);

  const requestKinds = new Map(
    requests.map(view => [
      `${view.request.generation}:${view.request.node_id}:${view.request.item_index}`,
      view.request.kind,
    ])
  );

  const pendingRequestCount = requests.filter(
    view => view.state === "pending" && view.isAnswerable
  ).length;
  const nowTurnsSlot =
    story.now?.isGoalNode === true ? (
      <LoopRunTurnsDisclosure
        hasMore={goalTurnsPaging?.hasMore}
        isLoadingMore={goalTurnsPaging?.isLoading}
        isLive={isLive}
        onLoadMore={goalTurnsPaging?.onLoadMore}
        turns={goalTurns.filter(turn => turn.nodeId === story.now?.nodeId)}
      />
    ) : undefined;

  // A terminal run has nothing happening: `showNowCard` already excludes every
  // terminal status, so a canceled run renders no live card even while its node
  // lifecycle rows still exist in the projection (VC-R2).
  const nowCard = showNowCard ? (
    <LoopRunNowCard
      run={run}
      now={story.now}
      watchLastWakeAt={watchEvents?.last_wake_at ?? undefined}
      watchCadence={watchCadence}
      stepElapsedLabel={stepElapsedLabel}
      isLive={isLive}
      nodeLines={nodeNowLines}
      nodesById={nodesById}
      renderNodeActions={renderNodeActions}
      nodeSessions={nodeSessions}
    >
      {nowTurnsSlot}
    </LoopRunNowCard>
  ) : null;

  return (
    <div
      className={cn("flex min-h-0 flex-1 flex-col overflow-y-auto", className)}
      data-testid="loop-run-detail-content"
      {...divProps}
    >
      <div className="mx-auto w-full max-w-[1240px] px-9 pt-6 pb-18 max-[1080px]:px-5">
        <div className="grid grid-cols-1 items-start gap-8 min-[1080px]:grid-cols-[minmax(0,1fr)_320px]">
          <main className="flex min-w-0 flex-col gap-6.5">
            {status === "needs-approval" || quarantinedNodes.length > 0 || requests.length > 0 ? (
              <LoopRunNeedsYouCard
                fallbackFacts={approvalFallbackFacts}
                isPending={pendingAction === "approve"}
                onDecision={onDecision}
                onOpenQuarantine={onOpenQuarantine}
                quarantinedNodes={quarantinedNodes}
                request={approvalRequest}
                requestState={requestState}
                requestFocus={requestFocus}
                requests={requests}
                run={run}
                showApproval={status === "needs-approval"}
                workspaceId={workspaceId}
              />
            ) : null}
            <LoopRunAttentionPanel
              nodes={attentionNodes ?? []}
              onOpenQuarantine={onOpenQuarantine}
              renderNodeActions={renderNodeActions}
              runId={run.id}
            />
            {OUTCOME_STATUSES.has(status) ? (
              <LoopRunOutcomeCard
                run={run}
                failure={failure}
                fromStatus={terminalFromStatus}
                terminalAt={terminalAt}
                cause={terminalCause}
                noProgressWindow={contract.no_progress.window}
                repeatedIssueIds={latestVerdict?.blockingIssues.map(issue => issue.id) ?? []}
                onStartNewRun={onStartNewRun}
                isStartPending={pendingAction === "start-new-run"}
              />
            ) : null}
            {status === "paused" ? nowCard : null}
            <LoopRunProgressPanel
              title={contract.goal}
              doneWhen={contract.definition_of_done}
              progress={progress}
            />
            <LoopStrategyProgress models={strategyProgress} />
            {status !== "paused" ? nowCard : null}
            <LoopRunWaitingPanel
              nodes={waitingNodes ?? []}
              renderNodeActions={renderNodeActions}
              requestKinds={requestKinds}
              runId={run.id}
            />
            <LoopRunStoryTimeline
              rows={story.rows}
              isLive={isLive}
              goalNodeIds={goalIds}
              goalTurns={goalTurns}
              hasMoreGoalTurns={goalTurnsPaging?.hasMore}
              isLoadingMoreGoalTurns={goalTurnsPaging?.isLoading}
              onLoadMoreGoalTurns={goalTurnsPaging?.onLoadMore}
            />
            <LoopRunNextNote note={nextNote} />
          </main>
          <aside data-testid="loop-run-detail-rail">
            <div className="rounded-lg border border-line bg-canvas-soft">
              <LoopRunUsageRail rows={usageRows} note={usageNote} />
              {nodeLifecycles && nodeLifecycles.length > 0 ? (
                <LoopRunWaitsRail
                  nodes={nodeLifecycles}
                  pendingRequests={pendingRequestCount}
                  runId={run.id}
                />
              ) : null}
              <LoopRunAboutRail
                run={run}
                versionLabel={versionLabel}
                inputRows={inputRows}
                startedBy={startedBy}
                workspaceLabel={workspaceLabel}
              />
              <div className="flex items-center justify-between border-t border-line-soft px-3 py-2.5">
                <Button
                  className="min-h-6"
                  data-testid="loop-run-open-inspect"
                  onClick={() => inspect.onOpenChange(true)}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  <Search aria-hidden="true" className="size-3" />
                  Inspect
                </Button>
                {run.definition_digest ? (
                  <span className="font-mono text-pill-group-badge text-faint">
                    digest {shortDigest(run.definition_digest)}
                  </span>
                ) : null}
              </div>
            </div>
          </aside>
        </div>
      </div>
      <LoopRunInspectSheet
        open={inspect.open}
        onOpenChange={inspect.onOpenChange}
        run={run}
        definition={definition}
        graph={graph}
        latestVerdict={latestVerdict}
        watchEvents={watchEvents}
        generations={generations}
        frames={frames}
        onOpenRun={onOpenRun}
        onCompareGeneration={onCompareGeneration}
        onForkGeneration={onForkGeneration}
      />
    </div>
  );
}

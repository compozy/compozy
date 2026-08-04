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
import type { LoopRunProgressModel } from "../../lib/loop-run-progress";
import type { LoopRunStory } from "../../lib/loop-run-story";
import type { LoopRunUsageRow } from "../../lib/loop-run-usage";
import type { GoalTurnTimelineItem } from "../../hooks/use-goal-turns";
import type {
  LoopDefinition,
  LoopRunEventFrame,
  LoopRunGeneration,
  LoopRunRecord,
  LoopRunStatus,
  LoopWatchEventsState,
} from "../../types";
import { LoopRunAboutRail } from "./loop-run-about-rail";
import { LoopRunInspectSheet } from "./loop-run-inspect-sheet";
import { LoopRunNeedsYouCard } from "./loop-run-needs-you-card";
import { LoopRunNextNote } from "./loop-run-next-note";
import { LoopRunNowCard } from "./loop-run-now-card";
import { LoopRunOutcomeCard } from "./loop-run-outcome-card";
import { LoopRunProgressPanel } from "./loop-run-progress-panel";
import { LoopRunStoryTimeline } from "./loop-run-story-timeline";
import { LoopRunSubhead } from "./loop-run-subhead";
import { LoopRunTurnsDisclosure } from "./loop-run-turns-disclosure";
import { LoopRunUsageRail } from "./loop-run-usage-rail";

/** Statuses that render the terminal extra card at the top of the main column (§7). */
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
  graph: LoopGraph | null;
  isLive: boolean;
  subject: string | null;
  hasWatchSource: boolean;
  elapsedLabel: string;
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
  /** Opens the quarantine entry sheet for one node. */
  onOpenQuarantine?: (node: LoopNodeLifecycle) => void;
  onDecision: (decision: LoopGateDecision, gateId: string) => void;
  onStartNewRun: () => void;
}

/** `sha256:4f9c2a1…` → `4f9c2a1` for the rail foot. */
function shortDigest(digest: string): string {
  return digest.replace(/^sha256:/, "").slice(0, 7);
}

/**
 * The redesigned run page body (§2 anatomy): subhead, the status-gated main
 * column — Needs you / outcome card / Progress / Happening now / What happened /
 * What happens next — and the 320px Usage + About rail with the Inspect foot.
 * Purely presentational; the location wires the view-model hook into it.
 */
export function LoopRunPageBody({
  run,
  definition,
  graph,
  isLive,
  subject,
  hasWatchSource,
  elapsedLabel,
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
  onOpenQuarantine,
  onDecision,
  onStartNewRun,
  className,
  ...divProps
}: LoopRunPageBodyProps) {
  const status = run.status;
  const contract = definition?.contract;
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
        <LoopRunSubhead
          run={run}
          subject={subject}
          hasWatchSource={hasWatchSource}
          elapsedLabel={elapsedLabel}
        />
        <div className="mt-5.5 grid grid-cols-1 items-start gap-8 min-[1080px]:grid-cols-[minmax(0,1fr)_320px]">
          <main className="flex min-w-0 flex-col gap-6.5">
            {status === "needs-approval" ? (
              <LoopRunNeedsYouCard
                run={run}
                request={approvalRequest}
                fallbackFacts={approvalFallbackFacts}
                isPending={pendingAction === "approve"}
                onDecision={onDecision}
              />
            ) : null}
            <LoopRunAttentionPanel
              nodes={attentionNodes ?? []}
              onOpenQuarantine={onOpenQuarantine}
              renderNodeActions={renderNodeActions}
            />
            {OUTCOME_STATUSES.has(status) ? (
              <LoopRunOutcomeCard
                run={run}
                failure={failure}
                fromStatus={terminalFromStatus}
                terminalAt={terminalAt}
                cause={terminalCause}
                noProgressWindow={contract?.no_progress.window}
                repeatedIssueIds={latestVerdict?.blockingIssues.map(issue => issue.id) ?? []}
                onStartNewRun={onStartNewRun}
                isStartPending={pendingAction === "start-new-run"}
              />
            ) : null}
            {status === "paused" ? nowCard : null}
            <LoopRunProgressPanel
              title={contract?.goal ?? `Run ${run.loop_name}`}
              doneWhen={contract?.definition_of_done}
              progress={progress}
            />
            {status !== "paused" ? nowCard : null}
            <LoopRunWaitingPanel nodes={waitingNodes ?? []} renderNodeActions={renderNodeActions} />
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
                <LoopRunWaitsRail nodes={nodeLifecycles} />
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
      />
    </div>
  );
}

import type { GoalTurnsRead } from "../../hooks/use-goal-turns";
import { useState } from "react";
import type { ComponentProps, ReactNode } from "react";
import { Search } from "@compozy/ui/icons";

import { Button, SkeletonRegion } from "@compozy/ui";

import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";

import type {
  LoopApprovalFact,
  LoopApprovalRequest,
  LoopGateDecision,
} from "../../lib/loop-events";
import type { LoopGraph } from "../../lib/loop-graph";
import type { LoopRunInputRow } from "../../lib/loop-run-about";
import type { LoopRequestView } from "../../lib/loop-request-model";
import type { LoopRunUsageRow } from "../../lib/loop-run-usage";
import type {
  LoopContract,
  LoopRunGeneration,
  LoopRunRecord,
  LoopWatchEventsState,
} from "../../types";
import type {
  LoopNodeSelection,
  LoopRunRegisters as LoopRunRegistersModel,
} from "../../lib/loop-run-registers-view";
import type { LoopInspectLane } from "./inspect/loop-run-inspect-register";
import type { LoopFanoutRollup, LoopRosterNode } from "../../types";
import { LoopRunAboutRail } from "./loop-run-about-rail";
import { LoopRunBriefing } from "./loop-run-briefing";
import { LoopRunPageFrame } from "./loop-run-page-frame";
import { RAIL_CARD_CLASS } from "./loop-run-page-classes";
import { LoopRunBriefingSkeleton, LoopRunStepsProgressSkeleton } from "./loop-run-page-skeleton";
import { LoopRunChildReadContext } from "../../hooks/use-loop-run-child-read";
import { LOOP_NEEDS_YOU_ANCHOR_ID } from "./loop-run-briefing-constants";
import { LoopRunRegisters } from "./loop-run-registers";
import type { LoopRunEventsRead } from "./inspect/loop-run-events-lane";
import { LoopRunStepsProgress } from "./loop-run-steps-progress";
import { LoopRunStory } from "./loop-run-story";
export type { LoopRunStoryPaging } from "./loop-run-story";
import { LoopRunLineageSection } from "./loop-run-lineage-section";
import { LoopRunNeedsYouCard } from "./loop-run-needs-you-card";
import type { LoopRunStoryPaging } from "./loop-run-story";
import type { LoopRunRequestState } from "./requests/loop-request-questionnaire";
import { LoopRunUsageRail } from "./loop-run-usage-rail";

const NO_REQUESTS: readonly LoopRequestView[] = [];
/** One stable identity, so an absent paging read is not a new object each render. */
const NO_PAGING: LoopRunStoryPaging = {
  hasOlder: false,
  isLoading: false,
  isError: false,
  isLoadingOlder: false,
  onLoadOlder: () => {},
};

/**
 * Whether the roster read has actually answered yet.
 *
 * Without this the Inspect lanes cannot tell "this run reached no steps" from
 * "we have not read its steps", and both render as `No steps ran`.
 */
export interface LoopRunRosterRead {
  isLoading: boolean;
  isError: boolean;
}

/**
 * Whether the briefing read has answered yet. Until it does, the strip and the progress card it
 * feeds show their skeletons instead of being absent, so they do not push the page down later.
 */
export interface LoopRunBriefingRead {
  isLoading: boolean;
}

export interface LoopRunGoalTurnsPaging {
  hasMore: boolean;
  isLoading: boolean;
  onLoadMore?: () => void;
}

export interface LoopRunInspectState {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * The one in-flight page action; the matching control disables while set.
 *
 * Only `approve` — the redesigned body has no start-a-new-run control, so a
 * pending state for it would be a value no element could ever spend.
 */
export type LoopRunPendingAction = "approve";

export interface LoopRunPageBodyProps extends Omit<ComponentProps<"div">, "children"> {
  goalTurns?: GoalTurnsRead;
  run: LoopRunRecord;
  materializedContract: LoopContract;
  graph: LoopGraph | null;
  isLive: boolean;
  /** The clock in-progress rows measure against; stories pin it for capture. */
  nowMs: number;
  /** Both registers, projected from the briefing, roster and timeline reads. */
  registers: LoopRunRegistersModel;
  rosterNodes: readonly LoopRosterNode[];
  rosterRollups: readonly LoopFanoutRollup[];
  /** Pulls the next block of roster pages past the page's own budget. */
  onLoadMoreRoster?: () => void;
  isLoadingMoreRoster?: boolean;
  /** The durable story's backward paging; the live tail arrives by stream. */
  storyPaging?: LoopRunStoryPaging;
  /** Read state for the roster, so Inspect never calls a pending read empty. */
  rosterRead?: LoopRunRosterRead;
  /** Read state for the briefing, so the strip and progress hold their place while it arrives. */
  briefingRead?: LoopRunBriefingRead;
  /**
   * The Events lane's raw activity read (`view=all`), when the page has one.
   *
   * Absent means the lane borrows Story's notable projection and says so; it
   * never presents a filtered subset as the whole event log.
   */
  events?: Omit<LoopRunEventsRead, "view">;
  /** True when the stream dropped: the page keeps its last reconciled read. */
  isReconnecting?: boolean;
  usageRows: LoopRunUsageRow[];
  usageNote: string | null;
  approvalRequest: LoopApprovalRequest | null;
  approvalFallbackFacts: LoopApprovalFact[];
  generations: readonly LoopRunGeneration[];
  inputRows: LoopRunInputRow[];
  startedBy: string;
  workspaceLabel: string;
  workspaceId?: string;
  /** `v3 · pinned` when the run pins its executed definition. */
  versionLabel?: string;
  watchEvents?: LoopWatchEventsState | null;
  inspect: LoopRunInspectState;
  pendingAction?: LoopRunPendingAction;
  /** Every node with declared lifecycle state; feeds the rail's waits panel. */
  nodeLifecycles?: readonly LoopNodeLifecycle[];
  /** Which node the operator register has open, owned by the page. */
  nodeSelection: LoopNodeSelection | null;
  onNodeSelectionChange: (selection: LoopNodeSelection | null) => void;
  /** Sessions retention removed; only the open node's is ever known. */
  prunedSessionIds?: ReadonlySet<string>;
  /** Renders the verb cluster for a node opened in the operator register. */
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;
  /** Opens the quarantine entry sheet for the quarantined node id. */
  onOpenQuarantine?: (nodeId: string) => void;
  onDecision: (decision: LoopGateDecision, gateId: string) => void;

  requests?: readonly LoopRequestView[];
  requestFocus?: { generation?: number; nodeId: string; itemIndex: number };

  requestState?: LoopRunRequestState;

  onCompareGeneration?: (generation: number) => void;

  onForkGeneration?: (generation: number) => void;
}

/** Composes run content from the page model while keeping data reads and controls with its owner. */
export function LoopRunPageBody({
  goalTurns,
  run,
  materializedContract,
  graph,
  isLive,
  nowMs,
  registers,
  rosterNodes,
  rosterRollups,
  onLoadMoreRoster,
  isLoadingMoreRoster = false,
  storyPaging,
  rosterRead,
  briefingRead,
  events,
  isReconnecting = false,
  usageRows,
  usageNote,
  approvalRequest,
  approvalFallbackFacts,
  generations,
  inputRows,
  startedBy,
  workspaceLabel,
  workspaceId = "",
  versionLabel,
  watchEvents,
  inspect,
  pendingAction,
  nodeLifecycles,
  nodeSelection,
  onNodeSelectionChange,
  prunedSessionIds,
  renderNodeActions,
  onOpenQuarantine,
  onDecision,
  requests = NO_REQUESTS,
  requestFocus,
  requestState,
  onCompareGeneration,
  onForkGeneration,
  ...frameProps
}: LoopRunPageBodyProps) {
  const [inspectLane, setInspectLane] = useState<LoopInspectLane>("graph");
  // The briefing and the progress card exist only once the briefing has been read.
  const briefingPending = briefingRead?.isLoading === true && registers.briefing === null;
  const status = run.status;
  const contract = materializedContract;
  const quarantinedNodes = (nodeLifecycles ?? []).filter(node => node.quarantined);

  return (
    // Child runs surface in Progress, the graph and the node panel; they read
    // the workspace and the page clock from here rather than through each register.
    <LoopRunChildReadContext value={{ workspaceId, nowMs, clockLive: isLive }}>
      <LoopRunPageFrame
        data-testid="loop-run-detail-content"
        {...frameProps}
        main={
          <>
            {/* Four elements, in order, and nothing competing with them. Failure
              and needs-you render here whatever is collapsed below: a signal
              you have to expand to see is a signal you will miss. */}
            {registers.briefing ? (
              <LoopRunBriefing
                briefing={registers.briefing}
                onOpenInspect={() => inspect.onOpenChange(true)}
                outcome={registers.outcome}
              />
            ) : briefingPending ? (
              <SkeletonRegion label="Loading run status">
                <LoopRunBriefingSkeleton />
              </SkeletonRegion>
            ) : null}
            {status === "needs-approval" || quarantinedNodes.length > 0 || requests.length > 0 ? (
              // The anchor, and only the anchor. `LoopRunNeedsYouCard` owns the
              // `loop-run-needs-you` test id and the labelled region; repeating
              // either here would give strict selectors two matching nodes and
              // screen readers two regions with one name.
              <section id={LOOP_NEEDS_YOU_ANCHOR_ID} tabIndex={-1}>
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
              </section>
            ) : null}
            {registers.progress ? (
              <LoopRunStepsProgress
                doneWhen={contract.definition_of_done}
                goal={contract.goal}
                progress={registers.progress}
                reach={registers.reach}
                stepsPending={rosterRead?.isLoading === true}
              />
            ) : briefingPending ? (
              <LoopRunStepsProgressSkeleton />
            ) : null}
            <LoopRunStory
              beats={registers.beats}
              isReconnecting={isReconnecting}
              paging={storyPaging ?? NO_PAGING}
            />
            {/* A forked or time-travelled run is part of the story: the point
              it branched at is recorded here, and it links the related run so
              the reader can follow it (US-009.EC-3). */}
            <LoopRunLineageSection forkedFrom={run.forked_from ?? null} forks={run.forks} />
            {/* Everything the default read demoted lives one disclosure down. */}
            <LoopRunRegisters
              goalTurns={goalTurns}
              bestGeneration={run.best_generation}
              generations={generations}
              nodeLifecycles={nodeLifecycles ?? []}
              isLoadingMoreRoster={isLoadingMoreRoster}
              onLoadMoreRoster={onLoadMoreRoster}
              renderNodeActions={renderNodeActions}
              graph={graph}
              isLive={isLive}
              isReconnecting={isReconnecting}
              nowMs={nowMs}
              runStatus={status}
              nodes={rosterNodes}
              onCompareGeneration={onCompareGeneration}
              onForkGeneration={onForkGeneration}
              onOpenChange={inspect.onOpenChange}
              onSelectionChange={onNodeSelectionChange}
              open={inspect.open}
              prunedSessionIds={prunedSessionIds}
              registers={registers}
              rollups={rosterRollups}
              events={events}
              rosterRead={rosterRead}
              selection={nodeSelection}
              lane={inspectLane}
              onLaneChange={setInspectLane}
              watchEvents={watchEvents}
            />
          </>
        }
        rail={
          <div className={RAIL_CARD_CLASS}>
            <LoopRunUsageRail rows={usageRows} note={usageNote} />
            <LoopRunAboutRail
              run={run}
              versionLabel={versionLabel}
              inputRows={inputRows}
              startedBy={startedBy}
              workspaceLabel={workspaceLabel}
              lastWakeAt={watchEvents?.last_wake_at}
              onOpenGeneration={() => {
                setInspectLane("generations");
                inspect.onOpenChange(true);
              }}
            />
            <div className="flex items-center border-t border-line-soft px-3 py-2">
              <Button
                data-testid="loop-run-open-inspect"
                onClick={() => inspect.onOpenChange(true)}
                size="sm"
                type="button"
                variant="ghost"
              >
                <Search aria-hidden="true" />
                Inspect
              </Button>
            </div>
          </div>
        }
      />
    </LoopRunChildReadContext>
  );
}

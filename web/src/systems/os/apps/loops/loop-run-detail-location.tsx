import { AlertCircle } from "lucide-react";

import { useNavigate } from "@tanstack/react-router";

import { Empty, useTopbarSlot } from "@compozy/ui";
import { loopRunsTrail } from "./loop-window-crumbs";
import { useLoopRunDetail } from "./use-loop-run-detail";
import { useLoopRunTopbar } from "./hooks/use-loop-run-topbar";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  isTerminalLoopStatus,
  LoopForkDialog,
  LoopNodeAmendDialog,
  LoopNodeControlDialog,
  LoopNodeRerunDialog,
  LoopNodeRowActions,
  LoopQuarantineSheet,
  LoopRunControlDialog,
  LoopRunPageBody,
  LoopRunPageSkeleton,
} from "@/systems/loops";
import { DESTINATION_COPY, useActiveWorkspace } from "@/systems/workspace";
import { useLoopWindowScope } from "./hooks/use-loop-window-scope";
import { LoopUnboundState } from "./loop-unbound-state";

export function LoopRunDetailLocation({
  runId,
  requestFocus,
}: {
  runId: string;
  requestFocus?: { generation?: number; nodeId: string; itemIndex: number };
}) {
  const navigate = useNavigate();
  const { registeredWorkspaces } = useActiveWorkspace();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const { scope, workspaceId, search } = useLoopWindowScope();
  const workspaceName =
    scope?.kind === "user"
      ? DESTINATION_COPY.user
      : registeredWorkspaces.find(workspace => workspace.id === workspaceId)?.name;
  const openLoops = () => {
    void navigate({ to: "/loops" });
  };
  const openRuns = () => {
    void navigate({ to: "/loop-runs" });
  };
  useTopbarSlot(
    workspaceId === ""
      ? loopRunsTrail({ level: "run", onBack: openRuns, openLoops, openRuns, runId })
      : null
  );

  if (workspaceId === "") {
    return (
      <LoopUnboundState
        description="Open this run from Runs."
        scope={scope}
        testId="loop-run-detail-no-workspace"
      />
    );
  }

  // Key by runId so the SSE reducer state resets cleanly on a run switch.
  return (
    <LoopRunDetail
      key={runId}
      workspaceId={workspaceId}
      runId={runId}
      liveDataEnabled={liveDataEnabled}
      navigate={navigate}
      bindingSearch={search}
      openLoops={openLoops}
      openRuns={openRuns}
      workspaceName={workspaceName}
      requestFocus={requestFocus}
    />
  );
}

interface LoopRunDetailProps {
  workspaceId: string;
  runId: string;
  openLoops: () => void;
  openRuns: () => void;
  workspaceName?: string;
  liveDataEnabled: boolean;
  navigate: ReturnType<typeof useNavigate>;
  bindingSearch: { workspace?: string };
  requestFocus?: { generation?: number; nodeId: string; itemIndex: number };
}

/** Connects a run page to its data, window navigation, and coordinated control overlays. */
function LoopRunDetail({
  workspaceId,
  runId,
  openLoops,
  openRuns,
  workspaceName,
  liveDataEnabled,
  navigate,
  bindingSearch,
  requestFocus,
}: LoopRunDetailProps) {
  const detail = useLoopRunDetail(workspaceId, runId, { liveDataEnabled });
  const { page, nodeControls, requests, timetravel, dialogs, events, goalTurns } = detail;
  useLoopRunTopbar(detail, { runId, openLoops, openRuns, navigate, bindingSearch });

  // "No run yet", not "fetching": a window outside the live-data budget has not started its
  // reads and must not read as a missing run.
  if (page.runQuery.isPending) return <LoopRunPageSkeleton />;

  if (page.runQuery.error || !page.effectiveRun || !page.materializedContract) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="loop-run-detail-not-found"
      >
        <Empty
          className="max-w-md"
          description={page.runQuery.error?.message || "This run no longer exists."}
          icon={AlertCircle}
          title="Couldn't open this run"
        />
      </div>
    );
  }

  // The dialog and the sheet are overlays, not page content — they render as
  // siblings so the body keeps its childless presentational contract.
  return (
    <>
      <LoopRunPageBody
        run={page.effectiveRun}
        workspaceId={workspaceId}
        materializedContract={page.materializedContract}
        graph={page.graph}
        isLive={page.isLive}
        nowMs={page.nowMs}
        registers={page.registers}
        rosterNodes={page.rosterNodes}
        rosterRollups={page.rosterRollups}
        onLoadMoreRoster={page.onLoadMoreRoster}
        isLoadingMoreRoster={page.isLoadingMoreRoster}
        nodeSelection={page.nodeSelection}
        onNodeSelectionChange={page.onNodeSelectionChange}
        prunedSessionIds={page.prunedSessionIds}
        storyPaging={page.storyPaging}
        rosterRead={page.rosterRead}
        briefingRead={page.briefingRead}
        events={events}
        goalTurns={goalTurns}
        isReconnecting={page.isReconnecting}
        usageRows={page.usageRows}
        usageNote={page.usageNote}
        approvalRequest={page.approvalRequest}
        approvalFallbackFacts={page.approvalFallbackFacts}
        generations={page.generations}
        inputRows={page.inputRows}
        startedBy={page.startedBy}
        workspaceLabel={workspaceName ?? page.effectiveRun.workspace_id}
        versionLabel={page.versionLabel}
        watchEvents={page.watchEvents}
        inspect={{ open: dialogs.inspectOpen, onOpenChange: dialogs.setInspectOpen }}
        pendingAction={page.pendingAction}
        nodeLifecycles={page.nodeLifecycles}
        renderNodeActions={
          page.effectiveRun.historical
            ? undefined
            : node => (
                <LoopNodeRowActions
                  node={node}
                  onVerb={nodeControls.onVerb}
                  runStatus={page.run?.status}
                  timetravel={nodeControls.timetravelFor(node)}
                />
              )
        }
        onOpenQuarantine={nodeControls.openQuarantine}
        onDecision={page.handleDecision}
        requests={page.requests}
        requestFocus={requestFocus}
        requestState={{
          ...requests,
          onAnswer: input => {
            void requests.onAnswer(input);
          },
        }}
        onCompareGeneration={timetravel.onCompareGeneration}
        onForkGeneration={timetravel.onForkGeneration}
      />
      <LoopRunDetailDialogs
        detail={detail}
        workspaceId={workspaceId}
        runId={runId}
        run={page.effectiveRun}
      />
    </>
  );
}

/** Keeps run and node overlays coordinated, including a dialog nested in quarantine. */
function LoopRunDetailDialogs({
  detail,
  workspaceId,
  runId,
  run,
}: {
  detail: ReturnType<typeof useLoopRunDetail>;
  workspaceId: string;
  runId: string;
  run: NonNullable<ReturnType<typeof useLoopRunDetail>["page"]["effectiveRun"]>;
}) {
  const { page, nodeControls, timetravel, dialogs } = detail;
  const quarantineNode =
    nodeControls.quarantineNodeId === null
      ? null
      : (page.nodesById.get(nodeControls.quarantineNodeId) ?? null);
  const sheetNestsNodeDialog = quarantineNode !== null && nodeControls.request !== null;

  return (
    <>
      <LoopNodeControlDialog
        answer={sheetNestsNodeDialog ? undefined : nodeControls.answer}
        error={sheetNestsNodeDialog ? undefined : nodeControls.error}
        isPending={nodeControls.isPending}
        onConfirm={nodeControls.commit}
        onOpenChange={open => {
          if (!open) nodeControls.closeDialog();
        }}
        request={sheetNestsNodeDialog ? null : nodeControls.request}
      />
      <LoopNodeAmendDialog
        answer={nodeControls.timetravelAnswer}
        fieldErrors={nodeControls.amendFieldErrors}
        isPending={nodeControls.isTimetravelPending}
        node={nodeControls.amendNode}
        onConfirm={input => {
          void nodeControls.commitAmend(input);
        }}
        onOpenChange={open => {
          if (!open) nodeControls.closeTimetravel();
        }}
        open={nodeControls.amendNode !== null}
        originalOutput={nodeControls.amendOriginalOutput}
        outputSchema={nodeControls.amendOutputSchema}
        workspaceId={workspaceId}
      />
      <LoopNodeRerunDialog
        answer={nodeControls.timetravelAnswer}
        isPending={nodeControls.isTimetravelPending}
        node={nodeControls.rerunNode}
        onConfirm={input => {
          void nodeControls.commitRerun(input);
        }}
        onOpenChange={open => {
          if (!open) nodeControls.closeTimetravel();
        }}
        open={nodeControls.rerunNode !== null}
        rerunSet={nodeControls.rerunSet}
      />
      <LoopForkDialog
        blockedReason={timetravel.forkBlockedReason}
        defaultGeneration={timetravel.forkGeneration}
        fieldErrors={timetravel.forkFieldErrors}
        generations={timetravel.forkGenerations}
        inputSchema={timetravel.forkInputSchema}
        isPending={timetravel.isForkPending}
        loopName={timetravel.loopName}
        onOpenChange={open => {
          if (!open) timetravel.onCloseFork();
        }}
        onSubmit={input => {
          void timetravel.onSubmitFork(input);
        }}
        open={timetravel.forkGeneration !== null}
        sourceInputs={timetravel.forkSourceInputs}
        workspaceId={workspaceId}
      />
      <LoopRunControlDialog
        answer={page.cancelAnswer}
        elapsedLabel={page.elapsedLabel}
        error={page.cancelError}
        generation={run.generation}
        inFlightCount={
          page.nodeLifecycles.filter(
            node => !node.parked && node.state !== "canceled" && node.cancelState !== "canceled"
          ).length
        }
        isPending={page.isCancelPending}
        onConfirm={verb => {
          void dialogs.confirmRunControl(verb);
        }}
        onOpenChange={open => {
          if (!open) dialogs.closeRunControl();
        }}
        runId={runId}
        status={run.status}
        verb={dialogs.runVerb}
        waitingOnYouCount={page.waitingNodes.length}
      />
      <LoopQuarantineSheet
        isRequeuePending={nodeControls.isPending}
        node={quarantineNode}
        runEnded={isTerminalLoopStatus(run.status)}
        onOpenChange={open => {
          if (!open) nodeControls.closeQuarantine();
        }}
        onVerb={nodeControls.onVerb}
        open={quarantineNode !== null}
      >
        {sheetNestsNodeDialog ? (
          <LoopNodeControlDialog
            answer={nodeControls.answer}
            error={nodeControls.error}
            isPending={nodeControls.isPending}
            onConfirm={nodeControls.commit}
            onOpenChange={open => {
              if (!open) nodeControls.closeDialog();
            }}
            request={nodeControls.request}
          />
        ) : null}
      </LoopQuarantineSheet>
    </>
  );
}

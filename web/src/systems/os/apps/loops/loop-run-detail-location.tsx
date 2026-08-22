import { Activity, AlertCircle } from "lucide-react";

import { useNavigate } from "@tanstack/react-router";

import { Empty, Pill, Spinner, useTopbarSlot } from "@compozy/ui";
import { loopRunsTrail } from "./loop-window-crumbs";
import { useLoopRunDetail } from "./use-loop-run-detail";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  LoopForkDialog,
  LoopNodeAmendDialog,
  LoopNodeControlDialog,
  LoopNodeRerunDialog,
  LoopNodeRowActions,
  LoopQuarantineSheet,
  LoopRunControlDialog,
  LoopRunControls,
  LoopRunOverflowMenu,
  LoopRunPageBody,
  LoopStatusPill,
} from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

export function LoopRunDetailLocation({
  runId,
  routeWorkspaceId,
  requestFocus,
}: {
  runId: string;
  routeWorkspaceId?: string;
  requestFocus?: { generation?: number; nodeId: string; itemIndex: number };
}) {
  const navigate = useNavigate();
  const { activeWorkspace, runtimeWorkspaceId, workspaces } = useActiveWorkspace();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const workspaceId = routeWorkspaceId ?? runtimeWorkspaceId ?? "";
  const workspaceName = workspaces.find(workspace => workspace.id === workspaceId)?.name;
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
      <div
        className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
        data-testid="loop-run-detail-no-workspace"
      >
        <Empty
          className="max-w-md"
          description="Select a workspace to monitor this run."
          icon={Activity}
          title="No workspace selected"
        />
      </div>
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
      openLoops={openLoops}
      openRuns={openRuns}
      workspaceName={
        workspaceName ?? (activeWorkspace?.id === workspaceId ? activeWorkspace.name : undefined)
      }
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
  requestFocus?: { generation?: number; nodeId: string; itemIndex: number };
}

function LoopRunDetail({
  workspaceId,
  runId,
  openLoops,
  openRuns,
  workspaceName,
  liveDataEnabled,
  navigate,
  requestFocus,
}: LoopRunDetailProps) {
  const { page, nodeControls, requests, timetravel, dialogs, events } = useLoopRunDetail(
    workspaceId,
    runId,
    { liveDataEnabled }
  );
  const quarantineNode =
    nodeControls.quarantineNodeId === null
      ? null
      : (page.nodesById.get(nodeControls.quarantineNodeId) ?? null);
  const sheetNestsNodeDialog = quarantineNode !== null && nodeControls.request !== null;

  const loopName = page.run?.loop_name;
  useTopbarSlot({
    ...loopRunsTrail({
      level: "run",
      loopName,
      onBack: openRuns,
      openLoop:
        loopName === undefined
          ? undefined
          : () => {
              void navigate({ to: "/loops/$name", params: { name: loopName } });
            },
      openLoops,
      openRuns,
      runId,
    }),
    status: page.run ? (
      <span className="flex items-center gap-2">
        <LoopStatusPill status={page.run.status} data-testid="loop-run-status-pill" />
        {page.run.historical ? (
          <Pill data-testid="loop-run-history-pill" size="xs" tone="neutral">
            History
          </Pill>
        ) : null}
      </span>
    ) : undefined,
    actions:
      page.run && !page.run.historical ? (
        <div className="flex items-center gap-2">
          <LoopRunControls
            status={page.run.status}
            pauseRequested={page.run.pause_requested}
            pendingVerb={page.pendingRunVerb}
            onPause={page.handlePause}
            onResume={page.handleResume}
            onCancel={() => dialogs.openRunControl("cancel")}
          />
          <LoopRunOverflowMenu
            isKillPending={page.isKillPending}
            loopName={page.run.loop_name}
            // Kill is offered only while the run is live; a terminal run gets the
            // views but no verb the daemon would reject.
            onKill={page.canKillRun ? () => dialogs.openRunControl("kill") : undefined}
          />
        </div>
      ) : undefined,
  });

  if (page.runQuery.isLoading) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center"
        data-testid="loop-run-detail-loading"
      >
        <Spinner aria-hidden="true" className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.runQuery.error || !page.effectiveRun || !page.materializedContract) {
    return (
      <div
        className="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 px-6 text-center"
        data-testid="loop-run-detail-not-found"
      >
        <AlertCircle className="size-6 text-danger" />
        <p className="text-sm text-muted">
          {page.runQuery.error?.message || `Run ${runId} not found.`}
        </p>
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
        events={events}
        isReconnecting={page.isReconnecting}
        usageRows={page.usageRows}
        usageNote={page.usageNote}
        approvalRequest={page.live.needsApproval}
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
        answer={dialogs.runVerb === "kill" ? page.killAnswer : page.cancelAnswer}
        elapsedLabel={page.elapsedLabel}
        error={dialogs.runVerb === "kill" ? page.killError : page.cancelError}
        generation={page.effectiveRun.generation}
        inFlightCount={
          page.nodeLifecycles.filter(
            node => !node.parked && node.state !== "canceled" && node.cancelState !== "canceled"
          ).length
        }
        isPending={page.isCancelPending || page.isKillPending}
        onConfirm={verb => {
          void dialogs.confirmRunControl(verb);
        }}
        onOpenChange={open => {
          if (!open) dialogs.closeRunControl();
        }}
        runId={runId}
        status={page.effectiveRun.status}
        verb={dialogs.runVerb}
        waitingOnYouCount={page.waitingNodes.length}
      />
      <LoopQuarantineSheet
        isRequeuePending={nodeControls.isPending}
        node={quarantineNode}
        onOpenChange={open => {
          if (!open) nodeControls.closeQuarantine();
        }}
        onVerb={nodeControls.onVerb}
        open={quarantineNode !== null}
        runId={runId}
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

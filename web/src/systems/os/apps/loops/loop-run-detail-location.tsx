import { Activity, AlertCircle } from "lucide-react";

import { useNavigate } from "@tanstack/react-router";

import { Empty, Spinner, useTopbarSlot, type TopbarSlotValue } from "@compozy/ui";
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
}: {
  runId: string;
  routeWorkspaceId?: string;
}) {
  const navigate = useNavigate();
  const { activeWorkspace, runtimeWorkspaceId, workspaces } = useActiveWorkspace();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const workspaceId = routeWorkspaceId ?? runtimeWorkspaceId ?? "";
  const workspaceName = workspaces.find(workspace => workspace.id === workspaceId)?.name;
  const backToRuns = () => {
    void navigate({ to: "/loop-runs" });
  };
  const topbarIdentity: Pick<TopbarSlotValue, "crumb" | "crumbs" | "onBack"> = {
    onBack: backToRuns,
    crumbs: [
      {
        id: "loops",
        label: "Loops",
        onSelect: () => {
          void navigate({ to: "/loops" });
        },
      },
      { id: "runs", label: "Runs", onSelect: backToRuns },
    ],
    crumb: runId,
  };

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
      topbarIdentity={topbarIdentity}
      workspaceName={
        workspaceName ?? (activeWorkspace?.id === workspaceId ? activeWorkspace.name : undefined)
      }
    />
  );
}

interface LoopRunDetailProps {
  workspaceId: string;
  runId: string;
  topbarIdentity: Pick<TopbarSlotValue, "crumb" | "crumbs" | "onBack">;
  workspaceName?: string;
  liveDataEnabled: boolean;
  navigate: ReturnType<typeof useNavigate>;
}

function LoopRunDetail({
  workspaceId,
  runId,
  topbarIdentity,
  workspaceName,
  liveDataEnabled,
  navigate,
}: LoopRunDetailProps) {
  const { page, nodeControls, requests, timetravel, dialogs } = useLoopRunDetail(
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
    ...topbarIdentity,
    crumbs: loopName
      ? [
          {
            id: "loops",
            label: "Loops",
            onSelect: () => {
              void navigate({ to: "/loops" });
            },
          },
          {
            id: "loop",
            label: loopName,
            onSelect: () => {
              void navigate({ to: "/loops/$name", params: { name: loopName } });
            },
          },
        ]
      : topbarIdentity.crumbs,
    status: page.run ? (
      <LoopStatusPill status={page.run.status} data-testid="loop-run-status-pill" />
    ) : undefined,
    actions: page.run ? (
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

  if (page.runQuery.error || !page.effectiveRun || !page.progress || !page.materializedContract) {
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
        definition={page.definition}
        materializedContract={page.materializedContract}
        graph={page.graph}
        isLive={page.isLive}
        subject={page.subject}
        hasWatchSource={page.hasWatchSource}
        elapsedLabel={page.elapsedLabel}
        stepElapsedLabel={page.stepElapsedLabel}
        progress={page.progress}
        story={page.story}
        goalIds={page.goalIds}
        goalTurns={page.goalTurns}
        goalTurnsPaging={{
          hasMore: page.goalTurnsQuery.hasNextPage,
          isLoading: page.goalTurnsQuery.isFetchingNextPage,
          onLoadMore: () => {
            void page.goalTurnsQuery.fetchNextPage();
          },
        }}
        usageRows={page.usageRows}
        usageNote={page.usageNote}
        approvalRequest={page.live.needsApproval}
        approvalFallbackFacts={page.approvalFallbackFacts}
        failure={page.live.failure}
        latestVerdict={page.latestVerdict}
        watchEvents={page.watchEvents}
        watchCadence={page.watchCadence}
        generations={page.generations}
        frames={page.live.frames}
        inputRows={page.inputRows}
        startedBy={page.startedBy}
        workspaceLabel={workspaceName ?? page.effectiveRun.workspace_id}
        versionLabel={page.versionLabel}
        nextNote={page.nextNote}
        showNowCard={page.showNowCard}
        terminalFromStatus={page.terminalFromStatus}
        terminalAt={page.terminalAt}
        terminalCause={page.terminalCause}
        inspect={{ open: dialogs.inspectOpen, onOpenChange: dialogs.setInspectOpen }}
        pendingAction={page.pendingAction}
        nodeLifecycles={page.nodeLifecycles}
        nodeNowLines={page.nodeNowLines}
        waitingNodes={page.waitingNodes}
        attentionNodes={page.attentionNodes}
        nodesById={page.nodesById}
        nodeSessions={page.nodeSessions}
        renderNodeActions={node => (
          <LoopNodeRowActions
            isPending={nodeControls.isBusy}
            node={node}
            onVerb={nodeControls.onVerb}
            runStatus={page.run?.status}
            timetravel={nodeControls.timetravelFor(node)}
          />
        )}
        onOpenQuarantine={nodeControls.openQuarantine}
        onDecision={page.handleDecision}
        onStartNewRun={page.handleStartNewRun}
        requests={page.requests}
        requestState={{
          ...requests,
          onAnswer: input => {
            void requests.onAnswer(input);
          },
        }}
        strategyProgress={page.strategyProgress}
        onOpenRun={timetravel.onOpenRun}
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

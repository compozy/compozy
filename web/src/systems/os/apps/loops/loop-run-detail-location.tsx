import { useState } from "react";
import { Activity, AlertCircle } from "lucide-react";

import { useNavigate } from "@tanstack/react-router";

import { Empty, Spinner, useTopbarSlot, type TopbarSlotValue } from "@agh/ui";
import { useLoopRunPage } from "./use-loop-run-page";
import {
  LoopRunControls,
  LoopRunOverflowMenu,
  LoopRunPageBody,
  LoopStatusPill,
} from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

export function LoopRunDetailLocation({ runId }: { runId: string }) {
  const navigate = useNavigate();
  const { activeWorkspace, activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
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
      topbarIdentity={topbarIdentity}
      workspaceName={activeWorkspace?.name}
    />
  );
}

interface LoopRunDetailProps {
  workspaceId: string;
  runId: string;
  topbarIdentity: Pick<TopbarSlotValue, "crumb" | "crumbs" | "onBack">;
  workspaceName?: string;
}

function LoopRunDetail({ workspaceId, runId, topbarIdentity, workspaceName }: LoopRunDetailProps) {
  const page = useLoopRunPage(workspaceId, runId);
  const [inspectOpen, setInspectOpen] = useState(false);

  useTopbarSlot({
    ...topbarIdentity,
    status: page.run ? (
      <LoopStatusPill status={page.run.status} data-testid="loop-run-status-pill" />
    ) : undefined,
    actions: page.run ? (
      <div className="flex items-center gap-2">
        <LoopRunControls
          status={page.run.status}
          pauseRequested={page.run.pause_requested}
          isPausePending={page.isPausePending}
          isResumePending={page.isResumePending}
          isStopPending={page.isStopPending}
          onPause={page.handlePause}
          onResume={page.handleResume}
          onStop={page.handleStop}
        />
        <LoopRunOverflowMenu loopName={page.run.loop_name} onInspect={() => setInspectOpen(true)} />
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

  if (page.runQuery.error || !page.effectiveRun || !page.progress) {
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

  return (
    <LoopRunPageBody
      run={page.effectiveRun}
      definition={page.definition}
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
      frames={page.live.frames}
      inputRows={page.inputRows}
      startedBy={page.startedBy}
      workspaceLabel={workspaceName ?? page.effectiveRun.workspace_id}
      versionLabel={page.versionLabel}
      nextNote={page.nextNote}
      showNowCard={page.showNowCard}
      terminalFromStatus={page.terminalFromStatus}
      terminalAt={page.terminalAt}
      inspect={{ open: inspectOpen, onOpenChange: setInspectOpen }}
      pendingAction={page.pendingAction}
      onDecision={page.handleDecision}
      onStartNewRun={page.handleStartNewRun}
    />
  );
}

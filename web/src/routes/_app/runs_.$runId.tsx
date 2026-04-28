import { useCallback, useEffect, useRef, useState, type ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { Alert, SkeletonRow } from "@compozy/ui";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { dashboardKeys } from "@/systems/dashboard";
import {
  RunDetailView,
  isTerminalKind,
  runKeys,
  useCancelRun,
  useRunEventFeed,
  useRunSnapshot,
  useRunStream,
  useRunTranscript,
  type RunStreamOverflow,
} from "@/systems/runs";

export const Route = createFileRoute("/_app/runs_/$runId")({
  component: RunDetailRoute,
});

function RunDetailRoute(): ReactElement {
  const { runId } = useParams({ from: "/_app/runs_/$runId" });
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const snapshotQuery = useRunSnapshot(runId);
  const transcriptQuery = useRunTranscript(runId);
  const cancelMutation = useCancelRun();
  const [cancelMessage, setCancelMessage] = useState<string | null>(null);
  const [cancelError, setCancelError] = useState<string | null>(null);
  const [terminalEventSeen, setTerminalEventSeen] = useState(false);
  const { append, events } = useRunEventFeed(runId);
  const closeStreamRef = useRef<() => void>(() => {});

  useEffect(() => {
    setTerminalEventSeen(false);
  }, [runId]);

  const handleOverflow = useCallback(
    (_overflow: RunStreamOverflow) => {
      void queryClient.invalidateQueries({ queryKey: runKeys.snapshot(runId) });
      void queryClient.invalidateQueries({ queryKey: runKeys.transcript(runId) });
    },
    [queryClient, runId]
  );

  const handleStreamEvent = useCallback(
    (signal: { eventId: string | null; payload: unknown }) => {
      const normalized = append(signal.eventId, signal.payload);
      if (normalized && isTerminalKind(normalized.kind)) {
        closeStreamRef.current();
        setTerminalEventSeen(true);
        void queryClient.invalidateQueries({ queryKey: runKeys.snapshot(runId) });
        void queryClient.invalidateQueries({ queryKey: runKeys.transcript(runId) });
        void queryClient.invalidateQueries({ queryKey: runKeys.lists() });
        void queryClient.invalidateQueries({
          queryKey: dashboardKeys.byWorkspace(activeWorkspace.id),
        });
      }
    },
    [activeWorkspace.id, append, queryClient, runId]
  );

  const initialCursor = snapshotQuery.data?.next_cursor ?? null;
  const runTerminated = terminalEventSeen || isTerminalStatus(snapshotQuery.data?.run?.status);
  const runStream = useRunStream({
    runId,
    enabled: Boolean(snapshotQuery.data) && !runTerminated,
    initialCursor,
    onOverflow: handleOverflow,
    onEvent: handleStreamEvent,
  });
  closeStreamRef.current = runStream.close;

  async function handleCancel() {
    setCancelMessage(null);
    setCancelError(null);
    try {
      await cancelMutation.mutateAsync({ runId });
      setCancelMessage("Cancellation requested — the daemon will drain and stop the run.");
    } catch (error) {
      setCancelError(apiErrorMessage(error, "Failed to cancel run"));
    }
  }

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
      header={
        <div className="flex w-full items-center justify-between gap-3">
          <button
            className="text-xs font-medium text-primary transition-colors hover:text-foreground"
            data-testid="run-detail-back"
            onClick={() => void navigate({ to: "/runs" })}
            type="button"
          >
            ← Back to runs
          </button>
          <span className="eyebrow text-muted-foreground">run detail</span>
        </div>
      }
    >
      {snapshotQuery.isLoading && !snapshotQuery.data ? (
        <div className="space-y-3" data-testid="run-detail-loading">
          <p className="sr-only">Loading run snapshot…</p>
          <SkeletonRow />
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}
      {snapshotQuery.isError && !snapshotQuery.data ? (
        <Alert data-testid="run-detail-load-error" variant="error">
          {apiErrorMessage(snapshotQuery.error, "Failed to load run snapshot")}
        </Alert>
      ) : null}
      {snapshotQuery.data ? (
        <RunDetailView
          cancelDisabled={runTerminated || !snapshotQuery.data}
          cancelError={cancelError}
          cancelSuccess={cancelMessage}
          isCancelling={cancelMutation.isPending}
          isRefreshingSnapshot={snapshotQuery.isRefetching}
          isLoadingTranscript={transcriptQuery.isLoading}
          isTranscriptError={transcriptQuery.isError}
          lastHeartbeatAt={runStream.lastHeartbeat?.receivedAt ?? null}
          liveEvents={events}
          onCancelRun={handleCancel}
          onReconnectStream={runStream.reconnect}
          overflowReason={runStream.lastOverflow?.reason ?? null}
          snapshot={snapshotQuery.data}
          streamError={
            runStream.error
              ? apiErrorMessage(runStream.error, "Run stream encountered an error")
              : null
          }
          streamEventCount={runStream.eventCount}
          streamStatus={runStream.status}
          transcript={transcriptQuery.data}
          transcriptError={
            transcriptQuery.error
              ? apiErrorMessage(transcriptQuery.error, "Failed to load run transcript")
              : null
          }
        />
      ) : null}
    </AppShellLayout>
  );
}

function isTerminalStatus(status: string | undefined): boolean {
  if (!status) {
    return false;
  }
  const normalized = status.toLowerCase();
  return (
    normalized === "completed" ||
    normalized === "succeeded" ||
    normalized === "failed" ||
    normalized === "crashed" ||
    normalized === "canceled" ||
    normalized === "cancelled"
  );
}

import { useCallback, useState, type ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import {
  RunDetailView,
  isTerminalKind,
  runKeys,
  useCancelRun,
  useRunEventFeed,
  useRunSnapshot,
  useRunStream,
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
  const cancelMutation = useCancelRun();
  const [cancelMessage, setCancelMessage] = useState<string | null>(null);
  const [cancelError, setCancelError] = useState<string | null>(null);
  const feed = useRunEventFeed(runId);

  const handleOverflow = useCallback(
    (_overflow: RunStreamOverflow) => {
      void queryClient.invalidateQueries({ queryKey: runKeys.snapshot(runId) });
    },
    [queryClient, runId]
  );

  const handleStreamEvent = useCallback(
    (signal: { eventId: string | null; payload: unknown }) => {
      const normalized = feed.append(signal.eventId, signal.payload);
      if (normalized && isTerminalKind(normalized.kind)) {
        void queryClient.invalidateQueries({ queryKey: runKeys.snapshot(runId) });
        void queryClient.invalidateQueries({ queryKey: runKeys.lists() });
      }
    },
    [feed, queryClient, runId]
  );

  const initialCursor = snapshotQuery.data?.next_cursor ?? null;
  const runTerminated = isTerminalStatus(snapshotQuery.data?.run?.status);
  const runStream = useRunStream({
    runId,
    enabled: Boolean(snapshotQuery.data) && !runTerminated,
    initialCursor,
    onOverflow: handleOverflow,
    onEvent: handleStreamEvent,
  });

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
            className="text-xs text-accent hover:underline"
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
        <p className="text-sm text-muted-foreground" data-testid="run-detail-loading">
          Loading run snapshot…
        </p>
      ) : null}
      {snapshotQuery.isError && !snapshotQuery.data ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="run-detail-load-error"
          role="alert"
        >
          {apiErrorMessage(snapshotQuery.error, "Failed to load run snapshot")}
        </p>
      ) : null}
      {snapshotQuery.data ? (
        <RunDetailView
          cancelDisabled={runTerminated || !snapshotQuery.data}
          cancelError={cancelError}
          cancelSuccess={cancelMessage}
          isCancelling={cancelMutation.isPending}
          isRefreshingSnapshot={snapshotQuery.isRefetching}
          lastHeartbeatAt={runStream.lastHeartbeat?.receivedAt ?? null}
          liveEvents={feed.events}
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
    normalized === "canceled" ||
    normalized === "cancelled"
  );
}

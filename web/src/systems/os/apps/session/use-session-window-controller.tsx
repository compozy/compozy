import { useSessionClearDialog } from "@/hooks/routes/use-session-clear-dialog";
import { useSessionPageControls } from "@/hooks/routes/use-session-page-controls";
import { shallowEqual } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

import { useSessionWindowPullRequest } from "./use-session-window-pull-request";
import { useSessionWindowSidebar } from "./use-session-window-sidebar";
import { useSessionWindowLifecycle } from "./hooks/use-session-window-lifecycle";
import {
  getSessionPromptRuntimeSnapshot,
  type InspectorUsage,
  isSessionTransportDisconnected,
  type SessionOriginView,
  type SessionPayload,
  SessionTransportChip,
  useSessionCommands,
  useSessionGoalHeader,
  useSessionInspectorState,
  useSessionReviewPanelState,
  useSessionLinkedWork,
  useSessionPromptRuntimeContext,
  sessionSuccessorId,
  useSessionTopbarSlot,
  useSessionTransportState,
  useSessionWorktreeBinding,
  useSessionContext,
  useSessionUsageTurns,
} from "@/systems/session";
import { alwaysOnMainInstance, useAgentInstances } from "@/systems/agent";
import type { WorktreePayload } from "@/systems/workspace";

function toInspectorUsage(
  usage: ReturnType<typeof useSessionContext>["usage"]
): InspectorUsage | null {
  return usage
    ? {
        tokensIn: usage.input_tokens ?? undefined,
        cacheReadTokens: usage.cache_read_tokens ?? undefined,
        cacheWriteTokens: usage.cache_write_tokens ?? undefined,
        tokensOut: usage.output_tokens ?? undefined,
        totalTokens: usage.total_tokens ?? undefined,
        costUsd: usage.total_cost ?? undefined,
        costCurrency: usage.cost_currency || undefined,
        costStatus: usage.cost_status ?? undefined,
        costSource: usage.cost_source ?? undefined,
        turnCount: usage.turn_count,
        workTime: usage.work_time,
      }
    : null;
}

/** Coordinates session window reads and projects query state into inspector view models. */
export function useSessionWindowController(input: {
  windowId: string;
  sessionId: string;
  workspaceId: string;
  session: SessionPayload;
  liveDataEnabled: boolean;
  onOpenWorktreeContext?: (workspaceId: string, worktree: WorktreePayload) => void;
  onResolveMissingWorktree?: (workspaceId: string, worktree: WorktreePayload) => void;
  /** Origin chrome published into the window topbar. */
  origin?: SessionOriginView | null;
  onOpenOriginSource?: (sessionId: string) => void;
}) {
  const {
    windowId,
    sessionId,
    workspaceId,
    session,
    liveDataEnabled,
    onOpenWorktreeContext,
    onResolveMissingWorktree,
    origin,
    onOpenOriginSource,
  } = input;
  // The window host owns the lifecycle confirmations; the rail inside this body opens them.
  const lifecycle = useSessionWindowLifecycle();
  const promptRuntime = useSessionPromptRuntimeContext();
  const promptRuntimeSnapshot = useSelector(
    promptRuntime,
    () => getSessionPromptRuntimeSnapshot(promptRuntime),
    shallowEqual
  );
  const controls = useSessionPageControls(sessionId, session, {
    getRuntimeSnapshot: () => getSessionPromptRuntimeSnapshot(promptRuntime),
    liveDataEnabled,
    workspaceId: session.workspace_id,
  });
  const inspector = useSessionInspectorState(sessionId);
  const review = useSessionReviewPanelState(sessionId);
  const usageEnabled = liveDataEnabled || inspector.open;
  const sessionContext = useSessionContext(sessionId, workspaceId, session.state, {
    enabled: usageEnabled,
  });
  const sessionUsageTurns = useSessionUsageTurns(sessionId, workspaceId, session.state, {
    enabled: usageEnabled,
  });
  const sessionCommands = useSessionCommands(workspaceId, sessionId, { enabled: liveDataEnabled });
  const inspectorUsage = toInspectorUsage(sessionContext.usage);
  const clearDialog = useSessionClearDialog(controls.handleClear);
  const transport = useSessionTransportState();
  const sidebar = useSessionWindowSidebar({
    sessionId,
    // The list never reads "connected" under a dead stream (US-018.AC-1).
    transportDisconnected:
      transport.phase === "failed" || isSessionTransportDisconnected(transport),
    windowId,
    lifecycle: lifecycle ?? undefined,
  });
  // The strip owns Goal controls and stream reconciliation. This reader only
  // supplies activity and the session-stop consequence from the shared cache.
  const goal = useSessionGoalHeader(workspaceId, sessionId, {
    enabled: liveDataEnabled,
    stream: false,
    sessionState: session.state,
    stopping: controls.isStopping,
  });

  const worktreeBinding = useSessionWorktreeBinding({
    workspaceId,
    worktreeId: session.worktree_id,
    availability: session.worktree_binding,
    commandCatalog: sessionCommands.catalog,
    enabled: liveDataEnabled,
  });

  const pullRequest = useSessionWindowPullRequest({
    session,
    workspaceId,
    worktreeId: session.worktree_id,
    bound: worktreeBinding.bound && worktreeBinding.worktree !== undefined,
    liveDataEnabled,
    running: controls.isSessionRunning,
    queuePrompt: controls.handleQueuePrompt,
  });

  const instances = useAgentInstances();
  const successorId = sessionSuccessorId(session, instances);
  const linkedWork = useSessionLinkedWork({ session, workspaceId, enabled: liveDataEnabled });

  useSessionTopbarSlot({
    session,
    pullRequestExit: pullRequest.pullRequestExit,
    pullRequestRefresh: pullRequest.pullRequestRefresh,
    onSendPrompt: pullRequest.onSendPrompt,
    linkedWork,
    worktreeAction: item => {
      const worktree = worktreeBinding.worktrees.find(row => row.id === item.worktree_id);
      return worktree && onOpenWorktreeContext
        ? () => onOpenWorktreeContext(workspaceId, worktree)
        : undefined;
    },
    onOpenSession: id =>
      sidebar.openSession({ sessionId: id, agentName: "", workspaceId }, { newWindow: false }),
    worktreeBinding: worktreeBinding.bound
      ? {
          worktreeId: worktreeBinding.worktreeId,
          worktree: worktreeBinding.worktree,
          onOpenContext:
            worktreeBinding.worktree && onOpenWorktreeContext
              ? () => onOpenWorktreeContext(workspaceId, worktreeBinding.worktree!)
              : undefined,
          onResolve:
            worktreeBinding.worktree && onResolveMissingWorktree
              ? () => onResolveMissingWorktree(workspaceId, worktreeBinding.worktree!)
              : undefined,
        }
      : undefined,
    isStopping: controls.isStopping,
    isUnarchiving: controls.isUnarchiving,
    inspectorOpen: inspector.open,
    reviewOpen: review.open,
    onReviewToggle: review.toggle,
    sidebarOpen: sidebar.open,
    onSidebarToggle: sidebar.toggle,
    transportChip: <SessionTransportChip windowLive={liveDataEnabled} />,
    hasLiveGoal: goal.hasLiveGoal,
    onInspectorToggle: inspector.toggle,
    // A superseded session's window is read-only: no composer, so no composer context control.
    composerContextControl: successorId === null,
    onStop: controls.handleStop,
    onUnarchive: controls.handleUnarchive,
    origin,
    onOpenOriginSource,
  });

  return {
    controls,
    inspector,
    review,
    sidebar,
    sessionContext,
    sessionUsageTurns,
    inspectorUsage,
    clearDialog,
    commandCatalog: sessionCommands.catalog,
    commandCatalogStatus: sessionCommands.isPending ? ("loading" as const) : ("ready" as const),
    refreshCommandCatalog: () => {
      void sessionCommands.refetch();
    },
    worktreeBinding,
    onSendPrompt: pullRequest.onSendPrompt,
    successorId,
    /** An always-on agent's current main: its window drops the message trail and context meter. */
    alwaysOnMain: alwaysOnMainInstance(instances, session.id) !== undefined,
    /** Dialogs and progress of an exit action started from the PR chip. */
    pullRequestExitSurface: pullRequest.exitSurface,
    promptRuntimeSnapshot,
    activityGoal: goal.snapshot,
  };
}

import { useDocumentVisible } from "@/hooks/use-document-visible";
import { useQuery } from "@tanstack/react-query";

import { useLoopRequestAttention } from "@/systems/loops";
import { useProfileReadScope } from "@/systems/profiles";
import {
  deriveAttentionBadges,
  type OsAttentionBadges,
  type OsAttentionSections,
  type OsAttentionRow,
} from "../lib/attention-model";
import {
  sessionListSortParam,
  type SessionCatalogStreamStatus,
  type SessionPayload,
  useSessionWindowOrder,
  useSessionCatalog,
} from "@/systems/session";
import { taskScopeForActiveWorkspace, useTaskDashboard } from "@/systems/tasks";
import {
  projectTerminalBadge,
  terminalInputRequestsQuery,
  terminalScope,
  terminalScopeKey,
} from "@/systems/terminal";
import {
  useActiveWorkspace,
  useScopedWorktreeFilter,
  workspaceSelectorOrAll,
  USER_SCOPE_SELECTOR,
  type DataScope,
  type WorkspacePayload,
} from "@/systems/workspace";

import { useBellNotifications } from "./use-bell-notifications";

import { useAttentionPolicy } from "./use-attention-policy";
import { useAttentionSummary, type OsAttentionSummaryModel } from "./use-attention-summary";
import { useFocusedWorktreeScopeId } from "./use-worktree-scope";

const ATTENTION_REFETCH_INTERVAL_MS = 5_000;

export interface OsAttentionModel {
  badges: OsAttentionBadges;
  notificationCount: number;
  notificationTotal?: number;
  notificationSnapshot?: string;
  acknowledging?: boolean;
  acknowledgementError?: string | null;
  onAcknowledge?: (row?: OsAttentionRow) => void;
  sections: OsAttentionSections;
  sessions: SessionPayload[];
  attentionSessionsDisconnected: boolean;
  sessionsDisconnected: boolean;
  tasksDisconnected: boolean;
  loopRequestsDisconnected: boolean;
  loading: boolean;
}

/**
 * The shell's attention view model. Session and terminal totals come from
 * scoped daemon projections; bell rows come from the notification ledger.
 * The optional modal page follows the focused window's workspace/worktree,
 * while the production session list owns explicit continuation separately.
 */
export function useOsAttention(
  runtimeWorkspace: WorkspacePayload | null | undefined,
  sessionCatalogStreamStatus: SessionCatalogStreamStatus,
  archived: boolean,
  modalEnabled = true
): OsAttentionModel {
  const { dataScope } = useActiveWorkspace();
  const documentVisible = useDocumentVisible();
  const workspaceId = runtimeWorkspace?.id ?? null;
  const sessions = useSessionAttentionSources({
    archived,
    dataScope,
    sessionCatalogStreamStatus,
    workspaceId,
    modalEnabled,
  });
  const tasks = useTaskAttentionSources({ dataScope, documentVisible });
  // The loops badge counts what the Loops listing shows: breadth + profile lens.
  const loops = useLoopRequestAttention(dataScope, true, documentVisible);
  // Terminals belong to a project or to User; All projects has no terminal badge.
  const terminal = useTerminalAttentionSources({
    documentVisible,
    terminalApprovals: sessions.summary.terminalApprovals,
    workspaceId: dataScope === null ? null : workspaceSelectorOrAll(dataScope),
  });
  const policy = useAttentionPolicy();

  const baseBadges = deriveAttentionBadges({
    summary: sessions.summary.summary,
    summaryStale: sessions.summary.stale,
    dashboard: tasks.dashboard,
    tasksStale: tasks.disconnected,
    loopsPending: loops.pendingCount,
  });
  const badges = {
    ...baseBadges,
    ...(terminal.badge === undefined ? {} : { terminal: terminal.badge }),
  };
  const notifications = useBellNotifications(policy.mutedWorkspaceIds);
  return {
    badges,
    notificationCount: notifications.count,
    notificationTotal: notifications.total,
    notificationSnapshot: notifications.snapshot,
    acknowledging: notifications.pending,
    acknowledgementError: notifications.error,
    onAcknowledge: notifications.acknowledge,
    sections: notifications.sections,
    sessions: sessions.modal,
    attentionSessionsDisconnected: notifications.stale,
    sessionsDisconnected: sessions.disconnected,
    tasksDisconnected: notifications.stale,
    loopRequestsDisconnected: notifications.stale,
    loading: notifications.loading,
  };
}

function useSessionAttentionSources({
  archived,
  dataScope,
  sessionCatalogStreamStatus,
  workspaceId,
  modalEnabled,
}: {
  modalEnabled: boolean;
  archived: boolean;
  dataScope: DataScope | null;
  sessionCatalogStreamStatus: SessionCatalogStreamStatus;
  /** The focused project; only a project narrows to a worktree. */
  workspaceId: string | null;
}) {
  const enabled = dataScope !== null;
  const worktree = useScopedWorktreeFilter(workspaceId, useFocusedWorktreeScopeId(), {
    enabled: enabled && dataScope.kind === "project",
  });
  // The modal renders the order of the window it follows (the focused one).
  const listPreferences = useSessionWindowOrder();
  const summary = useAttentionSummary(sessionCatalogStreamStatus);
  // Sessions-modal content: a shell surface, so it follows the focused window's
  // scope exactly like the menubar chip. Distinct query key, no shared snapshot.
  const modalSessionsQuery = useSessionCatalog(
    dataScope,
    {
      include_health: true,
      limit: 100,
      sort: sessionListSortParam(listPreferences.sort),
      worktree: worktree.worktreeId,
      ...(archived ? { archive: "only" as const } : {}),
    },
    modalEnabled && enabled && worktree.resolved,
    { facets: false }
  );
  return {
    disconnected:
      !enabled ||
      (worktree.resolved &&
        (sessionCatalogStreamStatus === "stale" || (modalEnabled && modalSessionsQuery.failed))),
    loading: enabled && (!worktree.resolved || summary.loading || modalSessionsQuery.loading),
    modal: modalSessionsQuery.sessions,
    summary,
  };
}

function useTaskAttentionSources({
  dataScope,
  documentVisible,
}: {
  dataScope: DataScope | null;
  documentVisible: boolean;
}) {
  const taskScope = taskScopeForActiveWorkspace(dataScope);
  const enabled = taskScope !== null;
  const dashboardQuery = useTaskDashboard(taskScope ?? {}, {
    enabled,
    refetchIntervalMs: documentVisible ? ATTENTION_REFETCH_INTERVAL_MS : false,
  });
  const dashboard = dashboardQuery.data ?? null;
  const disconnected =
    !enabled ||
    dashboardQuery.isError ||
    dashboardQuery.data === undefined ||
    (dashboard?.freshness.stale ?? true);
  return {
    dashboard,
    disconnected,
  };
}

function useTerminalAttentionSources({
  documentVisible,
  terminalApprovals,
  workspaceId,
}: {
  documentVisible: boolean;
  terminalApprovals: OsAttentionSummaryModel["terminalApprovals"];
  workspaceId: string | null;
}) {
  const profile = useProfileReadScope();
  const enabled = workspaceId !== null;
  const terminalReadScope = terminalScope(workspaceId ?? "", profile.destination);
  const terminalRequests = useQuery({
    ...terminalInputRequestsQuery(terminalReadScope),
    enabled,
    refetchInterval: documentVisible ? ATTENTION_REFETCH_INTERVAL_MS : false,
  });
  const ready =
    enabled &&
    !terminalRequests.isError &&
    terminalRequests.data !== undefined &&
    terminalApprovals !== undefined;
  const profileId = profile.destinationOwner?.id;
  const approvalCount =
    terminalApprovals?.find(
      approval =>
        approval.workspace_id === (workspaceId === USER_SCOPE_SELECTOR ? "" : workspaceId) &&
        approval.profile_id === profileId
    )?.count ?? 0;
  return {
    badge:
      !ready || !profileId
        ? undefined
        : projectTerminalBadge({
            scopeKey: terminalScopeKey(
              terminalReadScope.key.workspaceId,
              terminalReadScope.key.profileKey
            ),
            profileId,
            inputRequests: terminalRequests.data?.pending ?? [],
            pendingApprovalCount: approvalCount,
          }).count,
    loading: terminalRequests.isLoading || terminalApprovals === undefined,
    ready,
  };
}

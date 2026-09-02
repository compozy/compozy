import { useDocumentVisible } from "@/hooks/use-document-visible";
import { useQuery } from "@tanstack/react-query";

import { useLoopNodeExists, useLoopRequestAttention } from "@/systems/loops";
import { useProfileReadScope } from "@/systems/profiles";
import {
  attentionCount,
  deriveAttentionBadges,
  deriveAttentionSections,
  type OsAttentionBadges,
  type OsAttentionSections,
} from "../lib/attention-model";
import {
  sessionListSortParam,
  type SessionCatalogStreamStatus,
  type SessionPayload,
  useSessionListPreferences,
  useSessions,
} from "@/systems/session";
import { taskScopeForActiveWorkspace, useTaskDashboard, useTasks } from "@/systems/tasks";
import {
  projectTerminalBadge,
  terminalInputRequestsQuery,
  terminalScope,
  terminalScopeKey,
  type TerminalInputRequest,
} from "@/systems/terminal";
import {
  useActiveWorkspace,
  useScopedWorktreeFilter,
  type WorkspacePayload,
  type WorkspaceScopeMode,
} from "@/systems/workspace";

import { useAttentionPolicy } from "./use-attention-policy";
import { useAttentionSessions } from "./use-attention-rows";
import { useAttentionSummary } from "./use-attention-summary";
import { useFocusedWorktreeScopeId } from "./use-worktree-scope";

const ATTENTION_REFETCH_INTERVAL_MS = 5_000;

export interface OsAttentionModel {
  badges: OsAttentionBadges;
  notificationCount: number;
  sections: OsAttentionSections;
  sessions: SessionPayload[];
  attentionSessionsDisconnected: boolean;
  sessionsDisconnected: boolean;
  tasksDisconnected: boolean;
  loopRequestsDisconnected: boolean;
  loading: boolean;
}

/**
 * The shell's attention view model.
 *
 * Counts and rows have separate authorities on purpose. Counts come from the
 * daemon's cross-workspace summary projection, so they stay exact past any page
 * size and past any worktree scope. Rows come from server-filtered
 * cross-workspace reads. The modal leg stays worktree-scoped because it is a
 * window's content, not an attention signal — and `archived` only ever reaches
 * that leg, so the archive can never inflate an attention count.
 */
export function useOsAttention(
  runtimeWorkspace: WorkspacePayload | null | undefined,
  sessionCatalogStreamStatus: SessionCatalogStreamStatus,
  archived: boolean
): OsAttentionModel {
  const { scope, activeWorkspaceId, workspaces } = useActiveWorkspace();
  const documentVisible = useDocumentVisible();
  const workspaceId = runtimeWorkspace?.id ?? null;
  const sessions = useSessionAttentionSources({
    archived,
    scope,
    sessionCatalogStreamStatus,
    workspaceId,
  });
  const tasks = useTaskAttentionSources({ activeWorkspaceId, documentVisible, scope });
  const loops = useLoopAttentionSources({ documentVisible, workspaceId, workspaces });
  const terminal = useTerminalAttentionSources({
    documentVisible,
    sessions: sessions.attention.sessions,
    workspaceId,
  });
  const policy = useAttentionPolicy();

  const baseBadges = deriveAttentionBadges({
    summary: sessions.summary.summary,
    summaryStale: sessions.summary.stale,
    dashboard: tasks.dashboard,
    tasksStale: tasks.disconnected,
    loopsPending: loops.requests.pendingCount,
  });
  const badges = {
    ...baseBadges,
    ...(terminal.badge === undefined ? {} : { terminal: terminal.badge }),
  };
  const sections = deriveAttentionSections({
    sessions: sessions.attention.sessions,
    sessionRowsStale: sessions.attention.stale,
    workspaceLabels: new Map(workspaces.map(workspace => [workspace.id, workspace.name])),
    mutedWorkspaceIds: policy.mutedWorkspaceIds,
    tasks: tasks.rows,
    taskRowsStale: tasks.rowsDisconnected,
    loopWaitingPresent: loops.waitingPresent,
    loopAttentionPresent: loops.attentionPresent,
    loopRequests: loops.requests.items,
    terminalRequests: terminal.rows,
    terminalRowsStale: !terminal.ready,
    terminalWorkspaceId: workspaceId ?? undefined,
  });
  return {
    badges,
    notificationCount: attentionCount(badges),
    sections,
    sessions: sessions.modal,
    attentionSessionsDisconnected: sessions.attention.stale || sessions.summary.stale,
    sessionsDisconnected: sessions.disconnected,
    tasksDisconnected: tasks.rowsDisconnected,
    loopRequestsDisconnected: loops.requests.disconnected,
    loading: sessions.loading || tasks.loading || terminal.loading || loops.requests.loading,
  };
}

function useSessionAttentionSources({
  archived,
  scope,
  sessionCatalogStreamStatus,
  workspaceId,
}: {
  archived: boolean;
  scope: WorkspaceScopeMode;
  sessionCatalogStreamStatus: SessionCatalogStreamStatus;
  workspaceId: string | null;
}) {
  const enabled = workspaceId !== null;
  const worktree = useScopedWorktreeFilter(workspaceId, useFocusedWorktreeScopeId(), {
    enabled: enabled && scope === "workspace",
  });
  // The modal renders the same order the operator chose everywhere else.
  const listPreferences = useSessionListPreferences();
  const summary = useAttentionSummary(sessionCatalogStreamStatus);
  const attention = useAttentionSessions(sessionCatalogStreamStatus, enabled);
  // Sessions-modal content: a shell surface, so it follows the focused window's
  // scope exactly like the menubar chip. Distinct query key, no shared snapshot.
  const modalSessionsQuery = useSessions(workspaceId, {
    enabled: enabled && worktree.resolved,
    loadAll: listPreferences.scope === "workspace",
    filters: {
      include_health: true,
      limit: 100,
      sort: sessionListSortParam(listPreferences.sort),
      worktree: worktree.worktreeId,
      ...(archived ? { archive: "only" as const } : {}),
    },
  });
  return {
    attention,
    disconnected:
      !enabled ||
      (worktree.resolved &&
        (sessionCatalogStreamStatus !== "live" ||
          modalSessionsQuery.isError ||
          modalSessionsQuery.data === undefined)),
    loading:
      enabled &&
      (!worktree.resolved || summary.loading || attention.loading || modalSessionsQuery.isLoading),
    modal: modalSessionsQuery.data ?? [],
    summary,
  };
}

function useTaskAttentionSources({
  activeWorkspaceId,
  documentVisible,
  scope,
}: {
  activeWorkspaceId: string | null;
  documentVisible: boolean;
  scope: WorkspaceScopeMode;
}) {
  const taskScope = taskScopeForActiveWorkspace(scope, activeWorkspaceId);
  const enabled = taskScope !== null;
  const dashboardQuery = useTaskDashboard(taskScope ?? {}, {
    enabled,
    refetchIntervalMs: documentVisible ? ATTENTION_REFETCH_INTERVAL_MS : false,
  });
  const tasksQuery = useTasks(
    {
      ...taskScope,
      approval_state: "pending",
      limit: 100,
      sort: "recent",
    },
    {
      enabled,
      refetchIntervalMs: documentVisible ? ATTENTION_REFETCH_INTERVAL_MS : false,
    }
  );
  const dashboard = dashboardQuery.data ?? null;
  const disconnected =
    !enabled ||
    dashboardQuery.isError ||
    dashboardQuery.data === undefined ||
    (dashboard?.freshness.stale ?? true);
  return {
    dashboard,
    disconnected,
    loading: enabled && (dashboardQuery.isLoading || tasksQuery.isLoading),
    rows: tasksQuery.data ?? [],
    rowsDisconnected: disconnected || tasksQuery.isError || tasksQuery.data === undefined,
  };
}

function useLoopAttentionSources({
  documentVisible,
  workspaceId,
  workspaces,
}: {
  documentVisible: boolean;
  workspaceId: string | null;
  workspaces: WorkspacePayload[];
}) {
  const enabled = workspaceId !== null;
  const loopWorkspaceId = workspaceId ?? "";
  return {
    waitingPresent: useLoopNodeExists(loopWorkspaceId, "waiting", enabled),
    attentionPresent: useLoopNodeExists(loopWorkspaceId, "attention", enabled),
    requests: useLoopRequestAttention(workspaces, true, documentVisible),
  };
}

function useTerminalAttentionSources({
  documentVisible,
  sessions,
  workspaceId,
}: {
  documentVisible: boolean;
  sessions: readonly SessionPayload[];
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
  const terminalQueryReady = !terminalRequests.isError && terminalRequests.data !== undefined;
  return {
    badge: terminalAttentionCount({
      ready: terminalQueryReady,
      profileId: profile.destinationOwner?.id,
      workspaceId,
      sessions,
      scopeKey: terminalScopeKey(
        terminalReadScope.key.workspaceId,
        terminalReadScope.key.profileKey
      ),
      pendingRequests: terminalRequests.data?.pending ?? [],
    }),
    loading: terminalRequests.isLoading,
    ready: terminalQueryReady,
    rows: (terminalRequests.data?.pending ?? []).map(request => ({
      id: request.id,
      terminal_id: request.terminal_id,
      ...(request.workspace_id ? { workspace_id: request.workspace_id } : {}),
      reason: request.reason,
      redacted: request.redacted,
      requested_at: request.requested_at,
      requester_id: request.requester.id,
    })),
  };
}

function terminalAttentionCount(input: {
  ready: boolean;
  profileId: string | undefined;
  workspaceId: string | null;
  sessions: readonly SessionPayload[];
  scopeKey: string;
  pendingRequests: readonly TerminalInputRequest[];
}): number | undefined {
  if (!input.ready) return undefined;
  // Profile identity comes from the profile catalog, the authority that
  // bound this read. An input request is optional and cannot identify a
  // profile when approvals are the only pending terminal work.
  const profileId = input.profileId;
  if (!profileId) return undefined;
  const pendingApprovals: Array<{ profileId: string }> = [];
  for (const session of input.sessions) {
    if (session.workspace_id !== input.workspaceId || session.profile_id !== profileId) continue;
    for (const interaction of session.pending_interactions) {
      if (
        interaction.kind === "permission" &&
        interaction.status === "pending" &&
        interaction.tool_id?.startsWith("compozy__terminal_")
      ) {
        pendingApprovals.push({ profileId });
      }
    }
  }
  return projectTerminalBadge({
    scopeKey: input.scopeKey,
    profileId,
    inputRequests: input.pendingRequests,
    pendingApprovals,
  }).count;
}

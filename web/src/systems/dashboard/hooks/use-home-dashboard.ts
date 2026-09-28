import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import type { ConnectionStatus } from "@compozy/ui";

import { hasNoRecordedWork } from "../lib/home-overview-empty";
import { homeScopeForActiveWorkspace, type HomeScope } from "../lib/home-scope";
import { homeActivityOptions, homeOverviewOptions } from "../lib/query-options";
import type {
  HomeActivityEvent,
  HomeOverview,
  HomeSurfaceStatus,
  HomeUsageWindow,
  HomeWorkingNowModel,
} from "../types";
import { useHomeAttentionActions, type HomeAttentionActions } from "./use-home-attention-actions";
import { useHomeAgents, type HomeAgentsModel } from "./use-home-agents";
import { useHomeLive } from "./use-home-live";
import { homePrefsStore, useHomeSystemOpen, useHomeUsageWindow } from "./use-home-prefs-store";
import { useHomeSystem, type HomeSystemModel } from "./use-home-system";
import { useHomeWorkingNow } from "./use-home-working-now";
import { useDaemonHealth } from "@/systems/status";
import { useActiveWorkspace } from "@/systems/workspace";
import type { AttentionNotificationScope } from "@/systems/notifications";
import { useProfileReadScope } from "@/systems/profiles";

export interface HomeDashboardModel {
  scope: HomeScope;
  notificationScope?: AttentionNotificationScope;
  attentionActions: HomeAttentionActions;
  /** The per-profile usage breakdown belongs to the aggregate read alone (S10). */
  profileAggregate: boolean;
  connectionStatus: ConnectionStatus;
  usageWindow: HomeUsageWindow;
  setUsageWindow: (window: HomeUsageWindow) => void;
  overview: HomeOverview | undefined;
  overviewStatus: HomeSurfaceStatus;
  overviewErrorMessage: string | null;
  /** The overview loaded and reports no work at all — the first-run read. */
  hasNoWork: boolean;
  activity: HomeActivityEvent[] | undefined;
  activityStatus: HomeSurfaceStatus;
  activityErrorMessage: string | null;
  activeWorkspaceName: string | null;
  workingNow: HomeWorkingNowModel;
  agents: HomeAgentsModel;
  system: HomeSystemModel;
  systemOpen: boolean;
  setSystemOpen: (open: boolean) => void;
}

interface UseHomeDashboardOptions {
  liveEnabled?: boolean;
}

function surfaceStatus(isLoading: boolean, isError: boolean, hasData: boolean): HomeSurfaceStatus {
  if (hasData) {
    return "ready";
  }
  if (isError) {
    return "error";
  }
  return isLoading ? "loading" : "ready";
}

// Stable argument for disabled queries while workspace scope is undetermined.
const UNSETTLED_HOME_SCOPE: HomeScope = { workspaceParam: "", taskScope: { scope: "global" } };

export function useHomeDashboard({
  liveEnabled = true,
}: UseHomeDashboardOptions = {}): HomeDashboardModel {
  const { connectionStatus } = useDaemonHealth();
  const { activeWorkspace, activeWorkspaceId, scope: workspaceScope } = useActiveWorkspace();
  const usageWindow = useHomeUsageWindow();
  const systemOpen = useHomeSystemOpen();

  const { aggregate, params: profileScope, destination } = useProfileReadScope();
  const resolvedScope = homeScopeForActiveWorkspace(workspaceScope, activeWorkspaceId);
  const scopeSettled = resolvedScope !== null;
  const scope = resolvedScope ?? UNSETTLED_HOME_SCOPE;

  const overviewQuery = useQuery(
    homeOverviewOptions(
      {
        workspace: scope.workspaceParam || undefined,
        usageWindow,
        receiptProfile: destination,
        // Usage follows the active view: a scoped profile sees only its own
        // figures, and the aggregate sees every owner labelled (US-013).
        ...("all_profiles" in profileScope
          ? { allProfiles: true }
          : { profile: profileScope.profile }),
      },
      scopeSettled
    )
  );
  const activityQuery = useQuery(
    homeActivityOptions(
      // Activity is owned work, not machine state: the feed shows one profile's
      // events or the labeled aggregate, never an unscoped mix.
      { workspace_id: scope.workspaceParam || undefined, ...profileScope },
      scopeSettled
    )
  );

  useHomeLive({
    workspaceId: scope.workspaceParam,
    scope: profileScope,
    enabled: scopeSettled && liveEnabled,
  });

  const workingNow = useHomeWorkingNow(scope, scopeSettled && liveEnabled);
  const overviewPresentation = queryPresentation(overviewQuery, scopeSettled);
  const activityPresentation = queryPresentation(activityQuery, scopeSettled);
  const overview = overviewQuery.data;
  const agents = useHomeAgents();
  const system = useHomeSystem(
    overview?.system.hook_runs_today,
    overview?.system.hook_failures_today,
    overview?.system.retention_days
  );

  const notificationScope: AttentionNotificationScope = {
    ...profileScope,
    receipt_profile: destination,
    workspace: scope.workspaceParam || undefined,
    surface: "home",
  };
  const attentionActions = useHomeAttentionActions({
    snapshot: overview?.attention.snapshot,
    scope: scopeSettled ? notificationScope : undefined,
  });

  return {
    scope,
    notificationScope,
    attentionActions,
    profileAggregate: aggregate,
    connectionStatus,
    usageWindow,
    setUsageWindow: usageWindow => homePrefsStore.trigger.usageWindowSelected({ usageWindow }),
    overview,
    overviewStatus: overviewPresentation.status,
    overviewErrorMessage: overviewPresentation.errorMessage,
    hasNoWork: overview !== undefined && hasNoRecordedWork(overview),
    activity: activityQuery.data,
    activityStatus: activityPresentation.status,
    activityErrorMessage: activityPresentation.errorMessage,
    activeWorkspaceName: workspaceScope === "global" ? "Global" : (activeWorkspace?.name ?? null),
    workingNow,
    agents,
    system,
    systemOpen,
    setSystemOpen: open => {
      if (open) {
        homePrefsStore.trigger.systemPanelOpened();
        return;
      }
      homePrefsStore.trigger.systemPanelClosed();
    },
  };
}

function queryPresentation(
  query: Pick<UseQueryResult, "isLoading" | "isError" | "data" | "error">,
  scopeSettled: boolean
) {
  return {
    status: surfaceStatus(
      query.isLoading || !scopeSettled,
      query.isError,
      query.data !== undefined
    ),
    errorMessage: query.error instanceof Error ? query.error.message : null,
  };
}

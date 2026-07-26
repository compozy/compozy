import { useQuery } from "@tanstack/react-query";
import type { ConnectionStatus } from "@agh/ui";

import { useDaemonHealth, useDaemonStatus } from "@/systems/status";
import { useActiveWorkspace } from "@/systems/workspace";

import { homeScopeForActiveWorkspace, type HomeScope } from "../lib/home-scope";
import { homeActivityOptions, homeOverviewOptions } from "../lib/query-options";
import type {
  HomeActivityEvent,
  HomeOverview,
  HomeSurfaceStatus,
  HomeUsageWindow,
  HomeWorkingNowModel,
} from "../types";
import { useHomeAgents, type HomeAgentsModel } from "./use-home-agents";
import { useHomeLive } from "./use-home-live";
import { useHomeNetwork, type HomeNetworkModel } from "./use-home-network";
import { useHomePrefsActions, useHomeSystemOpen, useHomeUsageWindow } from "./use-home-prefs-store";
import { useHomeSystem, type HomeSystemModel } from "./use-home-system";
import { useHomeWorkingNow } from "./use-home-working-now";

export interface HomeDashboardModel {
  scope: HomeScope;
  connectionStatus: ConnectionStatus;
  usageWindow: HomeUsageWindow;
  setUsageWindow: (window: HomeUsageWindow) => void;
  overview: HomeOverview | undefined;
  overviewStatus: HomeSurfaceStatus;
  overviewErrorMessage: string | null;
  activity: HomeActivityEvent[] | undefined;
  activityStatus: HomeSurfaceStatus;
  activityErrorMessage: string | null;
  activeWorkspaceName: string | null;
  workingNow: HomeWorkingNowModel;
  network: HomeNetworkModel;
  agents: HomeAgentsModel;
  system: HomeSystemModel;
  systemOpen: boolean;
  setSystemOpen: (open: boolean) => void;
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

export function useHomeDashboard(): HomeDashboardModel {
  const { connectionStatus } = useDaemonHealth();
  const { activeWorkspace } = useActiveWorkspace();
  // Await daemon-owned home metadata before selecting global or workspace scope.
  const daemonStatus = useDaemonStatus();
  const userHomeDir = daemonStatus.data?.user_home_dir ?? undefined;
  const usageWindow = useHomeUsageWindow();
  const systemOpen = useHomeSystemOpen();
  const { setUsageWindow, setSystemOpen } = useHomePrefsActions();

  const resolvedScope = homeScopeForActiveWorkspace(activeWorkspace, userHomeDir);
  const scopeSettled = resolvedScope !== null;
  const scope = resolvedScope ?? UNSETTLED_HOME_SCOPE;

  const overviewQuery = useQuery(
    homeOverviewOptions(
      {
        workspace: scope.workspaceParam || undefined,
        usageWindow,
      },
      scopeSettled
    )
  );
  const activityQuery = useQuery(
    homeActivityOptions({ workspace_id: scope.workspaceParam || undefined }, scopeSettled)
  );

  useHomeLive({ workspaceId: scope.workspaceParam, enabled: scopeSettled });

  const workingNow = useHomeWorkingNow(scope, scopeSettled);
  const overview = overviewQuery.data;
  const network = useHomeNetwork(overview?.network.messages_today);
  const agents = useHomeAgents();
  const system = useHomeSystem(
    overview?.system.hook_runs_today,
    overview?.system.hook_failures_today,
    overview?.system.retention_days
  );

  return {
    scope,
    connectionStatus,
    usageWindow,
    setUsageWindow,
    overview,
    overviewStatus: surfaceStatus(
      overviewQuery.isLoading || !scopeSettled,
      overviewQuery.isError,
      overviewQuery.data !== undefined
    ),
    overviewErrorMessage: overviewQuery.error instanceof Error ? overviewQuery.error.message : null,
    activity: activityQuery.data,
    activityStatus: surfaceStatus(
      activityQuery.isLoading || !scopeSettled,
      activityQuery.isError,
      activityQuery.data !== undefined
    ),
    activityErrorMessage: activityQuery.error instanceof Error ? activityQuery.error.message : null,
    activeWorkspaceName: activeWorkspace?.name ?? null,
    workingNow,
    network,
    agents,
    system,
    systemOpen,
    setSystemOpen,
  };
}

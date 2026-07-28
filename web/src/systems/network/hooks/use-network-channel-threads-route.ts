import { useActiveNetworkSession, type UseActiveNetworkSessionResult } from "./use-active-session";
import { useThreadViewMode, type ThreadViewMode } from "./use-thread-view-mode";

export interface UseNetworkChannelThreadsRouteArgs {
  workspaceId: string;
  channel: string;
  activeThreadId: string | null;
  view?: "full";
}

export interface UseNetworkChannelThreadsRouteResult {
  activeThreadId: string | null;
  viewMode: ThreadViewMode;
  isFullPage: boolean;
  showOverlay: boolean;
  showList: boolean;
  activeSession: UseActiveNetworkSessionResult;
}

export function useNetworkChannelThreadsRoute({
  workspaceId,
  channel,
  activeThreadId,
  view,
}: UseNetworkChannelThreadsRouteArgs): UseNetworkChannelThreadsRouteResult {
  const viewMode = useThreadViewMode();
  const isFullPage = view === "full" || viewMode === "fullpage";
  const showOverlay = activeThreadId != null;
  return {
    activeThreadId,
    viewMode,
    isFullPage,
    showOverlay,
    showList: !showOverlay || !isFullPage,
    activeSession: useActiveNetworkSession(channel, { workspaceId }),
  };
}

import { useNetworkCreateChannelAction } from "./use-network-create-channel-action";
import { useNetworkInspectorView } from "./use-network-inspector-view";
import { useNetworkRailView } from "./use-network-rail-view";
import { useNetworkRouteShell } from "./use-network-route-shell";
import { useOpenWork } from "./use-work";
import { useThreadViewMode } from "./use-thread-view-mode";
import { useNetworkThreadDetail } from "./use-threads";
import { useNetworkDirectDetail } from "./use-directs";
import { resolveNetworkConversationLabel } from "../lib/network-window-location";
import type { UseNetworkRouteShellArgs } from "./use-network-route-shell";

export function useNetworkRouteView(args: UseNetworkRouteShellArgs) {
  const route = useNetworkRouteShell(args);
  const viewMode = useThreadViewMode();
  const threadIsFullPage = route.location.view === "full" || viewMode === "fullpage";
  const showOverlayInRightRail = route.activeThreadId != null && !threadIsFullPage;
  const containerSurface = route.activeThreadId
    ? ("thread" as const)
    : route.activeDirectId
      ? ("direct" as const)
      : null;
  const containerId = route.activeThreadId ?? route.activeDirectId ?? null;
  const channelKey = route.activeChannel?.channel ?? null;
  const threadDetail = useNetworkThreadDetail(channelKey, route.activeThreadId, {
    workspaceId: route.activeWorkspaceId,
    enabled: route.activeThreadId != null,
  });
  const directDetail = useNetworkDirectDetail(channelKey, route.activeDirectId, {
    workspaceId: route.activeWorkspaceId,
    enabled: route.activeDirectId != null,
  });
  const exactOpenCount =
    threadDetail.thread?.open_work_count ?? directDetail.direct?.open_work_count;
  const openWork = useOpenWork({
    workspaceId: route.activeWorkspaceId,
    channel: channelKey,
    surface: containerSurface,
    containerId,
    enabled: Boolean(channelKey) && containerSurface != null,
    exactOpenCount,
  });
  const inspectorView = useNetworkInspectorView({
    workspaceId: route.activeWorkspaceId,
    channel: channelKey,
    enabled: Boolean(channelKey) && !showOverlayInRightRail,
  });
  const railView = useNetworkRailView({
    workspaceId: route.activeWorkspaceId,
    channel: channelKey,
  });
  const networkCreate = useNetworkCreateChannelAction({
    enabled: route.page.isNetworkEnabled,
    workspaceId: route.activeWorkspaceId,
  });
  const conversationLabel = resolveNetworkConversationLabel(route.location, {
    thread: threadDetail.thread,
    direct: directDetail.direct,
    selfSessionId: railView.session.session?.sessionId ?? null,
  });

  return {
    ...route,
    inspectorView,
    networkCreate,
    conversationLabel,
    directDetail,
    openWork,
    railView,
    showOverlayInRightRail,
    threadDetail,
    threadIsFullPage,
  };
}

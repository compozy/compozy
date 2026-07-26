import { Skeleton, SkeletonRows } from "@agh/ui";

import { DaemonDown } from "../components/empty-states/daemon-down";
import { NetworkEmpty } from "../components/empty-states/network-empty";
import { NetworkInspector } from "../components/shell/network-inspector";
import { NetworkShell } from "../components/shell/network-shell";
import { ThreadOverlay } from "../components/thread-overlay/thread-overlay";
import { NetworkListFiltersProvider } from "../contexts/network-list-filters-context";
import { useNetworkListFilters } from "../hooks/use-network-list-filters";
import { useNetworkRouteView } from "../hooks/use-network-route-view";
import { useNetworkWindowTopbar } from "../hooks/use-network-window-topbar";
import { networkThreadsLocation, type NetworkWindowLocation } from "../lib/network-window-location";
import type { NetworkWindowNavigation } from "../hooks/use-network-route-shell";
import { NetworkWindowContent } from "./network-window-content";

export interface NetworkWindowControllerProps {
  active: boolean;
  location: NetworkWindowLocation;
  navigation: NetworkWindowNavigation;
}

/**
 * Network's router boundary now consumes the logical window location. The
 * domain keeps its nested route composition here while the OS adapter owns
 * the WM read and the routing coordinator remains the only URL writer.
 */
export function NetworkWindowController({
  active,
  location,
  navigation,
}: NetworkWindowControllerProps) {
  const view = useNetworkRouteView({ active, location, navigation });
  const { page, activeChannel, activeTab, activeThreadId, activeDirectId, hasUnread } = view;
  const workspaceId = view.activeWorkspaceId ?? "";
  const activeChannelKey = activeChannel?.channel ?? null;
  const filters = useNetworkListFilters({
    workspaceId,
    channel: activeChannelKey ?? "",
    enabled: Boolean(activeChannelKey),
  });

  const closeThread = () => {
    if (!workspaceId || !activeChannelKey) return;
    navigation.push(networkThreadsLocation(workspaceId, activeChannelKey));
  };
  const openThreadMain = () => {
    if (!workspaceId || !activeChannelKey || !activeThreadId) return;
    navigation.push(networkThreadsLocation(workspaceId, activeChannelKey, activeThreadId, "full"));
  };

  useNetworkWindowTopbar({
    location: view.location,
    navigation,
    channelCount: page.channels.length,
    status: page.status,
    openWorkCount: view.openWork.openCount,
    workEntries: view.openWork.entries,
    createAction: view.networkCreate.action,
    conversationLabel: view.conversationLabel,
  });

  if (page.isStatusLoading) {
    return (
      <>
        <div
          aria-label="Loading network workspace"
          className="flex min-h-0 flex-1"
          data-testid="network-loading"
          role="status"
        >
          <div className="hidden w-[232px] shrink-0 flex-col gap-2 border-r border-line-soft px-3 py-4 sm:flex">
            <Skeleton className="h-3 w-16" />
            <SkeletonRows count={5} rowClassName="py-1">
              <Skeleton className="h-3.5 w-full" />
            </SkeletonRows>
          </div>
          <div className="flex min-w-0 flex-1 flex-col gap-3 px-5 py-4">
            <Skeleton className="h-5 w-56" />
            <Skeleton className="h-3 w-80" />
            <SkeletonRows count={4} rowClassName="border-b border-line-soft py-3">
              <Skeleton className="h-3.5 w-2/3" />
              <Skeleton className="h-3 w-full" />
            </SkeletonRows>
          </div>
        </div>
        {view.networkCreate.dialog}
      </>
    );
  }

  if (page.statusError || !page.status) {
    return (
      <>
        <div
          className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
          data-testid="network-error"
        >
          <DaemonDown />
        </div>
        {view.networkCreate.dialog}
      </>
    );
  }

  if (page.isNetworkDisabled) {
    return (
      <>
        <div
          className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
          data-testid="network-disabled-state"
        >
          <NetworkEmpty disabledByAdmin onOpenSettings={view.networkCreate.openNetworkSettings} />
        </div>
        {view.networkCreate.dialog}
      </>
    );
  }

  if (page.channels.length === 0 && !page.isChannelsLoading) {
    return (
      <>
        <NetworkListFiltersProvider value={filters}>
          <NetworkShell
            workspaceId={workspaceId}
            activeChannel={null}
            activeChannelDetail={null}
            activeDirectId={null}
            activeTab="threads"
            directCount={null}
            directs={[]}
            hasUnread={() => false}
            inspectorOpen={false}
            loading={{ channels: false, directs: false, recents: false }}
            isPinned={() => false}
            onInspectorToggle={() => undefined}
            onTogglePinned={page.togglePinned}
            openWorkCount={0}
            pinnedChannels={[]}
            recents={[]}
            rightRailMode="thread"
            rightRailOpen={false}
            selfSessionId={null}
            threadCount={null}
            unpinnedChannels={[]}
          >
            <div
              className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
              data-testid="network-no-channels-state"
            >
              <NetworkEmpty
                className="max-w-xl"
                onOpenSettings={view.networkCreate.openNetworkSettings}
              />
            </div>
          </NetworkShell>
        </NetworkListFiltersProvider>
        {view.networkCreate.dialog}
      </>
    );
  }

  const { inspector, members: channelMembers } = view.inspectorView;
  const showInspectorInRightRail = !view.showOverlayInRightRail && inspector.open;
  const rightRailContent =
    view.showOverlayInRightRail && activeChannel && activeThreadId ? (
      <ThreadOverlay
        workspaceId={workspaceId}
        channel={activeChannel.channel}
        fullPage={false}
        onClose={closeThread}
        onOpenMain={openThreadMain}
        threadId={activeThreadId}
      />
    ) : showInspectorInRightRail && activeChannel ? (
      <NetworkInspector
        activeTab={inspector.tab}
        channel={activeChannel}
        isMembersLoading={channelMembers.isLoading}
        isWorkLoading={view.openWork.isLoading}
        members={channelMembers.members}
        onTabChange={inspector.setTab}
        workCount={view.openWork.openCount}
        workEntries={view.openWork.entries}
      />
    ) : null;

  const threadCount = activeChannelKey ? filters.threadsQuery.total : null;
  const directCount = activeChannelKey ? filters.directsQuery.total : null;

  return (
    <>
      <NetworkListFiltersProvider value={filters}>
        <NetworkShell
          workspaceId={workspaceId}
          activeChannel={activeChannel}
          activeChannelDetail={null}
          activeDirectId={activeDirectId}
          activeTab={activeTab}
          directCount={directCount}
          directs={view.railView.directs.directs}
          hasUnread={hasUnread}
          inspectorOpen={inspector.open}
          loading={{
            channels: page.isChannelsLoading,
            directs: view.railView.directs.isLoading,
            recents: page.isRecentsLoading,
          }}
          isPinned={page.isPinned}
          onInspectorToggle={inspector.toggle}
          onTogglePinned={page.togglePinned}
          openWorkCount={view.openWork.openCount}
          pinnedChannels={page.pinnedChannels}
          recents={page.recents}
          rightRailContent={rightRailContent}
          rightRailMode={view.showOverlayInRightRail ? "thread" : "inspector"}
          rightRailOpen={view.showOverlayInRightRail || showInspectorInRightRail}
          selfSessionId={view.railView.session.session?.sessionId ?? null}
          threadCount={threadCount}
          unpinnedChannels={page.unpinnedChannels}
        >
          {activeChannel && workspaceId ? (
            <NetworkWindowContent
              workspaceId={workspaceId}
              channel={activeChannel.channel}
              fanoutPolicy={activeChannel.fanout_policy ?? null}
              activeTab={activeTab}
              activeThreadId={activeThreadId}
              activeDirectId={activeDirectId}
              threadView={view.location.view}
              onCloseThread={closeThread}
              onOpenThreadMain={openThreadMain}
            />
          ) : null}
        </NetworkShell>
      </NetworkListFiltersProvider>
      {view.networkCreate.dialog}
    </>
  );
}

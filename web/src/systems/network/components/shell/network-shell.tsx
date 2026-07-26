import type { ReactNode } from "react";

import type {
  NetworkChannel,
  NetworkChannelSummary,
  NetworkDirectRoomSummary,
  NetworkRecentEntry,
} from "../../types";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
  RightRail,
  type RightRailMode,
  useDefaultLayout,
  WIDTH_DETAIL_INSPECTOR_INLINE,
  WIDTH_MESSAGE_BUBBLE_MAX,
  WIDTH_RIGHT_RAIL_DEFAULT,
  WIDTH_TABLE_CELL_LG,
} from "@agh/ui";

import { ChannelHeader } from "./channel-header";
import { ChannelRail } from "./channel-rail";
import type { ChannelTab } from "./channel-tabs-types";
import { ChannelToolbar } from "./channel-toolbar";

const MAIN_PANEL_ID = "network-main";
const RAIL_PANEL_ID = "network-rail";
const RAIL_PANEL_IDS = [MAIN_PANEL_ID, RAIL_PANEL_ID];

export interface NetworkShellProps {
  workspaceId: string;
  pinnedChannels: ReadonlyArray<NetworkChannelSummary>;
  unpinnedChannels: ReadonlyArray<NetworkChannelSummary>;
  recents: ReadonlyArray<NetworkRecentEntry>;
  directs: ReadonlyArray<NetworkDirectRoomSummary>;
  loading: {
    channels: boolean;
    recents: boolean;
    directs: boolean;
  };
  activeChannel: NetworkChannelSummary | null;
  activeChannelDetail: NetworkChannel | null;
  activeTab: ChannelTab;
  activeDirectId: string | null;
  selfSessionId: string | null;
  threadCount: number | null;
  directCount: number | null;
  openWorkCount: number;
  rightRailOpen: boolean;
  rightRailMode: RightRailMode;
  rightRailContent?: ReactNode;
  inspectorOpen: boolean;
  onInspectorToggle: () => void;
  isPinned: (channel: string) => boolean;
  onTogglePinned: (channel: string) => void;
  hasUnread: (channel: string) => boolean;
  children: ReactNode;
}

export function NetworkShell({
  workspaceId,
  pinnedChannels,
  unpinnedChannels,
  recents,
  directs,
  loading,
  activeChannel,
  activeChannelDetail,
  activeTab,
  activeDirectId,
  selfSessionId,
  threadCount,
  directCount,
  openWorkCount,
  rightRailOpen,
  rightRailMode,
  rightRailContent,
  inspectorOpen,
  onInspectorToggle,
  isPinned,
  onTogglePinned,
  hasUnread,
  children,
}: NetworkShellProps) {
  const storage = typeof window !== "undefined" ? window.localStorage : undefined;
  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "network:rail-layout",
    panelIds: RAIL_PANEL_IDS,
    storage,
  });

  return (
    <div className="flex min-h-0 flex-1 bg-canvas" data-testid="network-shell">
      <ChannelRail
        workspaceId={workspaceId}
        activeChannel={activeChannel?.channel ?? null}
        activeDirectId={activeDirectId}
        directs={directs}
        hasUnread={hasUnread}
        loading={loading}
        isPinned={isPinned}
        onTogglePinned={onTogglePinned}
        pinnedChannels={pinnedChannels}
        recents={recents}
        selfSessionId={selfSessionId}
        unpinnedChannels={unpinnedChannels}
      />

      <ResizablePanelGroup
        className="min-w-0 flex-1"
        defaultLayout={defaultLayout}
        onLayoutChanged={onLayoutChanged}
        orientation="horizontal"
      >
        <ResizablePanel
          className="flex min-h-0 min-w-0 flex-col"
          id={MAIN_PANEL_ID}
          minSize={WIDTH_TABLE_CELL_LG}
        >
          <main className="flex min-h-0 min-w-0 flex-1 flex-col" data-testid="network-main-pane">
            {activeChannel ? (
              <>
                <ChannelHeader
                  workspaceId={workspaceId}
                  channel={activeChannel}
                  detail={activeChannelDetail}
                  openWorkCount={openWorkCount}
                  threadCount={threadCount}
                />
                <ChannelToolbar
                  workspaceId={workspaceId}
                  activeTab={activeTab}
                  channel={activeChannel}
                  detail={activeChannelDetail}
                  directCount={directCount}
                  inspectorOpen={inspectorOpen}
                  onInspectorToggle={onInspectorToggle}
                  threadCount={threadCount}
                />
              </>
            ) : null}

            <div className="flex min-h-0 flex-1 flex-col" data-testid="network-tab-panel">
              {children}
            </div>
          </main>
        </ResizablePanel>

        {rightRailOpen ? (
          <>
            <ResizableHandle withHandle />
            <ResizablePanel
              className="flex min-h-0 min-w-0 flex-col"
              defaultSize={WIDTH_RIGHT_RAIL_DEFAULT}
              id={RAIL_PANEL_ID}
              maxSize={WIDTH_MESSAGE_BUBBLE_MAX}
              minSize={WIDTH_DETAIL_INSPECTOR_INLINE}
            >
              <RightRail mode={rightRailMode} open={rightRailOpen}>
                {rightRailContent}
              </RightRail>
            </ResizablePanel>
          </>
        ) : null}
      </ResizablePanelGroup>
    </div>
  );
}

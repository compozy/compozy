import { shallowEqual } from "@xstate/store";

import {
  leadWindowId as tabLeadWindowId,
  resolveActiveTabId,
  tabWindowIds,
} from "../lib/tab-model";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import { useDesktop } from "./use-desktop";
import {
  useDesktopOverviewDesktopAction,
  useDesktopOverviewSegmentRequest,
  usePendingWindowManagerCommand,
  useWindowManagerConflict,
  useWindowManagerDiagnostic,
  useWindowManagerOverlay,
} from "./use-window-manager-store";

/** Runtime inputs for the desktop overview and connection diagnostic surfaces. */
export function useDesktopManagerSurfaces() {
  const overlay = useWindowManagerOverlay();
  const overviewSegmentRequest = useDesktopOverviewSegmentRequest();
  const overviewDesktopAction = useDesktopOverviewDesktopAction();
  const conflict = useWindowManagerConflict();
  const diagnostic = useWindowManagerDiagnostic();
  const pending = usePendingWindowManagerCommand();
  const hydration = useDesktop(state => state.hydration);
  const connectionStatus = useDesktop(state => state.connectionStatus);
  const activeDesktopId = useDesktop(state => state.activeDesktopId);
  const canMutate = useDesktop(windowManagerCommandsAvailable);
  const overviewOpen = overlay?.kind === "desktops-overview";
  const projection = useDesktop(
    state =>
      overviewOpen
        ? {
            desktops: state.desktops,
            activeTabIds: state.activeTabIds,
            windows: state.windows,
            focusOrder: state.client?.focusOrder,
          }
        : null,
    shallowEqual
  );
  const focusOrder = projection?.focusOrder ?? [];
  const desktops = (projection?.desktops ?? []).map(desktop => {
    const activeTabId = projection?.activeTabIds[desktop.id] ?? resolveActiveTabId(desktop, null);
    const tabs = desktop.tabs.flatMap(tab => {
      const leadWindowId = tabLeadWindowId(tab, focusOrder);
      if (leadWindowId === null) return [];
      const ordered = [leadWindowId, ...tabWindowIds(tab).filter(id => id !== leadWindowId)];
      const members = ordered.flatMap(id => {
        const win = projection?.windows[id];
        return win ? [win] : [];
      });
      return members.length === 0 ? [] : [{ id: tab.id, leadWindowId, members }];
    });
    // Read from the daemon's tabs, not the projected ones: a tab whose app this web does not know
    // is dropped from `tabs` but still moves when the desktop closes. Only a tab holding one blank
    // New Tab has nothing to move (R2-08, FB-F-07).
    const movableTabCount = desktop.tabs.filter(tab => {
      const windowIds = tabWindowIds(tab);
      return !(windowIds.length === 1 && projection?.windows[windowIds[0]!]?.app === "new-tab");
    }).length;
    return {
      id: desktop.id,
      name: desktop.name,
      tabs,
      movableTabCount,
      activeTabId,
      // The thumbnail draws the tab this client shows on the desktop.
      activeRoot: desktop.tabs.find(tab => tab.id === activeTabId)?.root ?? null,
    };
  });

  return {
    activeDesktopId,
    canMutate,
    connectionStatus,
    conflict,
    desktops,
    diagnostic,
    hydration,
    overlay,
    overviewDesktopAction,
    overviewSegmentRequest,
    pending,
  };
}

export type DesktopManagerSurfacesModel = ReturnType<typeof useDesktopManagerSurfaces>;

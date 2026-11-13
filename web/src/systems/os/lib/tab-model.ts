import { descendantWindowIds } from "./layout-minimums";
import type {
  DesktopId,
  LayoutDesktop,
  LayoutTab,
  TabId,
  WindowId,
} from "./window-manager-types";

/** Every window of a tab, in reading order. */
export function tabWindowIds(tab: LayoutTab): WindowId[] {
  return descendantWindowIds(tab.root);
}

/** A tab holds more than one window: its windows sit side by side in a split. */
export function tabIsSplit(tab: LayoutTab): boolean {
  return tab.root.kind === "split";
}

/**
 * The tab this client shows on a desktop: its own choice while that tab still exists, else the
 * desktop's durable default, else the first tab (the daemon's repair rule).
 */
export function resolveActiveTabId(
  desktop: LayoutDesktop,
  clientActiveTab: Readonly<Record<DesktopId, TabId>> | null | undefined
): TabId | null {
  const own = clientActiveTab?.[desktop.id];
  if (own !== undefined && desktop.tabs.some(tab => tab.id === own)) return own;
  if (desktop.tabs.some(tab => tab.id === desktop.activeTabId)) return desktop.activeTabId;
  return desktop.tabs[0]?.id ?? null;
}

/**
 * The window that names a tab and takes focus when it activates: the member this client focused
 * last, else the tab's first window.
 */
export function leadWindowId(tab: LayoutTab, focusOrder: readonly WindowId[]): WindowId | null {
  const members = tabWindowIds(tab);
  return focusOrder.find(windowId => members.includes(windowId)) ?? members[0] ?? null;
}

export function findTab(
  desktops: readonly LayoutDesktop[],
  tabId: TabId
): { desktop: LayoutDesktop; tab: LayoutTab; index: number } | null {
  for (const desktop of desktops) {
    const index = desktop.tabs.findIndex(tab => tab.id === tabId);
    const tab = desktop.tabs[index];
    if (tab !== undefined) return { desktop, tab, index };
  }
  return null;
}

/** The number of pinned tabs that lead a desktop's bar. */
export function pinnedTabCount(desktop: LayoutDesktop): number {
  return desktop.tabs.filter(tab => tab.pinned).length;
}

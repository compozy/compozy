import type { OsCloseScope, OsDesktopRuntimeStore, OsWindow } from "./os-types";
import { tabWindowIds } from "./tab-model";

/**
 * Mirrors the daemon's close scopes: `tab` closes the one window (refused for the only window of
 * a pinned tab), `group` every window of its tab, and the bar scopes every unpinned tab they reach.
 */
export function windowCloseTargets(
  state: OsDesktopRuntimeStore,
  windowId: string,
  scope: OsCloseScope
): readonly OsWindow[] {
  const window = state.windows[windowId];
  if (!window) return [];
  const desktop = state.desktops.find(candidate => candidate.id === window.desktopId);
  const tabIndex = desktop?.tabs.findIndex(tab => tab.id === window.tabId) ?? -1;
  const tab = desktop?.tabs[tabIndex];
  if (desktop === undefined || tab === undefined) return [];
  const members = (ids: readonly string[]) =>
    ids.flatMap(id => (state.windows[id] ? [state.windows[id]] : []));
  if (scope === "tab") return tab.pinned && !window.inSplit ? [] : [window];
  if (scope === "group") return members(tabWindowIds(tab));
  return desktop.tabs.flatMap((candidate, index) => {
    if (candidate.pinned) return [];
    const selected =
      scope === "all" ||
      (scope === "others" && index !== tabIndex) ||
      (scope === "right" && index > tabIndex) ||
      (scope === "left" && index < tabIndex);
    return selected ? members(tabWindowIds(candidate)) : [];
  });
}

export type WindowCloseGuard = (
  targets: readonly OsWindow[],
  isCurrent: () => boolean
) => Promise<boolean>;

import { shallowEqual } from "@xstate/store";
import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";

import { descendantWindowIds } from "../lib/layout-minimums";
import type {
  OsPresentation,
  OsViewportState,
  OsWindow,
  WindowManagerController,
} from "../lib/os-types";
import type { LayoutDesktop, LayoutTab, TabId } from "../lib/window-manager-types";
import { WindowPaneHosts } from "../lib/window-pane-hosts";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

export interface DeskDesktopModel {
  desktop: LayoutDesktop;
  active: boolean;
  activeTabId: TabId | null;
}

export interface OsDeskLayerModel {
  layerRef: RefObject<HTMLDivElement | null>;
  desktops: readonly DeskDesktopModel[];
  presentation: OsPresentation;
  viewportState: OsViewportState;
}

/**
 * The tabs a desktop has shown, so a tab the operator leaves stays mounted and returning to it
 * never reloads its windows. Closed tabs drop out; the active tab always counts, and so does
 * every `adopted` tab (one that received a window whose pane is mounted).
 */
export function useVisitedTabs(
  tabIds: readonly TabId[],
  activeTabId: TabId | null,
  adopted: readonly TabId[] = []
): readonly TabId[] {
  const [visited, setVisited] = useState<readonly TabId[]>(() =>
    activeTabId === null ? [] : [activeTabId]
  );
  const present = new Set(tabIds);
  const next = visited.filter(tabId => present.has(tabId));
  const kept = new Set(next);
  for (const tabId of activeTabId === null ? adopted : [activeTabId, ...adopted]) {
    if (!present.has(tabId) || kept.has(tabId)) continue;
    kept.add(tabId);
    next.push(tabId);
  }
  if (next.length !== visited.length || next.some((tabId, index) => tabId !== visited[index])) {
    setVisited(next);
  }
  // Bar order keeps sibling tab layers stable while the set changes.
  return tabIds.filter(tabId => kept.has(tabId));
}

export interface DeskTabPanes {
  /** The tabs the desktop keeps mounted (`useVisitedTabs`), in bar order. */
  visited: readonly TabId[];
  /** Every window of the visited tabs; each renders its one pane into its host. */
  paneWindowIds: readonly string[];
  /** The hosts the panes render into and the slots place; stable for the layer's life. */
  hosts: WindowPaneHosts;
}

/**
 * The tabs a desktop keeps mounted and one pane for every window in them, keyed by window, never
 * by tab or split position: the pane renders into the window's host (`WindowPaneHosts`) and the
 * split slot it occupies adopts the host, so a window changing tab or position never remounts.
 * A tab that receives a mounted window (a move into a new tab that is not shown) is kept mounted
 * from then on, so the window's pane stays mounted too.
 */
export function useDeskTabPanes(
  tabs: readonly LayoutTab[],
  shownTabId: TabId | null,
  manager: Pick<WindowManagerController, "getState">,
  windows: Readonly<Record<string, OsWindow>>
): DeskTabPanes {
  // The windows whose panes the last render mounted: a tab holding one now adopts it.
  const [mounted, setMounted] = useState<readonly string[]>([]);
  const held = new Set(mounted);
  const adopted: TabId[] = [];
  for (const tab of tabs) {
    if (descendantWindowIds(tab.root).some(windowId => held.has(windowId))) adopted.push(tab.id);
  }
  const visited = useVisitedTabs(
    tabs.map(tab => tab.id),
    shownTabId,
    adopted
  );
  // Read at the moment a host acts, never from a render: focus may have moved since.
  const [hosts] = useState(
    () =>
      new WindowPaneHosts({
        isFocused: windowId => manager.getState().focusedId === windowId,
        routeOf: windowId => manager.getState().windows[windowId]?.route ?? null,
      })
  );
  const paneWindowIds = visited.flatMap(tabId => {
    const tab = tabs.find(entry => entry.id === tabId);
    return tab === undefined ? [] : descendantWindowIds(tab.root);
  });
  const paneKey = paneWindowIds.join("\n");
  if (paneKey !== mounted.join("\n")) setMounted(paneWindowIds);
  useEffect(() => {
    hosts.retain(new Set(paneKey === "" ? [] : paneKey.split("\n")));
  }, [hosts, paneKey]);
  useEffect(() => () => hosts.dispose(), [hosts]);
  // A window's route change, read in the commit that rendered it, before a later move or hide.
  useLayoutEffect(() => hosts.syncRoutes(), [hosts, windows]);
  return { visited, paneWindowIds, hosts };
}

/** Measures the one shared content area and lists each desktop with its active tab. */
export function useOsDeskLayer(): OsDeskLayerModel {
  const { manager } = useOsShell();
  const layerRef = useRef<HTMLDivElement | null>(null);
  const presentation = useDesktop(state => state.presentation);
  const viewportState = useDesktop(state => state.viewportState);
  const view = useDesktop(
    state => ({
      activeDesktopId: state.activeDesktopId,
      ready: state.windowManagerConfig !== null,
      desktops: state.desktops,
      activeTabIds: state.activeTabIds,
    }),
    shallowEqual
  );
  // A snapshot can precede its config; mount no desktop before its projection exists.
  const desktops = (view.ready ? view.desktops : []).map(desktop => ({
    desktop,
    active: desktop.id === view.activeDesktopId,
    activeTabId: view.activeTabIds[desktop.id] ?? null,
  }));

  useEffect(() => {
    const layer = layerRef.current;
    if (!layer) return;
    const measure = (size?: { width: number; height: number }) => {
      const bounds = layer.getBoundingClientRect();
      manager.setDesktopBounds({
        viewportWidth: window.innerWidth,
        width: size?.width ?? bounds.width,
        height: size?.height ?? bounds.height,
        origin: { x: bounds.left, y: bounds.top },
      });
    };
    const observer = new ResizeObserver(entries => {
      const entry = entries[0];
      if (!entry) return;
      measure(entry.contentRect);
    });
    const refreshOrigin = () => measure();
    observer.observe(layer);
    measure();
    window.addEventListener("resize", refreshOrigin);
    window.addEventListener("orientationchange", refreshOrigin);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", refreshOrigin);
      window.removeEventListener("orientationchange", refreshOrigin);
    };
  }, [manager]);

  return { layerRef, desktops, presentation, viewportState };
}

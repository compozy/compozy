import { shallowEqual } from "@xstate/store";
import { useEffect, useRef, useState, type RefObject } from "react";

import { descendantWindowIds } from "../lib/layout-minimums";
import type { OsPresentation, OsViewportState } from "../lib/os-types";
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
 * never reloads its windows. Closed tabs drop out; the active tab always counts.
 */
export function useVisitedTabs(
  tabIds: readonly TabId[],
  activeTabId: TabId | null
): readonly TabId[] {
  const [visited, setVisited] = useState<readonly TabId[]>(() =>
    activeTabId === null ? [] : [activeTabId]
  );
  const present = new Set(tabIds);
  const live = visited.filter(tabId => present.has(tabId));
  const next =
    activeTabId !== null && present.has(activeTabId) && !live.includes(activeTabId)
      ? [...live, activeTabId]
      : live;
  if (next.length !== visited.length || next.some((tabId, index) => tabId !== visited[index])) {
    setVisited(next);
  }
  // Bar order keeps sibling tab layers stable while the set changes.
  const kept = new Set(next);
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
 */
export function useDeskTabPanes(
  tabs: readonly LayoutTab[],
  shownTabId: TabId | null
): DeskTabPanes {
  const visited = useVisitedTabs(
    tabs.map(tab => tab.id),
    shownTabId
  );
  const [hosts] = useState(() => new WindowPaneHosts());
  const paneWindowIds = visited.flatMap(tabId => {
    const tab = tabs.find(entry => entry.id === tabId);
    return tab === undefined ? [] : descendantWindowIds(tab.root);
  });
  const paneKey = paneWindowIds.join("\n");
  useEffect(() => {
    hosts.retain(new Set(paneKey === "" ? [] : paneKey.split("\n")));
  }, [hosts, paneKey]);
  useEffect(() => () => hosts.dispose(), [hosts]);
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

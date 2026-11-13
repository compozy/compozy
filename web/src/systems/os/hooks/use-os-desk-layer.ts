import { shallowEqual } from "@xstate/store";
import { useEffect, useRef, useState, type RefObject } from "react";

import type { OsPresentation, OsViewportState } from "../lib/os-types";
import type { LayoutDesktop, TabId } from "../lib/window-manager-types";
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
  const live = visited.filter(tabId => tabIds.includes(tabId));
  const next =
    activeTabId !== null && tabIds.includes(activeTabId) && !live.includes(activeTabId)
      ? [...live, activeTabId]
      : live;
  if (next.length !== visited.length || next.some((tabId, index) => tabId !== visited[index])) {
    setVisited(next);
  }
  // Bar order keeps sibling tab layers stable while the set changes.
  return tabIds.filter(tabId => next.includes(tabId));
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

import { useEffect, useRef, type RefObject } from "react";
import { useShallow } from "zustand/shallow";

import type { LayoutDesktop, LayoutProjection } from "../lib/window-manager-types";
import type { OsPresentation, OsViewportState } from "../lib/os-types";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

export interface DesktopLayerModel {
  desktop: LayoutDesktop;
  windowIds: readonly string[];
  active: boolean;
  anyVisible: boolean;
}

export interface OsWinLayerModel {
  layerRef: RefObject<HTMLDivElement | null>;
  desktops: readonly DesktopLayerModel[];
  presentation: OsPresentation;
  viewportState: OsViewportState;
  activeProjection: LayoutProjection | undefined;
}

/** Measures one shared work area and groups the Query projection by desktop. */
export function useOsWinLayer(): OsWinLayerModel {
  const { store } = useOsShell();
  const layerRef = useRef<HTMLDivElement | null>(null);
  const presentation = useDesktop(state => state.presentation);
  const viewportState = useDesktop(state => state.viewportState);
  const projection = useDesktop(
    useShallow(state => ({
      activeDesktopId: state.activeDesktopId,
      desktops: state.desktops,
      projections: state.projections,
      windows: state.windows,
    }))
  );
  const windowIdsByDesktop = new Map<string, string[]>();
  for (const window of Object.values(projection.windows)) {
    const windowIds = windowIdsByDesktop.get(window.desktopId) ?? [];
    windowIds.push(window.id);
    windowIdsByDesktop.set(window.desktopId, windowIds);
  }
  const desktops = projection.desktops.map(desktop => ({
    desktop,
    active: desktop.id === projection.activeDesktopId,
    windowIds: windowIdsByDesktop.get(desktop.id) ?? [],
    anyVisible: (windowIdsByDesktop.get(desktop.id) ?? []).some(
      windowId => !projection.windows[windowId]?.minimized
    ),
  }));

  useEffect(() => {
    const layer = layerRef.current;
    if (!layer) return;
    const measure = (size?: { width: number; height: number }) => {
      const bounds = layer.getBoundingClientRect();
      store.getState().setDesktopBounds({
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
  }, [presentation, store, viewportState]);

  return {
    layerRef,
    desktops,
    presentation,
    viewportState,
    activeProjection: projection.activeDesktopId
      ? projection.projections[projection.activeDesktopId]
      : undefined,
  };
}

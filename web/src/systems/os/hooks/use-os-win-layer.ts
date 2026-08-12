import { useEffect, useRef, type RefObject } from "react";
import { shallowEqual } from "@xstate/store";

import type { OsWindowFrameModel } from "../lib/group-projection";
import type { LayoutDesktop, LayoutProjection } from "../lib/window-manager-types";
import type { OsPresentation, OsViewportState } from "../lib/os-types";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

export interface DesktopLayerModel {
  desktop: LayoutDesktop;
  frames: readonly OsWindowFrameModel[];
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

/** Measures one shared work area and groups the frame projection by desktop. */
export function useOsWinLayer(): OsWinLayerModel {
  const { manager } = useOsShell();
  const layerRef = useRef<HTMLDivElement | null>(null);
  const presentation = useDesktop(state => state.presentation);
  const viewportState = useDesktop(state => state.viewportState);
  const projection = useDesktop(
    state => ({
      activeDesktopId: state.activeDesktopId,
      desktops: state.desktops,
      projections: state.projections,
      frames: state.frames,
    }),
    shallowEqual
  );
  const desktops = projection.desktops.map(desktop => {
    const frames = projection.frames[desktop.id] ?? [];
    return {
      desktop,
      active: desktop.id === projection.activeDesktopId,
      frames,
      anyVisible: frames.some(frame => !frame.minimized),
    };
  });

  useEffect(() => {
    const layer = layerRef.current;
    if (!layer) return;
    const measure = (size?: { width: number; height: number }) => {
      const bounds = layer.getBoundingClientRect();
      manager.setDesktopBounds({
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

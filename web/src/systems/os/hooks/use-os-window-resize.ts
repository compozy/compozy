import type { RndResizeCallback, RndResizeStartCallback } from "react-rnd";

import { getOsAppMinimum, OS_WINDOW_CONSERVATIVE_MINIMUM } from "../lib/app-registry";
import type { OsWindowFrameModel } from "../lib/group-projection";
import type { OsRect, WindowManagerCommandOutcome, WindowManagerController } from "../lib/os-types";
import {
  directionEdges,
  resizedTiledZone,
  tiledZoneLimits,
  zoneOverlapsAny,
  type TiledResizeDirection,
} from "../lib/tiled-resize";
import { windowManagerLayoutArea } from "../lib/window-manager-layout-area";
import type { NormalizedRect, PixelRect, PixelSize } from "../lib/window-manager-types";
import { windowManagerStore } from "../stores/window-manager-store";

export interface OsWindowResizeDeps {
  manager: WindowManagerController;
  activeId: string;
  movesAsUnit: boolean;
  nextGestureAttempt: () => number;
  trackGestureOutcome: (
    rect: OsRect,
    outcome: WindowManagerCommandOutcome,
    gestureAttempt: number
  ) => void;
  resizeMax: { width: number; height: number } | null;
  setResizeMax: (maximum: { width: number; height: number } | null) => void;
  scheduleAuthoritativeRollback: (gestureAttempt: number) => void;
}

export interface OsWindowResizeModel {
  /** Live pixel ceiling for the active tiled edge drag; null when unconstrained. */
  resizeMax: { width: number; height: number } | null;
  handleResizeStart: RndResizeStartCallback;
  handleResizeStop: RndResizeCallback;
}

/**
 * Edge/corner resize for one frame: floating frames commit their rect, tiled
 * frames commit `window.resize` for the unit's zone — free edges only, growth
 * clamped at the nearest island both live (rnd max) and at the commit.
 */
export function useOsWindowResize(
  frame: OsWindowFrameModel,
  deps: OsWindowResizeDeps
): OsWindowResizeModel {
  const { manager, activeId } = deps;

  const layoutAreaRect = (): PixelRect | null => {
    const area = windowManagerStore.getSnapshot().context.workArea?.rect;
    const config = manager.getState().windowManagerConfig;
    if (area === undefined || config === null) return null;
    const layoutArea = windowManagerLayoutArea(area, config.gaps);
    return layoutArea.w > 0 && layoutArea.h > 0 ? layoutArea : null;
  };

  const tiledObstacles = (): NormalizedRect[] => {
    const frames = manager.getState().frames[frame.desktopId] ?? [];
    return frames.flatMap(candidate =>
      candidate.kind === "tiled" && candidate.id !== frame.id && candidate.zone !== null
        ? [candidate.zone]
        : []
    );
  };

  const frameMinimumPx = (): PixelSize => {
    const state = manager.getState();
    return frame.members.reduce<PixelSize>(
      (maximum, member) => {
        const app = state.windows[member]?.app;
        const minimum = app === undefined ? OS_WINDOW_CONSERVATIVE_MINIMUM : getOsAppMinimum(app);
        return {
          width: Math.max(maximum.width, minimum.width),
          height: Math.max(maximum.height, minimum.height),
        };
      },
      { width: 0, height: 0 }
    );
  };

  // Growth stops at the nearest island or the layout-area edge; shrink keeps
  // the app minimum. Passed to rnd so the live drag can't overshoot the commit.
  const handleResizeStart: RndResizeStartCallback = (_event, direction, _element) => {
    if (frame.kind === "floating" || frame.zone === null) return;
    const area = layoutAreaRect();
    if (area === null) return;
    const zone = frame.zone;
    const limits = tiledZoneLimits(zone, tiledObstacles());
    const moved = directionEdges(direction as TiledResizeDirection);
    const growW = moved.left
      ? zone.x - limits.left
      : moved.right
        ? limits.right - (zone.x + zone.w)
        : 0;
    const growH = moved.top
      ? zone.y - limits.top
      : moved.bottom
        ? limits.bottom - (zone.y + zone.h)
        : 0;
    deps.setResizeMax({
      width: frame.rect.w + Math.max(0, growW) * area.w,
      height: frame.rect.h + Math.max(0, growH) * area.h,
    });
  };

  const commitTiledResize = (
    direction: TiledResizeDirection,
    delta: { width: number; height: number },
    resized: OsRect,
    gestureAttempt: number
  ) => {
    const zone = frame.zone;
    const area = layoutAreaRect();
    if (zone === null || area === null) {
      deps.scheduleAuthoritativeRollback(gestureAttempt);
      return;
    }
    const obstacles = tiledObstacles();
    const minimum = frameMinimumPx();
    const next = resizedTiledZone({
      zone,
      direction,
      deltaW: delta.width / area.w,
      deltaH: delta.height / area.h,
      limits: tiledZoneLimits(zone, obstacles),
      minSize: { w: minimum.width / area.w, h: minimum.height / area.h },
    });
    if (next === null || zoneOverlapsAny(next, obstacles)) {
      deps.scheduleAuthoritativeRollback(gestureAttempt);
      return;
    }
    const outcome = manager.resizeWindowFrame(activeId, next);
    deps.trackGestureOutcome(resized, outcome, gestureAttempt);
  };

  const handleResizeStop: RndResizeCallback = (_event, direction, element, delta, position) => {
    deps.setResizeMax(null);
    const gestureAttempt = deps.nextGestureAttempt();
    const resized = {
      x: position.x,
      y: position.y,
      w: element.offsetWidth,
      h: element.offsetHeight,
    };
    if (frame.kind !== "floating") {
      commitTiledResize(direction as TiledResizeDirection, delta, resized, gestureAttempt);
      return;
    }
    const outcome = manager.commitFloatingRect(activeId, resized, undefined, deps.movesAsUnit);
    deps.trackGestureOutcome(resized, outcome, gestureAttempt);
  };

  return { resizeMax: deps.resizeMax, handleResizeStart, handleResizeStop };
}

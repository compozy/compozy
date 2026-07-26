import type {
  LayoutAxis,
  LayoutNodeId,
  PixelPoint,
  PixelRect,
  WindowId,
  WindowManagerEdgeCenterBinding,
} from "./window-manager-types";
import {
  createTileSnapTarget,
  type SnapEdge,
  type SnapSide,
  type TileSnapTarget,
} from "./tile-snap-target";
import { windowManagerLayoutArea } from "./window-manager-layout-area";

export {
  createTileSnapTarget,
  progressiveSnapRatio,
  type SnapCorner,
  type SnapEdge,
  type SnapSide,
  type TileSnapTarget,
} from "./tile-snap-target";
export type InsertionRelation = "before" | "after";
const OCCUPIED_SNAP_CENTER_INSET_RATIO = 0.25;
export interface SnapTargetConfig {
  edgeBand: number;
  cornerReach: number;
  exitSlack: number;
  innerGap: number;
  outerGaps: { top: number; right: number; bottom: number; left: number };
  repeatRatios: readonly number[];
  topCenter: WindowManagerEdgeCenterBinding;
  bottomCenter: WindowManagerEdgeCenterBinding;
}

export const DEFAULT_SNAP_TARGET_CONFIG: SnapTargetConfig = {
  edgeBand: 32,
  cornerReach: 150,
  exitSlack: 16,
  innerGap: 8,
  outerGaps: { top: 0, right: 0, bottom: 0, left: 0 },
  repeatRatios: [0.5, 0.666667, 0.333333],
  topCenter: "zoom",
  bottomCenter: "reserved",
};

export interface ZoomSnapTarget {
  kind: "zoom";
  id: "zoom";
  rect: PixelRect;
}

export interface InsertSnapTarget {
  kind: "insert";
  id: string;
  targetWindowId: WindowId;
  targetNodeId: LayoutNodeId;
  relation: InsertionRelation;
  axis: LayoutAxis;
  rect: PixelRect;
}

export interface SplitSnapTarget {
  kind: "split";
  id: string;
  targetWindowId: WindowId;
  targetNodeId: LayoutNodeId;
  side: SnapSide;
  axis: LayoutAxis;
  rect: PixelRect;
}

export interface StackSnapTarget {
  kind: "stack";
  id: string;
  targetWindowId: WindowId;
  targetNodeId: LayoutNodeId;
  rect: PixelRect;
}

export interface SwapSnapTarget {
  kind: "swap";
  id: string;
  targetWindowId: WindowId;
  targetNodeId: LayoutNodeId;
  rect: PixelRect;
}

export type SnapTarget =
  | TileSnapTarget
  | ZoomSnapTarget
  | InsertSnapTarget
  | SplitSnapTarget
  | StackSnapTarget
  | SwapSnapTarget;

export interface OccupiedSnapCandidate {
  windowId: WindowId;
  nodeId: LayoutNodeId;
  rect: PixelRect;
  z: number;
  parentAxis: LayoutAxis | null;
}

export interface ResolveSnapTargetInput {
  point: PixelPoint;
  workArea: PixelRect;
  cycleStep?: number;
  candidates?: readonly OccupiedSnapCandidate[];
  currentTarget?: SnapTarget | null;
  config: SnapTargetConfig;
  /** Occupied drops resolve to a whole-window swap while the configured swap modifier is held. */
  swapModifierActive?: boolean;
}

type EdgeResolution =
  | { state: "target"; target: SnapTarget }
  | { state: "blocked" }
  | { state: "no-special" };

function finite(value: number, fallback = 0): number {
  return Number.isFinite(value) ? value : fallback;
}

function positive(value: number): number {
  return Math.max(0, finite(value));
}

function normalizedRect(rect: PixelRect): PixelRect {
  return {
    x: finite(rect.x),
    y: finite(rect.y),
    w: positive(rect.w),
    h: positive(rect.h),
  };
}

function containsPoint(rect: PixelRect, point: PixelPoint): boolean {
  return (
    point.x >= rect.x &&
    point.x <= rect.x + rect.w &&
    point.y >= rect.y &&
    point.y <= rect.y + rect.h
  );
}

function clampPointToRect(point: PixelPoint, rect: PixelRect): PixelPoint {
  return {
    x: Math.min(rect.x + rect.w, Math.max(rect.x, finite(point.x))),
    y: Math.min(rect.y + rect.h, Math.max(rect.y, finite(point.y))),
  };
}

function intersectRect(rect: PixelRect, boundary: PixelRect): PixelRect | null {
  const left = Math.max(rect.x, boundary.x);
  const top = Math.max(rect.y, boundary.y);
  const right = Math.min(rect.x + rect.w, boundary.x + boundary.w);
  const bottom = Math.min(rect.y + rect.h, boundary.y + boundary.h);
  if (right <= left || bottom <= top) return null;
  return { x: left, y: top, w: right - left, h: bottom - top };
}

function configFrom(input: ResolveSnapTargetInput): SnapTargetConfig {
  return {
    edgeBand: positive(input.config.edgeBand),
    cornerReach: positive(input.config.cornerReach),
    exitSlack: positive(input.config.exitSlack),
    innerGap: positive(input.config.innerGap),
    outerGaps: {
      top: positive(input.config.outerGaps.top),
      right: positive(input.config.outerGaps.right),
      bottom: positive(input.config.outerGaps.bottom),
      left: positive(input.config.outerGaps.left),
    },
    repeatRatios: input.config.repeatRatios.map(ratio =>
      Math.max(0.1, Math.min(0.9, finite(ratio, 0.1)))
    ),
    topCenter: input.config.topCenter,
    bottomCenter: input.config.bottomCenter,
  };
}

function edgeTarget(
  point: PixelPoint,
  area: PixelRect,
  tileArea: PixelRect,
  band: number,
  reach: number,
  cycleStep: number,
  repeatRatios: readonly number[],
  topCenter: WindowManagerEdgeCenterBinding,
  bottomCenter: WindowManagerEdgeCenterBinding
): EdgeResolution {
  const distanceLeft = point.x - area.x;
  const distanceRight = area.x + area.w - point.x;
  const distanceTop = point.y - area.y;
  const distanceBottom = area.y + area.h - point.y;
  const nearLeft = distanceLeft <= band;
  const nearRight = distanceRight <= band;
  const nearTop = distanceTop <= band;
  const nearBottom = distanceBottom <= band;
  const withinTopReach = distanceTop <= reach;
  const withinBottomReach = distanceBottom <= reach;
  const withinLeftReach = distanceLeft <= reach;
  const withinRightReach = distanceRight <= reach;

  if (nearLeft || nearRight) {
    const edge: SnapEdge =
      nearLeft && nearRight
        ? distanceLeft <= distanceRight
          ? "left"
          : "right"
        : nearLeft
          ? "left"
          : "right";
    if (withinTopReach) {
      return {
        state: "target",
        target: createTileSnapTarget(tileArea, `top-${edge}`, repeatRatios, cycleStep),
      };
    }
    if (withinBottomReach) {
      return {
        state: "target",
        target: createTileSnapTarget(tileArea, `bottom-${edge}`, repeatRatios, cycleStep),
      };
    }
    return {
      state: "target",
      target: createTileSnapTarget(tileArea, edge, repeatRatios, cycleStep),
    };
  }
  if (nearTop) {
    if (withinLeftReach && withinRightReach) {
      return {
        state: "target",
        target: createTileSnapTarget(
          tileArea,
          distanceLeft <= distanceRight ? "top-left" : "top-right",
          repeatRatios,
          cycleStep
        ),
      };
    }
    if (withinLeftReach) {
      return {
        state: "target",
        target: createTileSnapTarget(tileArea, "top-left", repeatRatios, cycleStep),
      };
    }
    if (withinRightReach) {
      return {
        state: "target",
        target: createTileSnapTarget(tileArea, "top-right", repeatRatios, cycleStep),
      };
    }
    return edgeCenterTarget(topCenter, tileArea);
  }
  if (nearBottom) {
    if (withinLeftReach && withinRightReach) {
      return {
        state: "target",
        target: createTileSnapTarget(
          tileArea,
          distanceLeft <= distanceRight ? "bottom-left" : "bottom-right",
          repeatRatios,
          cycleStep
        ),
      };
    }
    if (withinLeftReach) {
      return {
        state: "target",
        target: createTileSnapTarget(tileArea, "bottom-left", repeatRatios, cycleStep),
      };
    }
    if (withinRightReach) {
      return {
        state: "target",
        target: createTileSnapTarget(tileArea, "bottom-right", repeatRatios, cycleStep),
      };
    }
    return edgeCenterTarget(bottomCenter, tileArea);
  }
  return { state: "no-special" };
}

function edgeCenterTarget(
  binding: WindowManagerEdgeCenterBinding,
  area: PixelRect
): EdgeResolution {
  switch (binding) {
    case "zoom":
      return { state: "target", target: { kind: "zoom", id: "zoom", rect: area } };
    case "none":
      return { state: "no-special" };
    case "reserved":
      return { state: "blocked" };
  }
}

function sideFromCandidate(point: PixelPoint, rect: PixelRect): SnapSide | null {
  const x = (point.x - rect.x) / rect.w;
  const y = (point.y - rect.y) / rect.h;
  const inset = OCCUPIED_SNAP_CENTER_INSET_RATIO;
  if (x >= inset && x <= 1 - inset && y >= inset && y <= 1 - inset) return null;
  const distances: Array<{ side: SnapSide; value: number }> = [
    { side: "left", value: x },
    { side: "right", value: 1 - x },
    { side: "top", value: y },
    { side: "bottom", value: 1 - y },
  ];
  distances.sort((a, b) => a.value - b.value || a.side.localeCompare(b.side));
  return distances[0]?.side ?? null;
}

function claimedSide(rect: PixelRect, side: SnapSide, gap: number): PixelRect {
  if (side === "left" || side === "right") {
    const available = Math.max(0, rect.w - gap);
    const leadingWidth = Math.floor(available / 2);
    const trailingWidth = available - leadingWidth;
    return side === "left"
      ? { x: rect.x, y: rect.y, w: leadingWidth, h: rect.h }
      : { x: rect.x + rect.w - trailingWidth, y: rect.y, w: trailingWidth, h: rect.h };
  }
  const available = Math.max(0, rect.h - gap);
  const leadingHeight = Math.floor(available / 2);
  const trailingHeight = available - leadingHeight;
  return side === "top"
    ? { x: rect.x, y: rect.y, w: rect.w, h: leadingHeight }
    : { x: rect.x, y: rect.y + rect.h - trailingHeight, w: rect.w, h: trailingHeight };
}

function occupiedTarget(
  point: PixelPoint,
  area: PixelRect,
  candidates: readonly OccupiedSnapCandidate[],
  gap: number,
  swapModifierActive: boolean
): SnapTarget | null {
  const ordered = [...candidates].sort((a, b) => b.z - a.z || a.windowId.localeCompare(b.windowId));
  for (const candidate of ordered) {
    const candidateRect = intersectRect(normalizedRect(candidate.rect), area);
    if (candidateRect === null || !containsPoint(candidateRect, point)) continue;
    if (swapModifierActive) {
      return {
        kind: "swap",
        id: `swap:${candidate.nodeId}`,
        targetWindowId: candidate.windowId,
        targetNodeId: candidate.nodeId,
        rect: candidateRect,
      };
    }
    const side = sideFromCandidate(point, candidateRect);
    if (side === null) {
      return {
        kind: "stack",
        id: `stack:${candidate.nodeId}`,
        targetWindowId: candidate.windowId,
        targetNodeId: candidate.nodeId,
        rect: candidateRect,
      };
    }
    const axis: LayoutAxis = side === "left" || side === "right" ? "horizontal" : "vertical";
    const rect = claimedSide(candidateRect, side, gap);
    if (candidate.parentAxis === axis) {
      const relation: InsertionRelation = side === "left" || side === "top" ? "before" : "after";
      return {
        kind: "insert",
        id: `insert:${candidate.nodeId}:${relation}`,
        targetWindowId: candidate.windowId,
        targetNodeId: candidate.nodeId,
        relation,
        axis,
        rect,
      };
    }
    return {
      kind: "split",
      id: `split:${candidate.nodeId}:${side}`,
      targetWindowId: candidate.windowId,
      targetNodeId: candidate.nodeId,
      side,
      axis,
      rect,
    };
  }
  return null;
}

function heldTarget(
  input: ResolveSnapTargetInput,
  point: PixelPoint,
  area: PixelRect,
  tileArea: PixelRect,
  config: SnapTargetConfig
): SnapTarget | null {
  const current = input.currentTarget;
  if (current === undefined || current === null) return null;
  const expanded = edgeTarget(
    point,
    area,
    tileArea,
    config.edgeBand + config.exitSlack,
    config.cornerReach + config.exitSlack,
    input.cycleStep ?? 0,
    config.repeatRatios,
    config.topCenter,
    config.bottomCenter
  );
  if (
    current.kind === "tile" &&
    expanded.state === "target" &&
    expanded.target.kind === "tile" &&
    current.edge === expanded.target.edge
  ) {
    return current;
  }
  return current.kind === "zoom" && expanded.state === "target" && expanded.target.kind === "zoom"
    ? current
    : null;
}

/**
 * Resolves one preview target. Overshoot clamps to the work area, so pushing
 * the pointer past an edge keeps that edge armed instead of dropping the
 * target; reserved edge-center bands still block.
 */
export function resolveSnapTarget(input: ResolveSnapTargetInput): SnapTarget | null {
  const area = normalizedRect(input.workArea);
  if (area.w <= 0 || area.h <= 0) return null;
  const point = clampPointToRect(input.point, area);
  const config = configFrom(input);
  const tileArea = windowManagerLayoutArea(area, config.outerGaps);
  const cycleStep = input.cycleStep ?? 0;
  const edge = edgeTarget(
    point,
    area,
    tileArea,
    config.edgeBand,
    config.cornerReach,
    cycleStep,
    config.repeatRatios,
    config.topCenter,
    config.bottomCenter
  );
  if (edge.state === "target") return edge.target;
  if (edge.state === "blocked") return null;
  const occupied = occupiedTarget(
    point,
    area,
    input.candidates ?? [],
    config.innerGap,
    input.swapModifierActive === true
  );
  if (occupied !== null) return occupied;
  return heldTarget(input, point, area, tileArea, config);
}

export function snapTargetIsContained(target: SnapTarget, workArea: PixelRect): boolean {
  const area = normalizedRect(workArea);
  return (
    target.rect.x >= area.x &&
    target.rect.y >= area.y &&
    target.rect.x + target.rect.w <= area.x + area.w &&
    target.rect.y + target.rect.h <= area.y + area.h
  );
}

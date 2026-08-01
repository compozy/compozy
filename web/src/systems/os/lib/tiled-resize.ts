import type { NormalizedRect } from "./window-manager-types";

/** Abutment tolerance for island edges sharing one boundary line. */
const EDGE_EPSILON = 0.0001;
/** Daemon floor for island frame extents (`minGroupFrameExtent`). */
const MIN_ZONE_EXTENT = 0.01;

export type TiledResizeDirection =
  | "top"
  | "right"
  | "bottom"
  | "left"
  | "topRight"
  | "bottomRight"
  | "bottomLeft"
  | "topLeft";

export interface TiledResizableEdges {
  left: boolean;
  right: boolean;
  top: boolean;
  bottom: boolean;
}

export interface TiledZoneLimits {
  left: number;
  right: number;
  top: number;
  bottom: number;
}

function spansOverlap(aStart: number, aEnd: number, bStart: number, bEnd: number): boolean {
  return aStart < bEnd - EDGE_EPSILON && bStart < aEnd - EDGE_EPSILON;
}

function abutsLine(edge: number, line: number): boolean {
  return Math.abs(edge - line) <= EDGE_EPSILON;
}

/**
 * A side is free exactly when no other tiled unit abuts it: shared boundaries
 * resize through their seam, free edges resize this unit alone.
 */
export function frameResizableEdges(
  zone: NormalizedRect,
  obstacles: readonly NormalizedRect[]
): TiledResizableEdges {
  const horizontalNeighbours = obstacles.filter(other =>
    spansOverlap(other.y, other.y + other.h, zone.y, zone.y + zone.h)
  );
  const verticalNeighbours = obstacles.filter(other =>
    spansOverlap(other.x, other.x + other.w, zone.x, zone.x + zone.w)
  );
  return {
    left: !horizontalNeighbours.some(other => abutsLine(other.x + other.w, zone.x)),
    right: !horizontalNeighbours.some(other => abutsLine(other.x, zone.x + zone.w)),
    top: !verticalNeighbours.some(other => abutsLine(other.y + other.h, zone.y)),
    bottom: !verticalNeighbours.some(other => abutsLine(other.y, zone.y + zone.h)),
  };
}

/** Outward growth bound per side: the nearest obstacle edge or the area boundary. */
export function tiledZoneLimits(
  zone: NormalizedRect,
  obstacles: readonly NormalizedRect[]
): TiledZoneLimits {
  const limits: TiledZoneLimits = { left: 0, right: 1, top: 0, bottom: 1 };
  for (const other of obstacles) {
    if (spansOverlap(other.y, other.y + other.h, zone.y, zone.y + zone.h)) {
      const otherRight = other.x + other.w;
      if (otherRight <= zone.x + EDGE_EPSILON) limits.left = Math.max(limits.left, otherRight);
      if (other.x >= zone.x + zone.w - EDGE_EPSILON) limits.right = Math.min(limits.right, other.x);
    }
    if (spansOverlap(other.x, other.x + other.w, zone.x, zone.x + zone.w)) {
      const otherBottom = other.y + other.h;
      if (otherBottom <= zone.y + EDGE_EPSILON) limits.top = Math.max(limits.top, otherBottom);
      if (other.y >= zone.y + zone.h - EDGE_EPSILON) {
        limits.bottom = Math.min(limits.bottom, other.y);
      }
    }
  }
  return limits;
}

export function directionEdges(direction: TiledResizeDirection): TiledResizableEdges {
  return {
    left: direction === "left" || direction === "topLeft" || direction === "bottomLeft",
    right: direction === "right" || direction === "topRight" || direction === "bottomRight",
    top: direction === "top" || direction === "topLeft" || direction === "topRight",
    bottom: direction === "bottom" || direction === "bottomLeft" || direction === "bottomRight",
  };
}

export interface ResizedTiledZoneInput {
  zone: NormalizedRect;
  direction: TiledResizeDirection;
  /** Normalized size deltas (resized minus original). */
  deltaW: number;
  deltaH: number;
  limits: TiledZoneLimits;
  /** Normalized minimum extents for this unit. */
  minSize: { w: number; h: number };
}

/**
 * Applies an edge/corner drag to a tiled unit's zone, clamped to the nearest
 * obstacles and the unit's minimum extents. Returns null when nothing moved.
 */
export function resizedTiledZone(input: ResizedTiledZoneInput): NormalizedRect | null {
  const { zone, limits } = input;
  const moved = directionEdges(input.direction);
  const minW = Math.max(MIN_ZONE_EXTENT, input.minSize.w);
  const minH = Math.max(MIN_ZONE_EXTENT, input.minSize.h);
  let left = zone.x;
  let right = zone.x + zone.w;
  let top = zone.y;
  let bottom = zone.y + zone.h;
  if (moved.left) {
    left = Math.min(right - minW, Math.max(limits.left, left - input.deltaW));
  } else if (moved.right) {
    right = Math.max(left + minW, Math.min(limits.right, right + input.deltaW));
  }
  if (moved.top) {
    top = Math.min(bottom - minH, Math.max(limits.top, top - input.deltaH));
  } else if (moved.bottom) {
    bottom = Math.max(top + minH, Math.min(limits.bottom, bottom + input.deltaH));
  }
  const next: NormalizedRect = { x: left, y: top, w: right - left, h: bottom - top };
  const unchanged =
    Math.abs(next.x - zone.x) <= EDGE_EPSILON &&
    Math.abs(next.y - zone.y) <= EDGE_EPSILON &&
    Math.abs(next.w - zone.w) <= EDGE_EPSILON &&
    Math.abs(next.h - zone.h) <= EDGE_EPSILON;
  if (unchanged || next.w < MIN_ZONE_EXTENT || next.h < MIN_ZONE_EXTENT) return null;
  return next;
}

/** Final defence for diagonal growth past per-side clamps. */
export function zoneOverlapsAny(
  zone: NormalizedRect,
  obstacles: readonly NormalizedRect[]
): boolean {
  return obstacles.some(
    other =>
      spansOverlap(zone.x, zone.x + zone.w, other.x, other.x + other.w) &&
      spansOverlap(zone.y, zone.y + zone.h, other.y, other.y + other.h)
  );
}

/**
 * Frozen v5 projection for the layout editor; S7 replaces it.
 *
 * Minimums, layout-area insets and the seam preview the editor's v5 canvas shares with its
 * projector, copied unchanged from the retired `systems/os` geometry.
 */
import type {
  LayoutDesktop,
  LayoutNode,
  PixelRect,
  PixelSize,
  ProjectedSeam,
  ProjectionGaps,
  WindowMinimums,
} from "./legacy-layout-types";

const EMPTY_MINIMUM: PixelSize = { width: 0, height: 0 };

function nonNegativeInteger(value: number): number {
  return Number.isFinite(value) ? Math.max(0, Math.round(value)) : 0;
}

export function minimumForWindow(windowId: string, minimums: WindowMinimums): PixelSize {
  const minimum = minimums[windowId];
  if (minimum === undefined) return EMPTY_MINIMUM;
  return {
    width: nonNegativeInteger(minimum.width),
    height: nonNegativeInteger(minimum.height),
  };
}

export function minimumForNode(
  node: LayoutNode,
  innerGap: number,
  minimums: WindowMinimums
): PixelSize {
  if (node.kind === "leaf") return minimumForWindow(node.windowId, minimums);
  if (node.kind === "stack") {
    return node.windowIds.reduce<PixelSize>((maximum, windowId) => {
      const minimum = minimumForWindow(windowId, minimums);
      return {
        width: Math.max(maximum.width, minimum.width),
        height: Math.max(maximum.height, minimum.height),
      };
    }, EMPTY_MINIMUM);
  }
  const children = node.children.map(child => minimumForNode(child, innerGap, minimums));
  const gapTotal = innerGap * Math.max(0, children.length - 1);
  if (node.axis === "horizontal") {
    return {
      width: children.reduce((sum, child) => sum + child.width, gapTotal),
      height: children.reduce((maximum, child) => Math.max(maximum, child.height), 0),
    };
  }
  return {
    width: children.reduce((maximum, child) => Math.max(maximum, child.width), 0),
    height: children.reduce((sum, child) => sum + child.height, gapTotal),
  };
}

/** Every window under a node, in tree order. */
export function descendantstrings(node: LayoutNode): string[] {
  if (node.kind === "leaf") return [node.windowId];
  if (node.kind === "stack") return [...node.windowIds];
  return node.children.flatMap(descendantstrings);
}


type OuterGaps = Omit<ProjectionGaps, "inner">;


function pixelRect(rect: PixelRect): PixelRect {
  return {
    x: Number.isFinite(rect.x) ? Math.round(rect.x) : 0,
    y: Number.isFinite(rect.y) ? Math.round(rect.y) : 0,
    w: nonNegativeInteger(rect.w),
    h: nonNegativeInteger(rect.h),
  };
}

/** The one pixel coordinate box used for tiled preview, normalization, and projection. */
export function windowManagerLayoutArea(area: PixelRect, gaps: OuterGaps): PixelRect {
  const rect = pixelRect(area);
  const left = Math.min(nonNegativeInteger(gaps.left), rect.w);
  const right = Math.min(nonNegativeInteger(gaps.right), rect.w - left);
  const top = Math.min(nonNegativeInteger(gaps.top), rect.h);
  const bottom = Math.min(nonNegativeInteger(gaps.bottom), rect.h - top);
  return {
    x: rect.x + left,
    y: rect.y + top,
    w: rect.w - left - right,
    h: rect.h - top - bottom,
  };
}

/**
 * Splits the inner gap across every interior edge of a tile that owns a whole
 * area zone. Sibling splits already reserve a full gap between children, but two
 * independent group frames only meet on a shared fraction — each side gives back
 * half so the boundary reads as exactly one gap, while edges flush with the
 * layout area stay flush against the outer gaps.
 */
export function windowManagerTileInset(
  rect: PixelRect,
  area: PixelRect,
  innerGap: number
): PixelRect {
  const gap = nonNegativeInteger(innerGap);
  if (gap === 0) return rect;
  const leading = Math.ceil(gap / 2);
  const trailing = gap - leading;
  const left = rect.x > area.x ? leading : 0;
  const top = rect.y > area.y ? leading : 0;
  const right = rect.x + rect.w < area.x + area.w ? trailing : 0;
  const bottom = rect.y + rect.h < area.y + area.h ? trailing : 0;
  return {
    x: rect.x + left,
    y: rect.y + top,
    w: Math.max(0, rect.w - left - right),
    h: Math.max(0, rect.h - top - bottom),
  };
}

/** Daemon floor for adjacent split weights (`reducer_layout_resize.go`). */
const MIN_SPLIT_WEIGHT = 0.01;

/**
 * Converts a seam drag in pixels to the weight-space delta `layout.resize`
 * commits, clamped by the projection's pixel minimums and the daemon's
 * adjacent-weight floor. Mirroring the reducer keeps the live preview and the
 * committed layout identical.
 */
export function seamWeightDelta(seam: ProjectedSeam, deltaPx: number): number {
  if (!Number.isFinite(deltaPx) || seam.axisSpan <= 0) return 0;
  const clampedPx = Math.min(
    seam.maxValue - seam.value,
    Math.max(seam.minValue - seam.value, deltaPx)
  );
  const delta = clampedPx / seam.axisSpan;
  const maxDelta = Math.max(seam.trailingWeight - MIN_SPLIT_WEIGHT, 0);
  const minDelta = Math.min(MIN_SPLIT_WEIGHT - seam.leadingWeight, 0);
  return Math.min(maxDelta, Math.max(minDelta, delta));
}

/** Returns the desktop with the previewed seam delta applied to its split. */
export function applySeamPreviewToDesktop(
  desktop: LayoutDesktop,
  seam: ProjectedSeam,
  deltaPx: number
): LayoutDesktop {
  const delta = seamWeightDelta(seam, deltaPx);
  if (delta === 0) return desktop;
  let changed = false;
  const groups = desktop.groups.map(group => {
    const root = adjustSplitWeights(group.root, seam.splitId, seam.boundaryIndex, delta);
    if (root === group.root) return group;
    changed = true;
    return { ...group, root };
  });
  return changed ? { ...desktop, groups } : desktop;
}

function adjustSplitWeights(
  node: LayoutNode,
  splitId: string,
  boundaryIndex: number,
  delta: number
): LayoutNode {
  if (node.kind !== "split") return node;
  if (node.id === splitId) {
    const leading = node.weights[boundaryIndex];
    const trailing = node.weights[boundaryIndex + 1];
    if (leading === undefined || trailing === undefined) return node;
    const nextLeading = leading + delta;
    const nextTrailing = trailing - delta;
    if (nextLeading === leading && nextTrailing === trailing) return node;
    const weights = [...node.weights];
    weights[boundaryIndex] = nextLeading;
    weights[boundaryIndex + 1] = nextTrailing;
    return { ...node, weights };
  }
  let changed = false;
  const children = node.children.map(child => {
    const adjusted = adjustSplitWeights(child, splitId, boundaryIndex, delta);
    if (adjusted !== child) changed = true;
    return adjusted;
  });
  return changed ? { ...node, children } : node;
}

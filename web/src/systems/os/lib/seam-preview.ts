import type { LayoutNode, LayoutNodeId, LayoutTab, ProjectedSeam } from "./window-manager-types";

/** Daemon floor for adjacent split weights (`reducer_layout_resize.go`). */
const MIN_SPLIT_WEIGHT = 0.01;

/** A seam drag in flight: the client previews it before `layout.resize` commits. */
export interface SeamPreview {
  readonly splitId: string;
  readonly boundaryIndex: number;
  readonly deltaPx: number;
}

/** A seam drag in pixels held to the bounds the projection gives the seam. */
export function clampSeamShift(seam: ProjectedSeam, deltaPx: number): number {
  if (!Number.isFinite(deltaPx)) return 0;
  return Math.min(seam.maxValue - seam.value, Math.max(seam.minValue - seam.value, deltaPx));
}

/**
 * Converts a seam drag in pixels to the weight-space delta `layout.resize`
 * commits, clamped by the projection's pixel minimums and the daemon's
 * adjacent-weight floor. Mirroring the reducer keeps the live preview and the
 * committed layout identical.
 */
export function seamWeightDelta(seam: ProjectedSeam, deltaPx: number): number {
  if (!Number.isFinite(deltaPx) || seam.axisSpan <= 0) return 0;
  const delta = clampSeamShift(seam, deltaPx) / seam.axisSpan;
  const maxDelta = Math.max(seam.trailingWeight - MIN_SPLIT_WEIGHT, 0);
  const minDelta = Math.min(MIN_SPLIT_WEIGHT - seam.leadingWeight, 0);
  return Math.min(maxDelta, Math.max(minDelta, delta));
}

/** Returns the tab with the previewed seam delta applied to its split. */
export function applySeamPreviewToTab(
  tab: LayoutTab,
  seam: ProjectedSeam,
  deltaPx: number
): LayoutTab {
  const delta = seamWeightDelta(seam, deltaPx);
  if (delta === 0) return tab;
  const root = adjustSplitWeights(tab.root, seam.splitId, seam.boundaryIndex, delta);
  return root === tab.root ? tab : { ...tab, root };
}

function adjustSplitWeights(
  node: LayoutNode,
  splitId: LayoutNodeId,
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

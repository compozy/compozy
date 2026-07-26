import type {
  LayoutDesktop,
  LayoutNode,
  LayoutNodeId,
  ProjectedSeam,
} from "./window-manager-types";

/** Daemon floor for adjacent split weights (`reducer_layout_resize.go`). */
const MIN_SPLIT_WEIGHT = 0.01;

export interface SeamPreview {
  readonly splitId: string;
  readonly boundaryIndex: number;
  readonly deltaPx: number;
}

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

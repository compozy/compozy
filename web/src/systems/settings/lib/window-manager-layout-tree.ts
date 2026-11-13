import type {
  WindowManagerLayoutAxis,
  WindowManagerLayoutEdge,
  WindowManagerLayoutLeaf,
  WindowManagerLayoutNode,
  WindowManagerLayoutSplit,
} from "./window-manager-layout-types";

/** The smallest share a member keeps when a seam is dragged or a weight is set. */
export const LAYOUT_MIN_WEIGHT = 0.05;

export type LayoutNodeIdFactory = (kind: "tab" | "leaf" | "split") => string;

export function layoutNodeWindowIds(node: WindowManagerLayoutNode): string[] {
  if (node.kind === "leaf") return [node.windowId];
  return node.children.flatMap(layoutNodeWindowIds);
}

export function layoutNodeIds(node: WindowManagerLayoutNode): string[] {
  if (node.kind === "leaf") return [node.id];
  return [node.id, ...node.children.flatMap(layoutNodeIds)];
}

export function findLayoutNode(
  node: WindowManagerLayoutNode,
  nodeId: string
): WindowManagerLayoutNode | null {
  if (node.id === nodeId) return node;
  if (node.kind === "leaf") return null;
  for (const child of node.children) {
    const hit = findLayoutNode(child, nodeId);
    if (hit) return hit;
  }
  return null;
}

export function findLayoutLeaf(
  node: WindowManagerLayoutNode,
  windowId: string
): WindowManagerLayoutLeaf | null {
  if (node.kind === "leaf") return node.windowId === windowId ? node : null;
  for (const child of node.children) {
    const hit = findLayoutLeaf(child, windowId);
    if (hit) return hit;
  }
  return null;
}

/** The split that directly holds `nodeId`, or null for a tab's root. */
export function findLayoutParentSplit(
  node: WindowManagerLayoutNode,
  nodeId: string
): WindowManagerLayoutSplit | null {
  if (node.kind === "leaf") return null;
  if (node.children.some(child => child.id === nodeId)) return node;
  for (const child of node.children) {
    const hit = findLayoutParentSplit(child, nodeId);
    if (hit) return hit;
  }
  return null;
}

/** Immutable node swap: untouched branches stay shared. */
export function replaceLayoutNode(
  node: WindowManagerLayoutNode,
  nodeId: string,
  next: WindowManagerLayoutNode
): WindowManagerLayoutNode {
  if (node.id === nodeId) return next;
  if (node.kind === "leaf") return node;
  let changed = false;
  const children = node.children.map(child => {
    const replaced = replaceLayoutNode(child, nodeId, next);
    if (replaced !== child) changed = true;
    return replaced;
  });
  return changed ? { ...node, children } : node;
}

/**
 * Rescales a weight vector so it sums to exactly 1 within the daemon's 1e-6
 * tolerance (`validate.go`). The last member absorbs the rounding remainder.
 */
export function normalizeWeights(weights: readonly number[]): number[] {
  const finite = weights.map(weight => (Number.isFinite(weight) && weight > 0 ? weight : 0));
  const total = finite.reduce((sum, weight) => sum + weight, 0);
  if (finite.length === 0) return [];
  if (total <= 0) return evenWeights(finite.length);
  const scaled = finite.map(weight => weight / total);
  const head = scaled.slice(0, -1);
  return [...head, 1 - head.reduce((sum, weight) => sum + weight, 0)];
}

export function evenWeights(count: number): number[] {
  if (count < 1) return [];
  return normalizeWeights(Array.from({ length: count }, () => 1));
}

/**
 * Restores the daemon's tree invariants after any edit: a split keeps at least
 * two members (one member replaces the split), weights stay positive and sum to
 * 1, and a split never holds a direct child on its own axis — that child's
 * members are lifted into the parent with their share of its weight.
 */
export function normalizeLayoutNode(node: WindowManagerLayoutNode): WindowManagerLayoutNode {
  if (node.kind === "leaf") return node;
  const children: WindowManagerLayoutNode[] = [];
  const weights: number[] = [];
  node.children.forEach((child, index) => {
    const weight = node.weights[index] ?? 0;
    const normalized = normalizeLayoutNode(child);
    if (normalized.kind === "split" && normalized.axis === node.axis) {
      normalized.children.forEach((grandchild, position) => {
        children.push(grandchild);
        weights.push(weight * (normalized.weights[position] ?? 0));
      });
      return;
    }
    children.push(normalized);
    weights.push(weight);
  });
  if (children.length === 1 && children[0]) return children[0];
  const floored = weights.map(weight => Math.max(weight, Number.EPSILON));
  return { ...node, children, weights: normalizeWeights(floored) };
}

/**
 * Removes one window's leaf. A split left with one member collapses into it;
 * the removed member's share goes to the others in proportion. Returns null
 * when the tree held only that window.
 */
export function removeLayoutLeaf(
  node: WindowManagerLayoutNode,
  windowId: string
): WindowManagerLayoutNode | null {
  if (node.kind === "leaf") return node.windowId === windowId ? null : node;
  const children: WindowManagerLayoutNode[] = [];
  const weights: number[] = [];
  node.children.forEach((child, index) => {
    const next = removeLayoutLeaf(child, windowId);
    if (next === null) return;
    children.push(next);
    weights.push(node.weights[index] ?? 0);
  });
  if (children.length === 0) return null;
  return normalizeLayoutNode({ ...node, children, weights });
}

function edgeAxis(edge: WindowManagerLayoutEdge): WindowManagerLayoutAxis {
  return edge === "left" || edge === "right" ? "horizontal" : "vertical";
}

/**
 * Places `windowId` beside the leaf showing `targetWindowId`. The daemon's
 * `horizontal` axis lays members out side by side, so left and right split on
 * it and top and bottom on `vertical`. The target gives up half its share.
 */
export function insertBesideLeaf(
  node: WindowManagerLayoutNode,
  targetWindowId: string,
  windowId: string,
  edge: WindowManagerLayoutEdge,
  mintId: LayoutNodeIdFactory
): WindowManagerLayoutNode {
  const target = findLayoutLeaf(node, targetWindowId);
  if (target === null) return node;
  const leaf: WindowManagerLayoutLeaf = { id: mintId("leaf"), kind: "leaf", windowId };
  const leading = edge === "left" || edge === "top";
  const split: WindowManagerLayoutSplit = {
    id: mintId("split"),
    kind: "split",
    axis: edgeAxis(edge),
    children: leading ? [leaf, target] : [target, leaf],
    weights: [0.5, 0.5],
  };
  // A same-axis parent absorbs the new split in normalization, keeping the
  // target's share divided between the two.
  return normalizeLayoutNode(replaceLayoutNode(node, target.id, split));
}

/** Changes one split's axis; normalization merges it into a parent on the new axis. */
export function setLayoutSplitAxis(
  node: WindowManagerLayoutNode,
  splitId: string,
  axis: WindowManagerLayoutAxis
): WindowManagerLayoutNode {
  const split = findLayoutNode(node, splitId);
  if (split === null || split.kind !== "split" || split.axis === axis) return node;
  return normalizeLayoutNode(replaceLayoutNode(node, splitId, { ...split, axis }));
}

/** Sets one split's weights, keeping every member above the minimum share. */
export function setLayoutSplitWeights(
  node: WindowManagerLayoutNode,
  splitId: string,
  weights: readonly number[]
): WindowManagerLayoutNode {
  const split = findLayoutNode(node, splitId);
  if (split === null || split.kind !== "split" || weights.length !== split.children.length) {
    return node;
  }
  const clamped = normalizeWeights(weights).map(weight => Math.max(weight, LAYOUT_MIN_WEIGHT));
  return replaceLayoutNode(node, splitId, { ...split, weights: normalizeWeights(clamped) });
}

/**
 * Moves the boundary between members `index` and `index + 1` so the leading
 * one takes `leadingShare` of the pair's combined weight. Only that pair
 * changes, as with a seam drag in the shell.
 */
export function moveLayoutSeam(
  node: WindowManagerLayoutNode,
  splitId: string,
  index: number,
  leadingShare: number
): WindowManagerLayoutNode {
  const split = findLayoutNode(node, splitId);
  if (split === null || split.kind !== "split") return node;
  const leading = split.weights[index];
  const trailing = split.weights[index + 1];
  if (leading === undefined || trailing === undefined) return node;
  const pair = leading + trailing;
  const minimum = Math.min(LAYOUT_MIN_WEIGHT, pair / 2);
  const nextLeading = Math.min(Math.max(leadingShare * pair, minimum), pair - minimum);
  const weights = [...split.weights];
  weights[index] = nextLeading;
  weights[index + 1] = pair - nextLeading;
  return replaceLayoutNode(node, splitId, { ...split, weights: normalizeWeights(weights) });
}

/** Swaps which window two leaves show; the tree shape stays as it is. */
export function swapLayoutWindows(
  node: WindowManagerLayoutNode,
  first: string,
  second: string
): WindowManagerLayoutNode {
  if (node.kind === "leaf") {
    if (node.windowId === first) return { ...node, windowId: second };
    if (node.windowId === second) return { ...node, windowId: first };
    return node;
  }
  return { ...node, children: node.children.map(child => swapLayoutWindows(child, first, second)) };
}

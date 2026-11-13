import { projectTab } from "./tab-projection";
import type { ProjectionGaps, TabProjection, WindowMinimums } from "./tab-projection-types";
import type {
  LayoutAxis,
  LayoutNode,
  LayoutNodeId,
  LayoutTab,
  PixelPoint,
  PixelRect,
  WindowId,
} from "./window-manager-types";

/** Constants from the spec's assumptions: not configuration. */
export const ROOT_EDGE_BAND_PX = 24;
export const TAB_DRAG_EXIT_PX = 24;
export const HEAD_DWELL_MS = 400;

export type DropEdge = "left" | "right" | "top" | "bottom";

/** Where a dragged tab or window head would land over a tab's content. */
export type ContentDrop =
  | { kind: "window"; windowId: WindowId; nodeId: LayoutNodeId; edge: DropEdge }
  | { kind: "root"; edge: DropEdge };

const EDGE_ORDER: readonly DropEdge[] = ["left", "right", "top", "bottom"];

function closest(distances: Record<DropEdge, number>): DropEdge {
  // Ties resolve in a fixed order (left, right, top, bottom), so the center and the diagonals
  // always map to the same edge.
  return EDGE_ORDER.reduce((best, edge) => (distances[edge] < distances[best] ? edge : best));
}

function contains(rect: PixelRect, point: PixelPoint): boolean {
  return (
    point.x >= rect.x &&
    point.x <= rect.x + rect.w &&
    point.y >= rect.y &&
    point.y <= rect.y + rect.h
  );
}

/**
 * The edge of a window nearest a point, measured as a share of the window's width or height:
 * every position over a window resolves to exactly one edge, and there is no center zone.
 */
export function nearestEdge(rect: PixelRect, point: PixelPoint): DropEdge {
  const x = rect.w > 0 ? (point.x - rect.x) / rect.w : 0.5;
  const y = rect.h > 0 ? (point.y - rect.y) / rect.h : 0.5;
  return closest({ left: x, right: 1 - x, top: y, bottom: 1 - y });
}

/** The content area's outer edge within the band of a point, else null. */
export function rootEdge(
  area: PixelRect,
  point: PixelPoint,
  band: number = ROOT_EDGE_BAND_PX
): DropEdge | null {
  const distances = {
    left: point.x - area.x,
    right: area.x + area.w - point.x,
    top: point.y - area.y,
    bottom: area.y + area.h - point.y,
  };
  const edge = closest(distances);
  return distances[edge] <= band ? edge : null;
}

/**
 * The drop a point means over a projected tab: the tab's root edge inside the outer band, else
 * the nearest edge of the window on screen under the point. Outside the content, or in a gap
 * between windows, there is none.
 */
export function contentDropAt(projection: TabProjection, point: PixelPoint): ContentDrop | null {
  if (!contains(projection.workArea, point)) return null;
  const edge = rootEdge(projection.workArea, point);
  if (edge !== null) return { kind: "root", edge };
  const pane = projection.windows.find(entry => entry.shown && contains(entry.rect, point));
  if (pane === undefined) return null;
  return {
    kind: "window",
    windowId: pane.windowId,
    nodeId: pane.nodeId,
    edge: nearestEdge(pane.rect, point),
  };
}

const EDGE_AXIS: Record<DropEdge, { axis: LayoutAxis; before: boolean }> = {
  left: { axis: "horizontal", before: true },
  right: { axis: "horizontal", before: false },
  top: { axis: "vertical", before: true },
  bottom: { axis: "vertical", before: false },
};

const PREVIEW_SPLIT_ID = "drop-preview-split";

function equalWeights(count: number): number[] {
  return Array.from({ length: count }, () => 1 / count);
}

/** Removes a window; a split left with one child is replaced by that child. */
export function withoutWindow(node: LayoutNode, windowId: WindowId): LayoutNode | null {
  if (node.kind === "leaf") return node.windowId === windowId ? null : node;
  const children: LayoutNode[] = [];
  const weights: number[] = [];
  node.children.forEach((child, index) => {
    const next = withoutWindow(child, windowId);
    if (next === null) return;
    children.push(next);
    weights.push(node.weights[index] ?? 1);
  });
  if (children.length === 0) return null;
  if (children.length === 1) return children[0]!;
  return { ...node, children, weights };
}

function pair(incoming: LayoutNode, node: LayoutNode, edge: DropEdge): LayoutNode {
  const { axis, before } = EDGE_AXIS[edge];
  return {
    id: PREVIEW_SPLIT_ID,
    kind: "split",
    axis,
    children: before ? [incoming, node] : [node, incoming],
    weights: [0.5, 0.5],
  };
}

/**
 * Mirrors the daemon's `window.move` edge rule: beside a window inside a split on the same axis
 * the split is extended with equal shares; otherwise the window and the incoming node nest in a
 * new half-and-half split.
 */
export function insertBesideWindow(
  node: LayoutNode,
  targetWindowId: WindowId,
  incoming: LayoutNode,
  edge: DropEdge
): LayoutNode {
  if (node.kind === "leaf") {
    return node.windowId === targetWindowId ? pair(incoming, node, edge) : node;
  }
  const { axis, before } = EDGE_AXIS[edge];
  const index = node.children.findIndex(
    child => child.kind === "leaf" && child.windowId === targetWindowId
  );
  if (index >= 0 && node.axis === axis) {
    const at = before ? index : index + 1;
    const children = [...node.children.slice(0, at), incoming, ...node.children.slice(at)];
    return { ...node, children, weights: equalWeights(children.length) };
  }
  return {
    ...node,
    children: node.children.map(child => insertBesideWindow(child, targetWindowId, incoming, edge)),
  };
}

/** A full-height or full-width member at the tab's outer edge (the daemon's root placement). */
export function insertAtRootEdge(
  root: LayoutNode,
  incoming: LayoutNode,
  edge: DropEdge
): LayoutNode {
  const { axis, before } = EDGE_AXIS[edge];
  if (root.kind === "split" && root.axis === axis) {
    const children = before ? [incoming, ...root.children] : [...root.children, incoming];
    return { ...root, children, weights: equalWeights(children.length) };
  }
  return pair(incoming, root, edge);
}

export interface DropFitInput {
  /** The tab whose content receives the drop, as it is now. */
  tab: LayoutTab;
  /** What lands: a whole tab's tree, or one window's leaf. */
  incoming: LayoutNode;
  /** A window that leaves this same tab for the drop (a head moved inside its split). */
  leavingWindowId?: WindowId;
  drop: ContentDrop;
  workArea: PixelRect;
  gaps: ProjectionGaps;
  minimums: WindowMinimums;
}

function shortfalls(tab: LayoutTab, input: DropFitInput): number {
  const projection = projectTab({
    tab,
    workArea: input.workArea,
    gaps: input.gaps,
    minimums: input.minimums,
  });
  return projection.diagnostics.length;
}

/**
 * A drop is offered only when its result keeps every member at its pixel minimum: it is withheld
 * when the resulting tab has a member or split that no longer fits and did not before.
 */
export function dropFits(input: DropFitInput): boolean {
  const base =
    input.leavingWindowId === undefined
      ? input.tab.root
      : withoutWindow(input.tab.root, input.leavingWindowId);
  if (base === null) return false;
  const root =
    input.drop.kind === "root"
      ? insertAtRootEdge(base, input.incoming, input.drop.edge)
      : insertBesideWindow(base, input.drop.windowId, input.incoming, input.drop.edge);
  return (
    shortfalls({ ...input.tab, root }, input) <= shortfalls({ ...input.tab, root: base }, input)
  );
}

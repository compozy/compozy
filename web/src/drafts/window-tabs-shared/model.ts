/**
 * The draft's layout model, shaped like snapshot v6 (`_spec.md` Core Interfaces): a desktop owns
 * tabs, a tab owns one layout tree, and a tree is leaves (windows) and splits (weighted children).
 */
import type { StateGlyphState } from "@compozy/ui";

export type DraftAppId = "session" | "terminal" | "files" | "dashboard" | "agents" | "automations";

export interface DraftWindow {
  id: string;
  app: DraftAppId;
  /** What the tab names: the open session, terminal or folder. */
  title: string;
  /** Session windows carry their provider's mark on the tab tile. */
  provider?: "claude" | "codex" | "gemini";
  /** Session activity, drawn with the session state glyphs. */
  signal?: Extract<StateGlyphState, "running" | "attention" | "failed">;
  /** Decisions waiting in that session. */
  pending?: number;
}

export type SplitDirection = "row" | "column";

export type LayoutNode =
  | { kind: "leaf"; windowId: string }
  | {
      kind: "split";
      id: string;
      direction: SplitDirection;
      children: readonly LayoutNode[];
      weights: readonly number[];
    };

export interface DraftTab {
  id: string;
  pinned?: boolean;
  root: LayoutNode;
}

export interface DraftDesktop {
  id: string;
  name: string;
  tabs: readonly DraftTab[];
  activeTabId: string;
  needsYou?: boolean;
}

export type DropEdge = "left" | "right" | "top" | "bottom";

export function leaf(windowId: string): LayoutNode {
  return { kind: "leaf", windowId };
}

export function split(
  id: string,
  direction: SplitDirection,
  children: readonly LayoutNode[],
  weights?: readonly number[]
): LayoutNode {
  return { kind: "split", id, direction, children, weights: weights ?? children.map(() => 1) };
}

/** Every window of a tree, in reading order. */
export function leafIds(node: LayoutNode): string[] {
  return node.kind === "leaf" ? [node.windowId] : node.children.flatMap(leafIds);
}

/** The window that names a tab: the first of its tree. */
export function leadWindowId(tab: DraftTab): string {
  return leafIds(tab.root)[0]!;
}

/** Removes a window; a split left with one child collapses into it. */
export function removeLeaf(node: LayoutNode, windowId: string): LayoutNode | null {
  if (node.kind === "leaf") return node.windowId === windowId ? null : node;
  const kept: LayoutNode[] = [];
  const weights: number[] = [];
  node.children.forEach((child, index) => {
    const next = removeLeaf(child, windowId);
    if (next) {
      kept.push(next);
      weights.push(node.weights[index] ?? 1);
    }
  });
  if (kept.length === 0) return null;
  if (kept.length === 1) return kept[0]!;
  return { ...node, children: kept, weights };
}

const EDGE_DIRECTION: Record<DropEdge, SplitDirection> = {
  left: "row",
  right: "row",
  top: "column",
  bottom: "column",
};

function before(edge: DropEdge): boolean {
  return edge === "left" || edge === "top";
}

let splitSeq = 0;
function nextSplitId(): string {
  splitSeq += 1;
  return `split-live-${splitSeq}`;
}

/**
 * Places `incoming` at an edge of a window: a split on the same axis is extended with an equal
 * share, another axis nests a new split (US-007.AC-3).
 */
export function insertAtWindow(
  node: LayoutNode,
  targetId: string,
  edge: DropEdge,
  incoming: LayoutNode
): LayoutNode {
  const direction = EDGE_DIRECTION[edge];
  if (node.kind === "leaf") {
    if (node.windowId !== targetId) return node;
    const children = before(edge) ? [incoming, node] : [node, incoming];
    return split(nextSplitId(), direction, children);
  }
  const index = node.children.findIndex(
    child => child.kind === "leaf" && child.windowId === targetId
  );
  if (index >= 0 && node.direction === direction) {
    const at = before(edge) ? index : index + 1;
    const children = [...node.children.slice(0, at), incoming, ...node.children.slice(at)];
    return { ...node, children, weights: children.map(() => 1) };
  }
  return {
    ...node,
    children: node.children.map(child => insertAtWindow(child, targetId, edge, incoming)),
  };
}

/** Adds a full-height or full-width member at the tab's outer edge (US-007.AC-5). */
export function insertAtRoot(root: LayoutNode, edge: DropEdge, incoming: LayoutNode): LayoutNode {
  const direction = EDGE_DIRECTION[edge];
  if (root.kind === "split" && root.direction === direction) {
    const children = before(edge) ? [incoming, ...root.children] : [...root.children, incoming];
    return { ...root, children, weights: children.map(() => 1) };
  }
  return split(nextSplitId(), direction, before(edge) ? [incoming, root] : [root, incoming]);
}

/** The edge of a box nearest a point, both as 0–1 fractions: exactly one for every position. */
export function nearestEdge(x: number, y: number): DropEdge {
  const distances: [DropEdge, number][] = [
    ["left", x],
    ["right", 1 - x],
    ["top", y],
    ["bottom", 1 - y],
  ];
  return distances.reduce((best, next) => (next[1] < best[1] ? next : best))[0];
}

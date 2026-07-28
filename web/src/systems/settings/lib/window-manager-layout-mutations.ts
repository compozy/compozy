import { layoutNodeWindowIds } from "./window-manager-layout-tree";
import type {
  WindowManagerLayoutAxis,
  WindowManagerLayoutNode,
} from "./window-manager-layout-types";

/**
 * What the person chooses. The daemon's `axis` divides the width on
 * `horizontal` (`layout-projection.ts` `projectSplit`), so "columns" is
 * `horizontal` and "rows" is `vertical` — the inverse of what the words suggest.
 * That mapping lives here, once, and the UI never says "axis".
 */
export type LayoutOrientation = "rows" | "columns";

export const LAYOUT_ORIENTATION_AXIS: Readonly<Record<LayoutOrientation, WindowManagerLayoutAxis>> =
  {
    rows: "vertical",
    columns: "horizontal",
  };

export function layoutOrientationOf(axis: WindowManagerLayoutAxis): LayoutOrientation {
  return axis === "vertical" ? "rows" : "columns";
}

export type LayoutNodeIdFactory = (kind: WindowManagerLayoutNode["kind"]) => string;

/** Every node id in a subtree, used to mint ids that cannot collide with the draft. */
export function layoutNodeIds(node: WindowManagerLayoutNode): string[] {
  if (node.kind !== "split") return [node.id];
  return [node.id, ...node.children.flatMap(layoutNodeIds)];
}

/**
 * Client-minted ids are namespaced and checked against the ids already in the
 * document: a remount resets any counter, and a reused id would make a later
 * node replacement rewrite the wrong branch.
 */
export function createLayoutNodeIdFactory(taken: Iterable<string>): LayoutNodeIdFactory {
  const used = new Set(taken);
  let sequence = 0;
  return kind => {
    let candidate = "";
    do {
      sequence += 1;
      candidate = `${kind}:settings-${sequence}`;
    } while (used.has(candidate));
    used.add(candidate);
    return candidate;
  };
}

/**
 * `validate.go` requires `|Σweights − 1| ≤ 1e-6` and the daemon validates the
 * document exactly as sent — nothing normalizes it on the way in. Every weight
 * vector this module emits is already normalized, so `topology.split_weight_sum`
 * is unreachable from the editor rather than merely reported by it.
 */
export function evenWeights(count: number): number[] {
  if (count < 1) return [];
  return Array.from({ length: count }, () => 1 / count);
}

/** Flattens a subtree into sibling leaves laid out along the chosen orientation. */
export function toSplit(
  node: WindowManagerLayoutNode,
  orientation: LayoutOrientation,
  mintId: LayoutNodeIdFactory
): WindowManagerLayoutNode {
  const windowIds = layoutNodeWindowIds(node);
  return {
    id: mintId("split"),
    kind: "split",
    axis: LAYOUT_ORIENTATION_AXIS[orientation],
    children: windowIds.map(windowId => ({
      id: mintId("leaf"),
      kind: "leaf" as const,
      windowId,
    })),
    weights: evenWeights(windowIds.length),
  };
}

/** Flattens a subtree into one stack, keeping the first window on top. */
export function toStack(
  node: WindowManagerLayoutNode,
  mintId: LayoutNodeIdFactory
): WindowManagerLayoutNode {
  const windowIds = layoutNodeWindowIds(node);
  return {
    id: mintId("stack"),
    kind: "stack",
    windowIds,
    activeId: windowIds[0] ?? "",
  };
}

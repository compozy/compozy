import { projectTab } from "./tab-projection";
import type { TabProjectionInput } from "./tab-projection-types";
import type { LayoutNode, LayoutNodeId, LayoutSplitNode } from "./window-manager-types";

/** The daemon's `weightTolerance`: a split within it of `1/n` each is already even. */
const EVEN_WEIGHT_TOLERANCE = 0.000001;

/**
 * What "Balance sizes" sends for one tab. `whole`: every split, evened together, still lays out
 * side by side wherever it does now, so one tab-wide `layout.balance` evens them all. Otherwise
 * `splitIds` names the splits that can be evened together without that, outermost first, each to
 * be balanced on its own; the others keep their weights. Empty when none can.
 */
export interface TabBalancePlan {
  whole: boolean;
  splitIds: readonly LayoutNodeId[];
}

function isEven(split: LayoutSplitNode): boolean {
  const expected = 1 / split.weights.length;
  return split.weights.every(weight => Math.abs(weight - expected) <= EVEN_WEIGHT_TOLERANCE);
}

/** Every split of the tree, outermost first. */
function splitsOf(node: LayoutNode): LayoutSplitNode[] {
  if (node.kind === "leaf") return [];
  return [node, ...node.children.flatMap(splitsOf)];
}

/** The tree with these splits' weights set to `1/n` each, as `layout.balance` writes them. */
function evened(node: LayoutNode, splitIds: ReadonlySet<LayoutNodeId>): LayoutNode {
  if (node.kind === "leaf") return node;
  const children = node.children.map(child => evened(child, splitIds));
  const weights = splitIds.has(node.id)
    ? node.children.map(() => 1 / node.children.length)
    : node.weights;
  return { ...node, children, weights };
}

/** The splits the projection lays out side by side: those that show a seam. */
function laidOutSplits(input: TabProjectionInput): Set<LayoutNodeId> {
  return new Set(projectTab(input).seams.map(seam => seam.splitId));
}

/**
 * The fit admission of a tab-wide balance: the seam double-click's rule ("a balance never leaves
 * a split one member at a time") solved jointly over every split of the tab, since evening an
 * outer split resizes every split inside it. The splits evened are tested together on the whole
 * projected tab. When evening all of them would turn a split that lays out side by side now to one
 * member at a time, the splits are tried outermost first and each one is kept only while the
 * tab, with every split kept so far, still lays them all out. Cost: one projection of the tab per
 * uneven split, plus two.
 */
export function planTabBalance(input: TabProjectionInput): TabBalancePlan {
  const uneven = splitsOf(input.tab.root).filter(split => !isEven(split));
  const required = laidOutSplits(input);
  const fits = (splitIds: ReadonlySet<LayoutNodeId>) => {
    const tab = { ...input.tab, root: evened(input.tab.root, splitIds) };
    const shown = laidOutSplits({ ...input, tab });
    return [...required].every(splitId => shown.has(splitId));
  };
  const all = new Set(uneven.map(split => split.id));
  if (fits(all)) return { whole: true, splitIds: [...all] };
  const kept = new Set<LayoutNodeId>();
  for (const split of uneven) {
    kept.add(split.id);
    if (!fits(kept)) kept.delete(split.id);
  }
  return { whole: false, splitIds: [...kept] };
}

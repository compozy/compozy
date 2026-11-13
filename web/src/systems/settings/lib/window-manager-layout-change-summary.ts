import type {
  WindowManagerLayoutNode,
  WindowManagerLayoutPreview,
} from "./window-manager-layout-types";

type CountedNoun = readonly [one: string, many: string];

const DESKTOP: CountedNoun = ["desktop", "desktops"];
const TAB: CountedNoun = ["tab", "tabs"];
const SPLIT: CountedNoun = ["split", "splits"];
const WINDOW: CountedNoun = ["window", "windows"];

function counted(count: number, [one, many]: CountedNoun): string | null {
  if (count === 0) return null;
  return `${count} ${count === 1 ? one : many}`;
}

function collectSplitIds(node: WindowManagerLayoutNode, into: Set<string>): void {
  if (node.kind !== "split") return;
  into.add(node.id);
  for (const child of node.children) collectSplitIds(child, into);
}

/**
 * The daemon reports every changed tree node, leaves included; a leaf is a
 * window slot, already counted as a window. Only split nodes of the layout that
 * would apply count as splits.
 */
function changedSplitCount(preview: WindowManagerLayoutPreview): number {
  const splitIds = new Set<string>();
  for (const desktop of preview.document.desktops) {
    for (const tab of desktop.tabs) collectSplitIds(tab.root, splitIds);
  }
  return preview.changes.nodeIds.filter(id => splitIds.has(id)).length;
}

/** "changes 1 desktop, 5 tabs, 2 splits, 6 windows" in the product's nouns; zero counts are left out. */
export function layoutChangeSummary(preview: WindowManagerLayoutPreview): string {
  const { changes } = preview;
  const parts = [
    counted(changes.desktopIds.length, DESKTOP),
    counted(changes.tabIds.length, TAB),
    counted(changedSplitCount(preview), SPLIT),
    counted(changes.windowIds.length, WINDOW),
  ].filter((part): part is string => part !== null);
  return parts.length === 0 ? "nothing changes" : `changes ${parts.join(", ")}`;
}

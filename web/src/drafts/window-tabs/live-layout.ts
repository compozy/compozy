/** The interactive state's layout and the rules a finished gesture applies to it. */
import type { DragGhostSource } from "../window-tabs-shared/drag-ghost";
import {
  insertAtRoot,
  insertAtWindow,
  leaf,
  leafIds,
  removeLeaf,
  type DraftTab,
  type LayoutNode,
} from "../window-tabs-shared/model";
import type { DropTarget } from "./live-drag";

export interface Layout {
  tabs: readonly DraftTab[];
  activeTabId: string;
  focusedId: string;
}

export interface Drag {
  source: DragGhostSource;
  /** The tab whose content shows as the drop target. */
  contentTabId: string | null;
  x: number;
  y: number;
  target: DropTarget;
}

export function tabOf(tabs: readonly DraftTab[], windowId: string): DraftTab | undefined {
  return tabs.find(tab => leafIds(tab.root).some(id => id === windowId));
}

export function mapSplit(
  node: LayoutNode,
  splitId: string,
  update: (weights: readonly number[]) => number[]
): LayoutNode {
  if (node.kind === "leaf") return node;
  if (node.id === splitId) return { ...node, weights: update(node.weights) };
  return { ...node, children: node.children.map(child => mapSplit(child, splitId, update)) };
}

/** Changes the shares of the two sides of one seam and nothing else (US-009.AC-1). */
export function resizePair(
  layout: Layout,
  splitId: string,
  index: number,
  share: (pair: number) => number
): Layout {
  return {
    ...layout,
    tabs: layout.tabs.map(tab => ({
      ...tab,
      root: mapSplit(tab.root, splitId, weights => {
        const pair = (weights[index] ?? 1) + (weights[index + 1] ?? 1);
        const updated = [...weights];
        updated[index] = share(pair);
        updated[index + 1] = pair - updated[index]!;
        return updated;
      }),
    })),
  };
}

/** Closes a tab; the tab to its right becomes active, or the one to its left (US-002.AC-1). */
export function closeTab(layout: Layout, tabId: string): Layout {
  const index = layout.tabs.findIndex(tab => tab.id === tabId);
  if (index < 0 || layout.tabs.length === 1) return layout;
  const tabs = layout.tabs.filter(tab => tab.id !== tabId);
  if (layout.activeTabId !== tabId) return { ...layout, tabs };
  const next = tabs[Math.min(index, tabs.length - 1)]!;
  return { tabs, activeTabId: next.id, focusedId: leafIds(next.root)[0]! };
}

/** Closes a window: its sibling takes the space, and the last one closes the tab (US-011). */
export function closeWindow(layout: Layout, windowId: string): Layout {
  const tab = tabOf(layout.tabs, windowId);
  if (!tab) return layout;
  const root = removeLeaf(tab.root, windowId);
  if (!root) return closeTab(layout, tab.id);
  const tabs = layout.tabs.map(item => (item.id === tab.id ? { ...item, root } : item));
  return { ...layout, tabs, focusedId: leafIds(root)[0]! };
}

function dropOnBar(layout: Layout, sourceTab: DraftTab, moving: LayoutNode, index: number) {
  const focusedId = leafIds(moving)[0]!;
  if (leafIds(sourceTab.root).length === leafIds(moving).length) {
    const without = layout.tabs.filter(tab => tab.id !== sourceTab.id);
    const at = layout.tabs.slice(0, index).filter(tab => tab.id !== sourceTab.id).length;
    const tabs = [...without.slice(0, at), sourceTab, ...without.slice(at)];
    return { tabs, activeTabId: sourceTab.id, focusedId };
  }
  const rest = removeLeaf(sourceTab.root, focusedId)!;
  const moved: DraftTab = { id: `t-live-${focusedId}-${layout.tabs.length}`, root: moving };
  const tabs = layout.tabs.map(tab => (tab.id === sourceTab.id ? { ...tab, root: rest } : tab));
  return {
    tabs: [...tabs.slice(0, index), moved, ...tabs.slice(index)],
    activeTabId: moved.id,
    focusedId,
  };
}

/** Applies a finished drag to the layout; an unresolved drop changes nothing. */
export function applyDrop(layout: Layout, drag: Drag): Layout {
  const { source, target } = drag;
  if (!target) return layout;
  const sourceTab = source.kind === "tab" ? source.tab : tabOf(layout.tabs, source.windowId);
  if (!sourceTab) return layout;
  const moving: LayoutNode = source.kind === "tab" ? sourceTab.root : leaf(source.windowId);
  if (target.kind === "bar") return dropOnBar(layout, sourceTab, moving, target.index);

  const hostId = drag.contentTabId;
  if (!hostId) return layout;
  const focusedId = leafIds(moving)[0]!;
  const wholeTab = source.kind === "tab";
  const tabs: DraftTab[] = [];
  for (const tab of layout.tabs) {
    if (wholeTab && tab.id === sourceTab.id) continue;
    let root = tab.root;
    if (!wholeTab && tab.id === sourceTab.id) root = removeLeaf(root, focusedId)!;
    if (tab.id === hostId) {
      root =
        target.kind === "root"
          ? insertAtRoot(root, target.edge, moving)
          : insertAtWindow(root, target.windowId, target.edge, moving);
    }
    tabs.push({ ...tab, root });
  }
  return { tabs, activeTabId: hostId, focusedId };
}

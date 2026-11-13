import type { windowManagerSnapshotFixture } from "./fixtures";

/**
 * A wire-level stand-in for the daemon's tab reducer (`internal/windowmanager/reducer_*.go`),
 * close enough for stories and route tests: tabs open, close, reopen, reorder, pin, move between
 * desktops and splits, and arrange into one split tab. It is not a second source of truth.
 */
export type MockSnapshot = typeof windowManagerSnapshotFixture;
export type MockDesktop = MockSnapshot["desktops"][number];
export type MockTab = MockDesktop["tabs"][number];
export type MockNode = MockTab["root"];
export type MockWindow = MockSnapshot["windows"][string];

export interface MockClosedEntry {
  desktopId: string;
  tabs: { tab: MockTab; index: number }[];
  windows: MockWindow[];
}

export interface MockReduction {
  windowIds: string[];
  tabIds: string[];
  desktopIds: string[];
  /** The window the acting client focuses next; `null` clears focus, absent keeps it. */
  focus?: string | null;
  /** The desktop and tab the acting client shows next. */
  show?: { desktopId: string; tabId: string };
}

export class MockCommandError extends Error {
  constructor(
    readonly kind: "invalid-command" | "window-not-found",
    message: string,
    readonly windowId = ""
  ) {
    super(message);
  }
}

let generated = 0;
export function generateId(prefix: string): string {
  generated += 1;
  return `${prefix}-mock-${generated}`;
}

export function nodeWindowIds(node: MockNode): string[] {
  if (node.kind === "leaf") return node.window_id ? [node.window_id] : [];
  return (node.children ?? []).flatMap(nodeWindowIds);
}

export function leaf(windowId: string): MockNode {
  return { id: generateId("node"), kind: "leaf", window_id: windowId };
}

export function equalWeights(count: number): number[] {
  return Array.from({ length: count }, () => 1 / count);
}

/** Removes a window's leaf; a split left with one child collapses into it. */
function removeLeaf(node: MockNode, windowId: string): MockNode | null {
  if (node.kind === "leaf") return node.window_id === windowId ? null : node;
  const children = node.children ?? [];
  const weights = node.weights ?? equalWeights(children.length);
  const kept: { child: MockNode; weight: number }[] = [];
  children.forEach((child, index) => {
    const next = removeLeaf(child, windowId);
    if (next !== null) kept.push({ child: next, weight: weights[index] ?? 0 });
  });
  if (kept.length === 0) return null;
  if (kept.length === 1) return kept[0]?.child ?? null;
  const total = kept.reduce((sum, entry) => sum + entry.weight, 0) || 1;
  return {
    ...node,
    children: kept.map(entry => entry.child),
    weights: kept.map(entry => entry.weight / total),
  };
}

export function findTabOf(
  snapshot: MockSnapshot,
  windowId: string
): { desktop: MockDesktop; index: number; tab: MockTab } {
  for (const desktop of snapshot.desktops) {
    const index = desktop.tabs.findIndex(tab => nodeWindowIds(tab.root).includes(windowId));
    const tab = desktop.tabs[index];
    if (tab !== undefined) return { desktop, index, tab };
  }
  throw new MockCommandError("window-not-found", `Window not found: ${windowId}`, windowId);
}

export function desktopById(snapshot: MockSnapshot, desktopId: string): MockDesktop {
  const desktop = snapshot.desktops.find(candidate => candidate.id === desktopId);
  if (!desktop) throw new MockCommandError("invalid-command", `Unknown desktop ${desktopId}.`);
  return desktop;
}

/** Inserts within the tab's pinned or unpinned range, like the daemon's `insertTab`. */
export function insertTab(desktop: MockDesktop, tab: MockTab, index: number): void {
  const pins = desktop.tabs.filter(candidate => candidate.pinned).length;
  const [min, max] = tab.pinned ? [0, pins] : [pins, desktop.tabs.length];
  desktop.tabs.splice(Math.min(max, Math.max(min, index)), 0, tab);
}

export function assignTab(snapshot: MockSnapshot, desktop: MockDesktop, tab: MockTab): void {
  for (const id of nodeWindowIds(tab.root)) {
    const window = snapshot.windows[id];
    if (window) snapshot.windows[id] = { ...window, desktop_id: desktop.id, tab_id: tab.id };
  }
}

/** Drops a window from its tab; an emptied tab leaves the bar and the default moves on. */
export function detachWindow(snapshot: MockSnapshot, windowId: string): void {
  const { desktop, index, tab } = findTabOf(snapshot, windowId);
  const root = removeLeaf(tab.root, windowId);
  if (root !== null) {
    desktop.tabs[index] = { ...tab, root };
    return;
  }
  desktop.tabs.splice(index, 1);
  repairActiveTab(desktop, index);
}

export function repairActiveTab(desktop: MockDesktop, removedIndex: number): void {
  if (desktop.tabs.some(tab => tab.id === desktop.active_tab_id)) return;
  const neighbor = desktop.tabs[Math.min(removedIndex, desktop.tabs.length - 1)];
  desktop.active_tab_id = neighbor?.id ?? "";
}

function lead(tab: MockTab | undefined): string | null {
  return tab ? (nodeWindowIds(tab.root)[0] ?? null) : null;
}

export function openWindow(
  snapshot: MockSnapshot,
  spec: {
    id: string;
    app: string;
    instanceKey?: string;
    route: MockWindow["route"];
    desktopId: string;
    afterWindowId?: string;
    split: boolean;
    focusedWindowId: string | null;
    activeTabId: string | null;
  }
): MockReduction {
  if (snapshot.windows[spec.id]) {
    throw new MockCommandError("invalid-command", `Window ${spec.id} already exists.`);
  }
  const anchorId = spec.afterWindowId ?? spec.focusedWindowId;
  const anchor = anchorId ? snapshot.windows[anchorId] : undefined;
  const desktop = desktopById(snapshot, anchor?.desktop_id ?? spec.desktopId);
  snapshot.windows[spec.id] = {
    id: spec.id,
    app: spec.app,
    ...(spec.instanceKey === undefined ? {} : { instance_key: spec.instanceKey }),
    route: spec.route,
    nav_stack: [],
    desktop_id: desktop.id,
    tab_id: "",
  };
  const activeTabId =
    anchor?.desktop_id === desktop.id ? anchor.tab_id : (spec.activeTabId ?? desktop.active_tab_id);
  const activeIndex = desktop.tabs.findIndex(tab => tab.id === activeTabId);
  const activeTab = desktop.tabs[activeIndex];
  if (spec.split && !spec.afterWindowId && activeTab) {
    const root: MockNode = {
      id: generateId("node"),
      kind: "split",
      axis: "horizontal",
      weights: [0.5, 0.5],
      children: [activeTab.root, leaf(spec.id)],
    };
    desktop.tabs[activeIndex] = { ...activeTab, root };
    assignTab(snapshot, desktop, { ...activeTab, root });
    desktop.active_tab_id = activeTab.id;
    return {
      windowIds: [spec.id],
      tabIds: [activeTab.id],
      desktopIds: [desktop.id],
      focus: spec.id,
      show: { desktopId: desktop.id, tabId: activeTab.id },
    };
  }
  const tab: MockTab = { id: generateId("tab"), pinned: false, root: leaf(spec.id) };
  insertTab(desktop, tab, activeIndex + 1);
  assignTab(snapshot, desktop, tab);
  desktop.active_tab_id = tab.id;
  return {
    windowIds: [spec.id],
    tabIds: [tab.id],
    desktopIds: [desktop.id],
    focus: spec.id,
    show: { desktopId: desktop.id, tabId: tab.id },
  };
}

/** Mirrors `reducer_window_close.go`: bar scopes skip pinned tabs, `tab` closes one window. */
export function closeWindows(
  snapshot: MockSnapshot,
  windowId: string,
  scope: string,
  closed: MockClosedEntry[]
): MockReduction {
  const { desktop, index, tab } = findTabOf(snapshot, windowId);
  const entry: MockClosedEntry = { desktopId: desktop.id, tabs: [], windows: [] };
  if (scope === "tab") {
    const members = nodeWindowIds(tab.root);
    if (members.length === 1 && tab.pinned) {
      throw new MockCommandError("invalid-command", "Unpin the tab before closing it.");
    }
    if (members.length === 1) entry.tabs.push({ tab: structuredClone(tab), index });
  } else {
    desktop.tabs.forEach((candidate, candidateIndex) => {
      const selected =
        scope === "group"
          ? candidateIndex === index
          : !candidate.pinned &&
            (scope === "all" ||
              (scope === "others" && candidateIndex !== index) ||
              (scope === "right" && candidateIndex > index) ||
              (scope === "left" && candidateIndex < index));
      if (selected) entry.tabs.push({ tab: structuredClone(candidate), index: candidateIndex });
    });
  }
  const ids =
    scope === "tab"
      ? [windowId]
      : entry.tabs.flatMap(closedTab => nodeWindowIds(closedTab.tab.root));
  if (ids.length === 0) return { windowIds: [], tabIds: [], desktopIds: [] };
  for (const id of ids) {
    const window = snapshot.windows[id];
    if (window) entry.windows.push(structuredClone(window));
    detachWindow(snapshot, id);
    delete snapshot.windows[id];
  }
  closed.push(entry);
  const shownTab = desktop.tabs.find(candidate => candidate.id === desktop.active_tab_id);
  return {
    windowIds: ids,
    tabIds: [tab.id, ...entry.tabs.map(closedTab => closedTab.tab.id)],
    desktopIds: [desktop.id],
    focus: lead(shownTab),
    ...(shownTab ? { show: { desktopId: desktop.id, tabId: shownTab.id } } : {}),
  };
}

/** Restores the newest closed entry: whole tabs at their index, a split member as a new tab. */
export function reopenClosed(snapshot: MockSnapshot, closed: MockClosedEntry[]): MockReduction {
  const entry = closed.pop();
  if (entry === undefined) return { windowIds: [], tabIds: [], desktopIds: [] };
  const desktop =
    snapshot.desktops.find(candidate => candidate.id === entry.desktopId) ?? snapshot.desktops[0];
  if (desktop === undefined) return { windowIds: [], tabIds: [], desktopIds: [] };
  for (const window of entry.windows) snapshot.windows[window.id] = structuredClone(window);
  const tabs =
    entry.tabs.length > 0
      ? entry.tabs
      : entry.windows.map(window => ({
          tab: { id: generateId("tab"), pinned: false, root: leaf(window.id) },
          index: desktop.tabs.length,
        }));
  for (const { tab, index } of tabs) {
    insertTab(desktop, tab, index);
    assignTab(snapshot, desktop, tab);
    desktop.active_tab_id = tab.id;
  }
  const shown = tabs[tabs.length - 1]?.tab;
  return {
    windowIds: entry.windows.map(window => window.id),
    tabIds: tabs.map(({ tab }) => tab.id),
    desktopIds: [desktop.id],
    focus: lead(shown),
    ...(shown ? { show: { desktopId: desktop.id, tabId: shown.id } } : {}),
  };
}

export function reorderTab(snapshot: MockSnapshot, windowId: string, index: number): MockReduction {
  const placement = findTabOf(snapshot, windowId);
  placement.desktop.tabs.splice(placement.index, 1);
  insertTab(placement.desktop, placement.tab, index);
  return { windowIds: [], tabIds: [placement.tab.id], desktopIds: [placement.desktop.id] };
}

export function activateTab(snapshot: MockSnapshot, windowId: string): MockReduction {
  const { desktop, tab } = findTabOf(snapshot, windowId);
  desktop.active_tab_id = tab.id;
  return {
    windowIds: [],
    tabIds: [tab.id],
    desktopIds: [desktop.id],
    focus: windowId,
    show: { desktopId: desktop.id, tabId: tab.id },
  };
}

/** Focus shows the window's tab for the acting client only; the desktop default stays. */
export function focusWindow(snapshot: MockSnapshot, windowId: string): MockReduction {
  const { desktop, tab } = findTabOf(snapshot, windowId);
  return {
    windowIds: [],
    tabIds: [],
    desktopIds: [],
    focus: windowId,
    show: { desktopId: desktop.id, tabId: tab.id },
  };
}

export function pinTab(snapshot: MockSnapshot, windowId: string, pinned: boolean): MockReduction {
  const placement = findTabOf(snapshot, windowId);
  if (placement.tab.pinned === pinned) return { windowIds: [], tabIds: [], desktopIds: [] };
  placement.desktop.tabs.splice(placement.index, 1);
  const tab = { ...placement.tab, pinned };
  const pins = placement.desktop.tabs.filter(candidate => candidate.pinned).length;
  insertTab(placement.desktop, tab, pins);
  return { windowIds: [], tabIds: [tab.id], desktopIds: [placement.desktop.id] };
}

function arrangeNodes(nodes: MockNode[], arrangement: string): MockNode {
  if (nodes.length === 1 && nodes[0]) return nodes[0];
  const split = (children: MockNode[], axis: "horizontal" | "vertical", weights?: number[]) => ({
    id: generateId("node"),
    kind: "split" as const,
    axis,
    children,
    weights: weights ?? equalWeights(children.length),
  });
  if (arrangement === "vertical") return split(nodes, "vertical");
  if (arrangement === "main_stack") {
    return split(
      [nodes[0] as MockNode, arrangeNodes(nodes.slice(1), "vertical")],
      "horizontal",
      [0.6, 0.4]
    );
  }
  if (arrangement === "grid") {
    const columns = Math.ceil(Math.sqrt(nodes.length));
    const rows: MockNode[] = [];
    for (let start = 0; start < nodes.length; start += columns) {
      rows.push(arrangeNodes(nodes.slice(start, start + columns), "horizontal"));
    }
    return arrangeNodes(rows, "vertical");
  }
  return split(nodes, "horizontal");
}

/** `layout.arrange`: the named windows leave their tabs and form one split tab in the first's place. */
export function arrange(
  snapshot: MockSnapshot,
  desktopId: string,
  windowIds: readonly string[],
  arrangement: string
): MockReduction {
  const desktop = desktopById(snapshot, desktopId);
  const first = windowIds[0];
  if (first === undefined) throw new MockCommandError("invalid-command", "Nothing to arrange.");
  const index = findTabOf(snapshot, first).index;
  for (const id of windowIds) detachWindow(snapshot, id);
  const tab: MockTab = {
    id: generateId("tab"),
    pinned: false,
    root: arrangeNodes(windowIds.map(leaf), arrangement),
  };
  insertTab(desktop, tab, index);
  assignTab(snapshot, desktop, tab);
  desktop.active_tab_id = tab.id;
  return {
    windowIds: [...windowIds],
    tabIds: [tab.id],
    desktopIds: [desktop.id],
    focus: first,
    show: { desktopId: desktop.id, tabId: tab.id },
  };
}

function containsNode(node: MockNode, nodeId: string): boolean {
  return node.id === nodeId || (node.children ?? []).some(child => containsNode(child, nodeId));
}

function mapSplits(node: MockNode, update: (split: MockNode) => MockNode): MockNode {
  if (node.kind === "leaf") return node;
  return update({
    ...node,
    children: (node.children ?? []).map(child => mapSplits(child, update)),
  });
}

/** `layout.balance` and `layout.resize`: weight edits on one split or every split of a tab. */
export function editWeights(
  snapshot: MockSnapshot,
  target: { tabId?: string; splitId?: string },
  update: (split: MockNode) => MockNode
): MockReduction {
  for (const desktop of snapshot.desktops) {
    desktop.tabs.forEach((tab, index) => {
      const touches =
        tab.id === target.tabId ||
        (target.splitId !== undefined && containsNode(tab.root, target.splitId));
      if (!touches) return;
      desktop.tabs[index] = {
        ...tab,
        root: mapSplits(tab.root, split =>
          target.splitId === undefined || split.id === target.splitId ? update(split) : split
        ),
      };
    });
  }
  return { windowIds: [], tabIds: target.tabId ? [target.tabId] : [], desktopIds: [] };
}

export function balancedSplit(split: MockNode): MockNode {
  return { ...split, weights: equalWeights(split.children?.length ?? 0) };
}

export function resizedSplit(boundaryIndex: number, delta: number) {
  return (split: MockNode): MockNode => {
    const weights = [...(split.weights ?? [])];
    const leading = weights[boundaryIndex];
    const trailing = weights[boundaryIndex + 1];
    if (leading === undefined || trailing === undefined) return split;
    const clamped = Math.min(trailing - 0.01, Math.max(0.01 - leading, delta));
    weights[boundaryIndex] = leading + clamped;
    weights[boundaryIndex + 1] = trailing - clamped;
    return { ...split, weights };
  };
}

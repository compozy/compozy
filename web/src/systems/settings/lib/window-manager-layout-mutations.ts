import {
  findLayoutLeaf,
  insertBesideLeaf,
  layoutNodeIds,
  layoutNodeWindowIds,
  normalizeLayoutNode,
  removeLayoutLeaf,
  swapLayoutWindows,
  type LayoutNodeIdFactory,
} from "./window-manager-layout-tree";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerLayoutEdge,
  WindowManagerLayoutNode,
  WindowManagerLayoutTab,
  WindowManagerLayoutWindow,
} from "./window-manager-layout-types";

/**
 * Every edit the layout editor makes is one of these pure functions. Each
 * returns the document unchanged when the edit would break a rule the daemon
 * enforces (a desktop without tabs, a window shown twice), and otherwise runs
 * the result through `normalizeLayoutDocument`, so the editor can only produce
 * documents that pass `layoutDocumentProblems`.
 */

/**
 * Client-minted ids are namespaced and checked against every id in the
 * document: a remount resets any counter, and a reused id would make a later
 * replacement rewrite the wrong branch.
 */
export function createLayoutIdFactory(document: WindowManagerLayoutDocument): LayoutNodeIdFactory {
  const used = new Set<string>();
  for (const desktop of document.desktops) {
    for (const tab of desktop.tabs) {
      used.add(tab.id);
      for (const id of layoutNodeIds(tab.root)) used.add(id);
    }
  }
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
 * Restores the document invariants the editor can disturb: desktop `order`
 * equals its index, pinned tabs form a prefix (pinning keeps relative order),
 * every desktop's default tab exists, every split is normalized, and each
 * window's `desktop_id` and `tab_id` follow the topology. Windows no leaf
 * shows are dropped, because the daemon requires every window to appear once.
 */
export function normalizeLayoutDocument(
  document: WindowManagerLayoutDocument
): WindowManagerLayoutDocument {
  const placement = new Map<string, { desktopId: string; tabId: string }>();
  const desktops = document.desktops.map((desktop, order): WindowManagerLayoutDesktop => {
    const tabs = [
      ...desktop.tabs.filter(tab => tab.pinned),
      ...desktop.tabs.filter(tab => !tab.pinned),
    ].map(tab => ({ ...tab, root: normalizeLayoutNode(tab.root) }));
    for (const tab of tabs) {
      for (const windowId of layoutNodeWindowIds(tab.root)) {
        placement.set(windowId, { desktopId: desktop.id, tabId: tab.id });
      }
    }
    const activeTabId = tabs.some(tab => tab.id === desktop.activeTabId)
      ? desktop.activeTabId
      : (tabs[0]?.id ?? desktop.activeTabId);
    return { ...desktop, order, tabs, activeTabId };
  });
  const windows: Record<string, WindowManagerLayoutWindow> = {};
  for (const [id, window] of Object.entries(document.windows)) {
    const place = placement.get(id);
    if (place) windows[id] = { ...window, id, desktopId: place.desktopId, tabId: place.tabId };
  }
  return { ...document, desktops, windows };
}

function updateDesktop(
  document: WindowManagerLayoutDocument,
  desktopId: string,
  update: (desktop: WindowManagerLayoutDesktop) => WindowManagerLayoutDesktop | null
): WindowManagerLayoutDocument {
  let changed = false;
  const desktops = document.desktops.map(desktop => {
    if (desktop.id !== desktopId) return desktop;
    const next = update(desktop);
    if (next === null || next === desktop) return desktop;
    changed = true;
    return next;
  });
  return changed ? normalizeLayoutDocument({ ...document, desktops }) : document;
}

/** Where a window is shown: its desktop, tab and the tab's position. */
export function locateLayoutWindow(
  document: WindowManagerLayoutDocument,
  windowId: string
): { desktop: WindowManagerLayoutDesktop; tab: WindowManagerLayoutTab; index: number } | null {
  for (const desktop of document.desktops) {
    const index = desktop.tabs.findIndex(tab => findLayoutLeaf(tab.root, windowId) !== null);
    const tab = desktop.tabs[index];
    if (tab) return { desktop, tab, index };
  }
  return null;
}

export function renameLayoutDesktop(
  document: WindowManagerLayoutDocument,
  desktopId: string,
  name: string
): WindowManagerLayoutDocument {
  const trimmed = name.trim();
  if (trimmed === "") return document;
  return updateDesktop(document, desktopId, desktop =>
    desktop.name === trimmed ? null : { ...desktop, name: trimmed }
  );
}

/** The tab a desktop opens on for a client that has not picked one yet. */
export function setLayoutDefaultTab(
  document: WindowManagerLayoutDocument,
  desktopId: string,
  tabId: string
): WindowManagerLayoutDocument {
  return updateDesktop(document, desktopId, desktop =>
    desktop.activeTabId === tabId || !desktop.tabs.some(tab => tab.id === tabId)
      ? null
      : { ...desktop, activeTabId: tabId }
  );
}

export function setLayoutTabPinned(
  document: WindowManagerLayoutDocument,
  desktopId: string,
  tabId: string,
  pinned: boolean
): WindowManagerLayoutDocument {
  return updateDesktop(document, desktopId, desktop => {
    const index = desktop.tabs.findIndex(tab => tab.id === tabId);
    const tab = desktop.tabs[index];
    if (!tab || tab.pinned === pinned) return null;
    const rest = desktop.tabs.filter(candidate => candidate.id !== tabId);
    // Pinning lands at the end of the pinned area; unpinning at the start of the rest.
    const boundary = rest.filter(candidate => candidate.pinned).length;
    const tabs = [...rest];
    tabs.splice(boundary, 0, { ...tab, pinned });
    return { ...desktop, tabs };
  });
}

/**
 * Moves a tab by `offset` positions within its own area — a pinned tab never
 * crosses into the regular tabs or back, which is what pinning is for.
 */
export function moveLayoutTab(
  document: WindowManagerLayoutDocument,
  desktopId: string,
  tabId: string,
  offset: number
): WindowManagerLayoutDocument {
  return updateDesktop(document, desktopId, desktop => {
    const index = desktop.tabs.findIndex(tab => tab.id === tabId);
    const tab = desktop.tabs[index];
    if (!tab) return null;
    const pinnedCount = desktop.tabs.filter(candidate => candidate.pinned).length;
    const [low, high] = tab.pinned ? [0, pinnedCount - 1] : [pinnedCount, desktop.tabs.length - 1];
    const target = Math.min(Math.max(index + offset, low), high);
    if (target === index) return null;
    const tabs = [...desktop.tabs];
    tabs.splice(index, 1);
    tabs.splice(target, 0, tab);
    return { ...desktop, tabs };
  });
}

/** Closes a tab and the windows it shows. A desktop keeps at least one tab. */
export function closeLayoutTab(
  document: WindowManagerLayoutDocument,
  desktopId: string,
  tabId: string
): WindowManagerLayoutDocument {
  return updateDesktop(document, desktopId, desktop => {
    if (desktop.tabs.length < 2 || !desktop.tabs.some(tab => tab.id === tabId)) return null;
    return { ...desktop, tabs: desktop.tabs.filter(tab => tab.id !== tabId) };
  });
}

/**
 * Takes a window out of its tab. Returns the remaining topology, or null when
 * the window is not shown or its removal would leave a desktop with no tab.
 */
function detachLayoutWindow(
  document: WindowManagerLayoutDocument,
  windowId: string
): { desktops: WindowManagerLayoutDesktop[]; desktopId: string; index: number } | null {
  const location = locateLayoutWindow(document, windowId);
  if (location === null) return null;
  const root = removeLayoutLeaf(location.tab.root, windowId);
  if (root === null && location.desktop.tabs.length < 2) return null;
  const desktops = document.desktops.map(desktop => {
    if (desktop.id !== location.desktop.id) return desktop;
    const tabs = desktop.tabs.flatMap(tab =>
      tab.id !== location.tab.id ? [tab] : root === null ? [] : [{ ...tab, root }]
    );
    return { ...desktop, tabs };
  });
  return { desktops, desktopId: location.desktop.id, index: location.index };
}

/** Removes a window from the layout; its tab closes with it when it was the only member. */
export function closeLayoutWindow(
  document: WindowManagerLayoutDocument,
  windowId: string
): WindowManagerLayoutDocument {
  const detached = detachLayoutWindow(document, windowId);
  if (detached === null) return document;
  return normalizeLayoutDocument({ ...document, desktops: detached.desktops });
}

/**
 * Unsplit: a split member becomes its own tab right after the one it left, on
 * the same desktop. A window that already has a tab to itself stays put.
 */
export function moveLayoutWindowToNewTab(
  document: WindowManagerLayoutDocument,
  windowId: string,
  mintId: LayoutNodeIdFactory
): WindowManagerLayoutDocument {
  const location = locateLayoutWindow(document, windowId);
  if (location === null || location.tab.root.kind === "leaf") return document;
  const detached = detachLayoutWindow(document, windowId);
  if (detached === null) return document;
  const tab: WindowManagerLayoutTab = {
    id: mintId("tab"),
    pinned: false,
    root: { id: mintId("leaf"), kind: "leaf", windowId },
  };
  const desktops = detached.desktops.map(desktop => {
    if (desktop.id !== detached.desktopId) return desktop;
    const tabs = [...desktop.tabs];
    tabs.splice(detached.index + 1, 0, tab);
    return { ...desktop, tabs };
  });
  return normalizeLayoutDocument({ ...document, desktops });
}

/**
 * Split: moves `windowId` beside `targetWindowId`, into the target's tab. The
 * tab it came from closes if it held nothing else.
 */
export function moveLayoutWindowBeside(
  document: WindowManagerLayoutDocument,
  windowId: string,
  targetWindowId: string,
  edge: WindowManagerLayoutEdge,
  mintId: LayoutNodeIdFactory
): WindowManagerLayoutDocument {
  if (windowId === targetWindowId) return document;
  const detached = detachLayoutWindow(document, windowId);
  if (detached === null) return document;
  const staged = { ...document, desktops: detached.desktops };
  const target = locateLayoutWindow(staged, targetWindowId);
  if (target === null) return document;
  const root: WindowManagerLayoutNode = insertBesideLeaf(
    target.tab.root,
    targetWindowId,
    windowId,
    edge,
    mintId
  );
  const desktops = staged.desktops.map(desktop =>
    desktop.id !== target.desktop.id
      ? desktop
      : {
          ...desktop,
          tabs: desktop.tabs.map(tab => (tab.id === target.tab.id ? { ...tab, root } : tab)),
        }
  );
  return normalizeLayoutDocument({ ...document, desktops });
}

/** Assign: the two windows trade places, wherever each is shown. */
export function swapLayoutWindowPlaces(
  document: WindowManagerLayoutDocument,
  first: string,
  second: string
): WindowManagerLayoutDocument {
  if (first === second) return document;
  if (locateLayoutWindow(document, first) === null) return document;
  if (locateLayoutWindow(document, second) === null) return document;
  const desktops = document.desktops.map(desktop => ({
    ...desktop,
    tabs: desktop.tabs.map(tab => ({ ...tab, root: swapLayoutWindows(tab.root, first, second) })),
  }));
  return normalizeLayoutDocument({ ...document, desktops });
}

/** Replaces one tab's tree, as the split inspector and the seam drag do. */
export function replaceLayoutTabRoot(
  document: WindowManagerLayoutDocument,
  desktopId: string,
  tabId: string,
  root: WindowManagerLayoutNode
): WindowManagerLayoutDocument {
  return updateDesktop(document, desktopId, desktop => {
    const tab = desktop.tabs.find(candidate => candidate.id === tabId);
    if (!tab || tab.root === root) return null;
    return {
      ...desktop,
      tabs: desktop.tabs.map(candidate =>
        candidate.id === tabId ? { ...candidate, root } : candidate
      ),
    };
  });
}

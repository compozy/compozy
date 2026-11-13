import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutGroup,
  WindowManagerLayoutNode,
  WindowManagerLayoutState,
} from "./window-manager-layout-types";
import type { WindowManagerSnapshot } from "@/systems/os";

type SnapshotDesktop = WindowManagerSnapshot["desktops"][number];
type SnapshotLayoutNode = SnapshotDesktop["tabs"][number]["root"];

const FULL_FRAME = { x: 0, y: 0, w: 1, h: 1 };
const DEFAULT_FLOATING_RECT = { x: 0.1, y: 0.1, w: 0.8, h: 0.8 };

function projectNode(node: SnapshotLayoutNode): WindowManagerLayoutNode {
  if (node.kind === "leaf") return { id: node.id, kind: node.kind, windowId: node.windowId };
  return {
    id: node.id,
    kind: node.kind,
    axis: node.axis,
    children: node.children.map(projectNode),
    weights: [...node.weights],
  };
}

/**
 * Bridges the v6 tab model onto the v5 groups the editor still edits (S7 replaces this): the
 * single-window tabs become one full-frame stack in bar order, and every split tab becomes its
 * own full-frame group.
 */
function projectGroups(desktop: SnapshotDesktop): WindowManagerLayoutGroup[] {
  const groups: WindowManagerLayoutGroup[] = [];
  const stacked: string[] = [];
  for (const tab of desktop.tabs) {
    if (tab.root.kind === "leaf") stacked.push(tab.root.windowId);
    else groups.push({ id: tab.id, frame: { ...FULL_FRAME }, root: projectNode(tab.root) });
  }
  const first = desktop.tabs[0];
  if (first === undefined || stacked.length === 0) return groups;
  const active = desktop.tabs.find(tab => tab.id === desktop.activeTabId)?.root;
  const activeId = active?.kind === "leaf" ? active.windowId : (stacked[0] ?? "");
  const root: WindowManagerLayoutNode =
    stacked.length === 1
      ? { id: `${first.id}:leaf`, kind: "leaf", windowId: stacked[0] ?? "" }
      : { id: `${first.id}:stack`, kind: "stack", windowIds: stacked, activeId };
  return [{ id: first.id, frame: { ...FULL_FRAME }, root }, ...groups];
}

function pinnedWindowIds(snapshot: WindowManagerSnapshot): Set<string> {
  const pinned = new Set<string>();
  for (const desktop of snapshot.desktops) {
    for (const tab of desktop.tabs) {
      if (!tab.pinned) continue;
      const stack = [tab.root];
      for (let node = stack.pop(); node !== undefined; node = stack.pop()) {
        if (node.kind === "leaf") pinned.add(node.windowId);
        else stack.push(...node.children);
      }
    }
  }
  return pinned;
}

function cloneSearch(search: Readonly<Record<string, unknown>>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(search).map(([key, value]) => [key, structuredClone(value)])
  );
}

function projectDocument(snapshot: WindowManagerSnapshot): WindowManagerLayoutDocument {
  const pinned = pinnedWindowIds(snapshot);
  return {
    version: 5,
    workspaceId: snapshot.workspaceId,
    desktops: snapshot.desktops.map(desktop => ({
      id: desktop.id,
      name: desktop.name,
      order: desktop.order,
      groups: projectGroups(desktop),
      floating: [],
      floatingStacks: [],
    })),
    windows: Object.fromEntries(
      Object.entries(snapshot.windows).map(([id, window]) => [
        id,
        {
          id: window.id,
          app: window.app,
          instanceKey: window.instanceKey,
          route: { pathname: window.route.pathname, search: cloneSearch(window.route.search) },
          navStack: window.navStack.map(route => ({
            pathname: route.pathname,
            search: cloneSearch(route.search),
          })),
          pinned: pinned.has(window.id),
          placement: "tiled",
          desktopId: window.desktopId,
          floatingRect: { ...DEFAULT_FLOATING_RECT },
          minimized: false,
          zoomed: false,
          returnAnchor: null,
        },
      ])
    ),
    overrides: Object.fromEntries(
      Object.entries(snapshot.overrides).map(([key, value]) => [key, structuredClone(value)])
    ),
  };
}

/** Mutable editor projection derived atomically from one daemon snapshot. */
export function windowManagerSnapshotToLayoutState(
  snapshot: WindowManagerSnapshot
): WindowManagerLayoutState {
  return {
    revision: snapshot.revision,
    document: projectDocument(snapshot),
  };
}

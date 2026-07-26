import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerLayoutGroup,
  WindowManagerLayoutNode,
} from "./window-manager-layout-types";

export function layoutNodeWindowIds(node: WindowManagerLayoutNode): string[] {
  if (node.kind === "leaf") return [node.windowId];
  if (node.kind === "stack") return [...node.windowIds];
  return node.children.flatMap(layoutNodeWindowIds);
}

export function desktopWindowIds(
  document: WindowManagerLayoutDocument,
  desktopId: string
): string[] {
  return Object.values(document.windows).flatMap(window =>
    window.desktopId === desktopId ? [window.id] : []
  );
}

export interface LayoutNodeLocation {
  node: WindowManagerLayoutNode;
  group: WindowManagerLayoutGroup;
}

export function findLayoutNode(
  desktop: WindowManagerLayoutDesktop,
  nodeId: string
): LayoutNodeLocation | null {
  for (const group of desktop.groups) {
    const node = findInSubtree(group.root, nodeId);
    if (node) return { node, group };
  }
  return null;
}

function findInSubtree(
  node: WindowManagerLayoutNode,
  nodeId: string
): WindowManagerLayoutNode | null {
  if (node.id === nodeId) return node;
  if (node.kind !== "split") return null;
  for (const child of node.children) {
    const hit = findInSubtree(child, nodeId);
    if (hit) return hit;
  }
  return null;
}

/** Immutable node swap: returns a new desktop, leaving untouched branches shared. */
export function replaceLayoutNode(
  desktop: WindowManagerLayoutDesktop,
  nodeId: string,
  next: WindowManagerLayoutNode
): WindowManagerLayoutDesktop {
  return {
    ...desktop,
    groups: desktop.groups.map(group => {
      const root = replaceInSubtree(group.root, nodeId, next);
      return root === group.root ? group : { ...group, root };
    }),
  };
}

function replaceInSubtree(
  node: WindowManagerLayoutNode,
  nodeId: string,
  next: WindowManagerLayoutNode
): WindowManagerLayoutNode {
  if (node.id === nodeId) return next;
  if (node.kind !== "split") return node;
  let changed = false;
  const children = node.children.map(child => {
    const replaced = replaceInSubtree(child, nodeId, next);
    if (replaced !== child) changed = true;
    return replaced;
  });
  return changed ? { ...node, children } : node;
}

/**
 * Which node currently shows each window. The daemon rejects a document that
 * places one window twice (`topology.window_membership`), so the window picker
 * reads this to disable the ones already spoken for.
 */
export function layoutWindowOwners(desktop: WindowManagerLayoutDesktop): Map<string, string> {
  const owners = new Map<string, string>();
  for (const group of desktop.groups) {
    collectOwners(group.root, owners);
  }
  return owners;
}

function collectOwners(node: WindowManagerLayoutNode, owners: Map<string, string>): void {
  if (node.kind === "split") {
    for (const child of node.children) collectOwners(child, owners);
    return;
  }
  for (const windowId of layoutNodeWindowIds(node)) owners.set(windowId, node.id);
}

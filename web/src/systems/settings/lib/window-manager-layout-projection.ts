import type { WindowManagerSnapshot } from "@/systems/os";

import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutNode,
  WindowManagerLayoutState,
} from "./window-manager-layout-types";

type SnapshotLayoutNode = WindowManagerSnapshot["desktops"][number]["groups"][number]["root"];

function projectNode(node: SnapshotLayoutNode): WindowManagerLayoutNode {
  switch (node.kind) {
    case "leaf":
      return { id: node.id, kind: node.kind, windowId: node.windowId };
    case "stack":
      return {
        id: node.id,
        kind: node.kind,
        windowIds: [...node.windowIds],
        activeId: node.activeId,
      };
    case "split":
      return {
        id: node.id,
        kind: node.kind,
        axis: node.axis,
        children: node.children.map(projectNode),
        weights: [...node.weights],
      };
  }
}

function projectDocument(snapshot: WindowManagerSnapshot): WindowManagerLayoutDocument {
  return {
    version: snapshot.version,
    workspaceId: snapshot.workspaceId,
    desktops: snapshot.desktops.map(desktop => ({
      id: desktop.id,
      name: desktop.name,
      order: desktop.order,
      purpose: desktop.purpose,
      focusOwner: desktop.focusOwner,
      groups: desktop.groups.map(group => ({
        id: group.id,
        frame: { ...group.frame },
        root: projectNode(group.root),
      })),
      floating: [...desktop.floating],
    })),
    windows: Object.fromEntries(
      Object.entries(snapshot.windows).map(([id, window]) => [
        id,
        {
          id: window.id,
          app: window.app,
          instanceKey: window.instanceKey,
          route: {
            pathname: window.route.pathname,
            search: Object.fromEntries(
              Object.entries(window.route.search).map(([key, value]) => [
                key,
                structuredClone(value),
              ])
            ),
          },
          placement: window.placement,
          desktopId: window.desktopId,
          floatingRect: { ...window.floatingRect },
          minimized: window.minimized,
          returnAnchor: window.returnAnchor === null ? null : structuredClone(window.returnAnchor),
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

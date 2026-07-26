import type { LayoutDesktop, LayoutGroup, LayoutNode } from "@/systems/os";

import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutGroup,
  WindowManagerLayoutNode,
} from "./window-manager-layout-types";

/**
 * The settings document and the runtime topology are the same shape; they differ
 * only in that the runtime's arrays are `readonly`. Going down (settings →
 * runtime) is a plain assignment the compiler accepts. Coming back needs this,
 * because a runtime function like `applySeamPreviewToDesktop` returns readonly
 * arrays the draft must stay free to replace.
 *
 * The copy is shallow-per-level and only touches the branches the caller already
 * rebuilt, so it costs nothing measurable inside a drag.
 */
export function toDraftDesktop(desktop: LayoutDesktop): WindowManagerLayoutDesktop {
  return {
    id: desktop.id,
    name: desktop.name,
    order: desktop.order,
    purpose: desktop.purpose,
    focusOwner: desktop.focusOwner,
    groups: desktop.groups.map(toDraftGroup),
    floating: [...desktop.floating],
  };
}

function toDraftGroup(group: LayoutGroup): WindowManagerLayoutGroup {
  return { id: group.id, frame: { ...group.frame }, root: toDraftNode(group.root) };
}

function toDraftNode(node: LayoutNode): WindowManagerLayoutNode {
  if (node.kind === "leaf") return { id: node.id, kind: "leaf", windowId: node.windowId };
  if (node.kind === "stack") {
    return {
      id: node.id,
      kind: "stack",
      windowIds: [...node.windowIds],
      activeId: node.activeId,
    };
  }
  return {
    id: node.id,
    kind: "split",
    axis: node.axis,
    children: node.children.map(toDraftNode),
    weights: [...node.weights],
  };
}

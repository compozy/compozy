import {
  assignTab,
  desktopById,
  detachWindow,
  equalWeights,
  findTabOf,
  generateId,
  insertTab,
  leaf,
  MockCommandError,
  nodeWindowIds,
  repairActiveTab,
  type MockNode,
  type MockReduction,
  type MockSnapshot,
  type MockTab,
} from "./window-manager-mock-reducer";

/**
 * `window.move` as `internal/windowmanager/reducer_window_move.go` reduces it: an edge placement
 * splits beside the target window (`insertRelativeInNode`) or at the target tab's root edge
 * (`splitAtRoot`); `tab` makes the window its own tab right after the target's tab, right after
 * its source tab on the same desktop, or at the end of another desktop's bar.
 */
const EDGE_AXIS: Record<string, { axis: "horizontal" | "vertical"; before: boolean }> = {
  left: { axis: "horizontal", before: true },
  right: { axis: "horizontal", before: false },
  top: { axis: "vertical", before: true },
  bottom: { axis: "vertical", before: false },
};

type Edge = (typeof EDGE_AXIS)[string];

function pairSplit(node: MockNode, incoming: MockNode, edge: Edge): MockNode {
  return {
    id: generateId("node"),
    kind: "split",
    axis: edge.axis,
    weights: [0.5, 0.5],
    children: edge.before ? [incoming, node] : [node, incoming],
  };
}

/** Beside the target leaf: a parent split on the same axis grows with equal shares, else it nests. */
function insertBesideWindow(
  node: MockNode,
  targetWindowId: string,
  incoming: MockNode,
  edge: Edge
): { node: MockNode; found: boolean } {
  if (node.kind === "leaf") {
    return node.window_id === targetWindowId
      ? { node: pairSplit(node, incoming, edge), found: true }
      : { node, found: false };
  }
  const children = node.children ?? [];
  const index = children.findIndex(
    child => child.kind === "leaf" && child.window_id === targetWindowId
  );
  if (index >= 0 && node.axis === edge.axis) {
    const at = edge.before ? index : index + 1;
    const next = [...children.slice(0, at), incoming, ...children.slice(at)];
    return { node: { ...node, children: next, weights: equalWeights(next.length) }, found: true };
  }
  for (const [childIndex, child] of children.entries()) {
    const inserted = insertBesideWindow(child, targetWindowId, incoming, edge);
    if (!inserted.found) continue;
    const next = children.map((entry, entryIndex) =>
      entryIndex === childIndex ? inserted.node : entry
    );
    return { node: { ...node, children: next }, found: true };
  }
  return { node, found: false };
}

/** A full-height or full-width member at the tab's outer edge. */
function insertAtRoot(root: MockNode, incoming: MockNode, edge: Edge): MockNode {
  if (root.kind === "split" && root.axis === edge.axis) {
    const children = edge.before
      ? [incoming, ...(root.children ?? [])]
      : [...(root.children ?? []), incoming];
    return { ...root, children, weights: equalWeights(children.length) };
  }
  return pairSplit(root, incoming, edge);
}

function resolveTargetTabId(
  snapshot: MockSnapshot,
  input: {
    windowId: string;
    destinationDesktopId: string;
    targetWindowId?: string;
    targetTabId?: string;
  }
): string | undefined {
  if (input.targetWindowId !== undefined) {
    if (input.targetTabId !== undefined || input.targetWindowId === input.windowId) {
      throw new MockCommandError("invalid-command", "Move needs one target other than itself.");
    }
    const target = snapshot.windows[input.targetWindowId];
    if (!target) {
      throw new MockCommandError(
        "window-not-found",
        `Window not found: ${input.targetWindowId}`,
        input.targetWindowId
      );
    }
    if (target.desktop_id !== input.destinationDesktopId) {
      throw new MockCommandError("invalid-command", "The target is on another desktop.");
    }
    return target.tab_id;
  }
  if (input.targetTabId !== undefined) {
    const desktop = desktopById(snapshot, input.destinationDesktopId);
    if (!desktop.tabs.some(tab => tab.id === input.targetTabId)) {
      throw new MockCommandError("invalid-command", `Unknown tab ${input.targetTabId}.`);
    }
  }
  return input.targetTabId;
}

const NO_CHANGE: MockReduction = { windowIds: [], tabIds: [], desktopIds: [] };

export function moveWindow(
  snapshot: MockSnapshot,
  input: {
    windowId: string;
    destinationDesktopId: string;
    targetWindowId?: string;
    targetTabId?: string;
    placement: string;
    moveGroup: boolean;
  }
): MockReduction {
  const source = findTabOf(snapshot, input.windowId);
  const destination = desktopById(snapshot, input.destinationDesktopId);
  const targetTabId = resolveTargetTabId(snapshot, input);
  const edge = EDGE_AXIS[input.placement];
  if (edge === undefined && input.placement !== "tab" && input.placement !== "center") {
    throw new MockCommandError("invalid-command", `Unknown placement ${input.placement}.`);
  }
  if (edge !== undefined && targetTabId === undefined) {
    throw new MockCommandError("invalid-command", "An edge placement needs a target.");
  }
  const whole = input.moveGroup || nodeWindowIds(source.tab.root).length === 1;
  if (whole && targetTabId === source.tab.id) {
    throw new MockCommandError("invalid-command", "A tab cannot move into itself.");
  }
  if (
    edge === undefined &&
    whole &&
    source.desktop.id === destination.id &&
    targetTabId === undefined
  ) {
    return NO_CHANGE;
  }
  const incoming: MockNode = whole ? source.tab.root : leaf(input.windowId);
  if (whole) {
    source.desktop.tabs.splice(source.index, 1);
    repairActiveTab(source.desktop, source.index);
  } else {
    detachWindow(snapshot, input.windowId);
  }
  let shown: MockTab;
  if (edge !== undefined) {
    const targetIndex = destination.tabs.findIndex(tab => tab.id === targetTabId);
    const target = destination.tabs[targetIndex];
    if (target === undefined) throw new MockCommandError("invalid-command", "Unknown target tab.");
    let root: MockNode;
    if (input.targetWindowId !== undefined) {
      const inserted = insertBesideWindow(target.root, input.targetWindowId, incoming, edge);
      if (!inserted.found) {
        throw new MockCommandError("window-not-found", "Target window left its tab.");
      }
      root = inserted.node;
    } else {
      root = insertAtRoot(target.root, incoming, edge);
    }
    shown = { ...target, root };
    destination.tabs[targetIndex] = shown;
  } else {
    shown = whole ? source.tab : { id: generateId("tab"), pinned: false, root: incoming };
    const sourceIndex = destination.tabs.findIndex(tab => tab.id === source.tab.id);
    const targetIndex = destination.tabs.findIndex(tab => tab.id === targetTabId);
    const index =
      targetIndex >= 0
        ? targetIndex + 1
        : destination.id === source.desktop.id
          ? sourceIndex + 1
          : destination.tabs.length;
    insertTab(destination, shown, index);
  }
  assignTab(snapshot, destination, shown);
  destination.active_tab_id = shown.id;
  return {
    windowIds: nodeWindowIds(incoming),
    tabIds: [source.tab.id, shown.id],
    desktopIds: [source.desktop.id, destination.id],
    focus: input.windowId,
    show: { desktopId: destination.id, tabId: shown.id },
  };
}

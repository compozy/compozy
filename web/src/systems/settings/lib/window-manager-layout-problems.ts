import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutNode,
  WindowManagerLayoutRoute,
} from "./window-manager-layout-types";

const WEIGHT_TOLERANCE = 1e-6;
const NAV_STACK_LIMIT = 200;

/**
 * The daemon's topology rules for a version 6 document, mirrored so the editor
 * can prove its edits keep a document valid. The daemon stays the authority —
 * Apply still validates there — and `overrides` is left to it, because only the
 * daemon knows the effective config they must yield.
 */
export function layoutDocumentProblems(document: WindowManagerLayoutDocument): string[] {
  const problems: string[] = [];
  const desktopIds = new Set<string>();
  const tabIds = new Set<string>();
  const nodeIds = new Set<string>();
  const shown = new Map<string, { desktopId: string; tabId: string }>();

  if (document.desktops.length === 0) problems.push("The layout needs at least one desktop.");
  document.desktops.forEach((desktop, index) => {
    if (desktopIds.has(desktop.id)) problems.push(`Desktop id ${desktop.id} is used twice.`);
    desktopIds.add(desktop.id);
    if (desktop.order !== index) problems.push(`Desktop ${desktop.id} is out of order.`);
    if (desktop.tabs.length === 0) problems.push(`Desktop ${desktop.id} has no tabs.`);
    if (!desktop.tabs.some(tab => tab.id === desktop.activeTabId)) {
      problems.push(`Desktop ${desktop.id} opens on a tab it does not have.`);
    }
    const firstRegular = desktop.tabs.findIndex(tab => !tab.pinned);
    if (firstRegular >= 0 && desktop.tabs.slice(firstRegular).some(tab => tab.pinned)) {
      problems.push(`Pinned tabs on desktop ${desktop.id} are not all at the start.`);
    }
    for (const tab of desktop.tabs) {
      if (tabIds.has(tab.id)) problems.push(`Tab id ${tab.id} is used twice.`);
      tabIds.add(tab.id);
      collectNodeProblems(tab.root, null, nodeIds, problems, windowId => {
        if (shown.has(windowId)) problems.push(`Window ${windowId} is shown twice.`);
        shown.set(windowId, { desktopId: desktop.id, tabId: tab.id });
      });
    }
  });

  for (const [key, window] of Object.entries(document.windows)) {
    const place = shown.get(key);
    if (!place) problems.push(`Window ${key} is not shown in any tab.`);
    if (window.id !== key) problems.push(`Window ${key} is stored under another id.`);
    if (window.app.trim() === "") problems.push(`Window ${key} has no app.`);
    if (place && (window.desktopId !== place.desktopId || window.tabId !== place.tabId)) {
      problems.push(`Window ${key} names a different desktop or tab than the one showing it.`);
    }
    for (const route of [window.route, ...window.navStack]) {
      if (!isInternalRoute(route)) problems.push(`Window ${key} has a route outside CompozyOS.`);
    }
    if (window.navStack.length > NAV_STACK_LIMIT) {
      problems.push(`Window ${key} keeps more than ${NAV_STACK_LIMIT} back steps.`);
    }
  }
  for (const windowId of shown.keys()) {
    if (!Object.hasOwn(document.windows, windowId)) {
      problems.push(`A tab shows window ${windowId}, which the layout does not define.`);
    }
  }
  return problems;
}

function collectNodeProblems(
  node: WindowManagerLayoutNode,
  parentAxis: string | null,
  nodeIds: Set<string>,
  problems: string[],
  onLeaf: (windowId: string) => void
): void {
  if (nodeIds.has(node.id)) problems.push(`Node id ${node.id} is used twice.`);
  nodeIds.add(node.id);
  if (node.kind === "leaf") {
    onLeaf(node.windowId);
    return;
  }
  if (node.children.length < 2) problems.push(`Split ${node.id} has fewer than two members.`);
  if (node.weights.length !== node.children.length) {
    problems.push(`Split ${node.id} needs one weight per member.`);
  }
  const total = node.weights.reduce((sum, weight) => sum + weight, 0);
  if (
    node.weights.some(weight => !Number.isFinite(weight) || weight <= 0) ||
    Math.abs(total - 1) > WEIGHT_TOLERANCE
  ) {
    problems.push(`Split ${node.id} has weights that are not positive or do not add up to 1.`);
  }
  if (parentAxis === node.axis) problems.push(`Split ${node.id} repeats its parent's direction.`);
  for (const child of node.children) {
    collectNodeProblems(child, node.axis, nodeIds, problems, onLeaf);
  }
}

/** An absolute in-app path: no scheme, host, query or fragment, and a JSON-object search. */
function isInternalRoute(route: WindowManagerLayoutRoute): boolean {
  const { pathname, search } = route;
  return (
    pathname.startsWith("/") &&
    !pathname.startsWith("//") &&
    !/[?#]/.test(pathname) &&
    typeof search === "object" &&
    search !== null &&
    !Array.isArray(search)
  );
}

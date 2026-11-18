import {
  windowManagerClientFixture,
  windowManagerSnapshotFixture,
  windowManagerStoryDesktopId,
  windowManagerStoryWindowId,
} from "./fixtures";
import {
  activateTab,
  arrange,
  balancedSplit,
  closeWindows,
  editWeights,
  focusWindow,
  MockCommandError,
  nodeWindowIds,
  openWindow,
  pinTab,
  reopenClosed,
  reorderTab,
  resizedSplit,
  type MockClosedEntry,
  type MockReduction,
  type MockSnapshot,
} from "./window-manager-mock-reducer";
import { moveWindow } from "./window-manager-mock-move";

type WindowManagerClient = ReturnType<typeof windowManagerClientFixture>;
type WindowManagerSnapshotFactory = (workspaceId: string) => MockSnapshot;

const defaultSnapshotFactory: WindowManagerSnapshotFactory = workspaceId => {
  const snapshot = structuredClone(windowManagerSnapshotFixture);
  snapshot.workspace_id = workspaceId;
  return snapshot;
};

export type WindowManagerMockCommandOutcome =
  | {
      kind: "applied";
      snapshot: MockSnapshot;
      client: WindowManagerClient;
      changes: { desktop_ids: string[]; window_ids: string[]; tab_ids: string[] };
    }
  | { kind: "client-not-found" }
  | { kind: "invalid-command"; message: string }
  | { kind: "window-not-found"; windowId: string };

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object";
}

function text(record: Record<string, unknown>, key: string): string | undefined {
  const value = Reflect.get(record, key);
  return typeof value === "string" && value.trim() !== "" ? value.trim() : undefined;
}

function requiredText(record: Record<string, unknown>, key: string): string {
  const value = text(record, key);
  if (value === undefined) throw new MockCommandError("invalid-command", `${key} is required.`);
  return value;
}

function numberField(record: Record<string, unknown>, key: string): number {
  const value = Reflect.get(record, key);
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new MockCommandError("invalid-command", `${key} must be a number.`);
  }
  return value;
}

function parseRoute(value: unknown): MockSnapshot["windows"][string]["route"] {
  if (!isRecord(value)) throw new MockCommandError("invalid-command", "route is required.");
  const pathname = text(value, "pathname");
  const search = Reflect.get(value, "search");
  if (pathname === undefined || !pathname.startsWith("/") || !isRecord(search)) {
    throw new MockCommandError("invalid-command", "route must name an absolute path.");
  }
  return { pathname, search: structuredClone(search) };
}

type MockNode = MockSnapshot["desktops"][number]["tabs"][number]["root"];

function swapLeaves(node: MockNode, first: string, second: string): MockNode {
  if (node.kind === "leaf") {
    if (node.window_id === first) return { ...node, window_id: second };
    if (node.window_id === second) return { ...node, window_id: first };
    return node;
  }
  return {
    ...node,
    children: (node.children ?? []).map(child => swapLeaves(child, first, second)),
  };
}

/** `window.swap`: two windows of one tab trade places; the shares stay with the places. */
function swapWindows(snapshot: MockSnapshot, first: string, second: string): MockReduction {
  for (const desktop of snapshot.desktops) {
    const index = desktop.tabs.findIndex(tab => {
      const members = nodeWindowIds(tab.root);
      return members.includes(first) && members.includes(second);
    });
    const tab = desktop.tabs[index];
    if (tab === undefined) continue;
    desktop.tabs[index] = { ...tab, root: swapLeaves(tab.root, first, second) };
    return { windowIds: [first, second], tabIds: [tab.id], desktopIds: [desktop.id] };
  }
  throw new MockCommandError("invalid-command", "Swapped windows must share a tab.");
}

function dedupe(values: readonly string[]): string[] {
  return [...new Set(values.filter(value => value !== ""))];
}

/** Storybook's in-memory daemon: one snapshot per workspace, one client view per registration. */
export class StorybookWindowManagerMockRuntime {
  private readonly snapshots = new Map<string, MockSnapshot>();
  private readonly closed = new Map<string, MockClosedEntry[]>();
  private readonly clients = new Map<string, Map<string, WindowManagerClient>>();

  constructor(private readonly snapshotFactory = defaultSnapshotFactory) {}

  reset(): void {
    this.snapshots.clear();
    this.closed.clear();
    this.clients.clear();
  }

  snapshot(workspaceId: string): MockSnapshot {
    return structuredClone(this.mutableSnapshot(workspaceId));
  }

  client(workspaceId: string, clientId: string): WindowManagerClient | null {
    return structuredClone(this.clients.get(workspaceId)?.get(clientId) ?? null);
  }

  register(workspaceId: string, clientId: string): WindowManagerClient {
    const snapshot = this.mutableSnapshot(workspaceId);
    const client = this.clientForSnapshot(clientId, workspaceId, snapshot);
    const workspaceClients = this.clients.get(workspaceId) ?? new Map();
    workspaceClients.set(clientId, client);
    this.clients.set(workspaceId, workspaceClients);
    return structuredClone(client);
  }

  execute(workspaceId: string, clientId: string, body: unknown): WindowManagerMockCommandOutcome {
    const client = this.clients.get(workspaceId)?.get(clientId);
    if (!client) return { kind: "client-not-found" };
    const snapshot = this.mutableSnapshot(workspaceId);
    // Reduce a copy so a refused command leaves the snapshot untouched.
    const draft = structuredClone(snapshot);
    let reduction: MockReduction;
    try {
      reduction = this.reduce(draft, workspaceId, client, body);
    } catch (error) {
      if (!(error instanceof MockCommandError)) throw error;
      return error.kind === "window-not-found"
        ? { kind: "window-not-found", windowId: error.windowId }
        : { kind: "invalid-command", message: error.message };
    }
    draft.revision = snapshot.revision + 1;
    draft.closed_entry_count = this.closedEntries(workspaceId).length;
    this.snapshots.set(workspaceId, draft);
    return {
      kind: "applied",
      snapshot: structuredClone(draft),
      changes: {
        desktop_ids: dedupe(reduction.desktopIds),
        window_ids: dedupe(reduction.windowIds),
        tab_ids: dedupe(reduction.tabIds),
      },
      client: this.updateClient(workspaceId, clientId, reduction, draft),
    };
  }

  private reduce(
    snapshot: MockSnapshot,
    workspaceId: string,
    client: WindowManagerClient,
    body: unknown
  ): MockReduction {
    if (!isRecord(body)) throw new MockCommandError("invalid-command", "Body must be an object.");
    const commandId = text(body, "command_id");
    const payload = Reflect.get(body, "payload");
    if (!isRecord(payload)) throw new MockCommandError("invalid-command", "payload is required.");
    switch (commandId) {
      case "window.open": {
        const window = Reflect.get(payload, "window");
        if (!isRecord(window)) throw new MockCommandError("invalid-command", "window is required.");
        const instanceKey = text(window, "instance_key");
        const afterWindowId = text(window, "stack_target_window_id");
        const desktopId = requiredText(window, "desktop_id");
        return openWindow(snapshot, {
          id: requiredText(window, "id"),
          app: requiredText(window, "app"),
          ...(instanceKey === undefined ? {} : { instanceKey }),
          route: parseRoute(Reflect.get(window, "route")),
          desktopId,
          ...(afterWindowId === undefined ? {} : { afterWindowId }),
          split: Reflect.get(window, "insert_tiled") === true,
          focusedWindowId: client.focused_window_id ?? null,
          activeTabId: client.active_tab[desktopId] ?? null,
        });
      }
      case "window.close":
        return closeWindows(
          snapshot,
          requiredText(payload, "window_id"),
          text(payload, "scope") ?? "tab",
          this.closedEntries(workspaceId)
        );
      case "window.reopen":
        return reopenClosed(snapshot, this.closedEntries(workspaceId));
      case "window.focus":
        return focusWindow(snapshot, requiredText(payload, "window_id"));
      case "window.tab.activate":
        return activateTab(snapshot, requiredText(payload, "window_id"));
      case "window.tab.reorder":
        return reorderTab(
          snapshot,
          requiredText(payload, "window_id"),
          numberField(payload, "index")
        );
      case "window.pin":
        return pinTab(
          snapshot,
          requiredText(payload, "window_id"),
          Reflect.get(payload, "pinned") === true
        );
      case "window.move": {
        const targetWindowId = text(payload, "target_window_id");
        const targetTabId = text(payload, "target_tab_id");
        return moveWindow(snapshot, {
          windowId: requiredText(payload, "window_id"),
          destinationDesktopId: requiredText(payload, "destination_desktop_id"),
          ...(targetWindowId === undefined ? {} : { targetWindowId }),
          ...(targetTabId === undefined ? {} : { targetTabId }),
          placement: text(payload, "placement") ?? "tab",
          moveGroup: Reflect.get(payload, "move_group") === true,
        });
      }
      case "window.swap":
        return swapWindows(
          snapshot,
          requiredText(payload, "first_window_id"),
          requiredText(payload, "second_window_id")
        );
      case "layout.arrange": {
        const windowIds = Reflect.get(payload, "window_ids");
        if (!Array.isArray(windowIds) || !windowIds.every(id => typeof id === "string")) {
          throw new MockCommandError("invalid-command", "window_ids must list windows.");
        }
        return arrange(
          snapshot,
          requiredText(payload, "desktop_id"),
          windowIds,
          text(payload, "arrangement") ?? "horizontal"
        );
      }
      case "layout.balance": {
        const tabId = text(payload, "tab_id");
        const splitId = text(payload, "split_id");
        return editWeights(
          snapshot,
          { ...(tabId ? { tabId } : {}), ...(splitId ? { splitId } : {}) },
          balancedSplit
        );
      }
      case "layout.resize":
        return editWeights(
          snapshot,
          { splitId: requiredText(payload, "split_id") },
          resizedSplit(numberField(payload, "boundary_index"), numberField(payload, "delta"))
        );
      default:
        throw new MockCommandError(
          "invalid-command",
          `Storybook does not simulate ${String(commandId)}.`
        );
    }
  }

  private closedEntries(workspaceId: string): MockClosedEntry[] {
    const existing = this.closed.get(workspaceId);
    if (existing) return existing;
    const created: MockClosedEntry[] = [];
    this.closed.set(workspaceId, created);
    return created;
  }

  private mutableSnapshot(workspaceId: string): MockSnapshot {
    const existing = this.snapshots.get(workspaceId);
    if (existing) return existing;
    const snapshot = structuredClone(this.snapshotFactory(workspaceId));
    this.snapshots.set(workspaceId, snapshot);
    return snapshot;
  }

  /** The acting client follows the command: focus, shown tab, and repaired active tabs. */
  private updateClient(
    workspaceId: string,
    clientId: string,
    reduction: MockReduction,
    snapshot: MockSnapshot
  ): WindowManagerClient {
    const previous =
      this.clients.get(workspaceId)?.get(clientId) ??
      windowManagerClientFixture(clientId, workspaceId);
    const live = (id: string | null | undefined) =>
      id !== null && id !== undefined && snapshot.windows[id] !== undefined;
    const focused =
      reduction.focus === undefined
        ? live(previous.focused_window_id)
          ? (previous.focused_window_id ?? null)
          : null
        : reduction.focus;
    const activeTab: Record<string, string> = {};
    for (const desktop of snapshot.desktops) {
      const own = previous.active_tab[desktop.id];
      activeTab[desktop.id] =
        own !== undefined && desktop.tabs.some(tab => tab.id === own) ? own : desktop.active_tab_id;
    }
    if (reduction.show) activeTab[reduction.show.desktopId] = reduction.show.tabId;
    const focusedWindow = focused ? snapshot.windows[focused] : undefined;
    const client: WindowManagerClient = {
      ...previous,
      presentation_revision: previous.presentation_revision + 1,
      active_desktop_id: reduction.show?.desktopId ?? previous.active_desktop_id,
      active_tab: activeTab,
      focused_window_id: focused,
      focus_order: dedupe([
        ...(focused ? [focused] : []),
        ...previous.focus_order.filter(id => live(id)),
      ]),
      palette_context: {
        ...previous.palette_context,
        window_focused: focused !== null,
        window_in_split:
          focusedWindow !== undefined &&
          snapshot.desktops.some(desktop =>
            desktop.tabs.some(tab => tab.id === focusedWindow.tab_id && tab.root.kind === "split")
          ),
      },
    };
    this.clients.get(workspaceId)?.set(clientId, client);
    return structuredClone(client);
  }

  private clientForSnapshot(
    clientId: string,
    workspaceId: string,
    snapshot: MockSnapshot
  ): WindowManagerClient {
    const focusedWindowId =
      snapshot.windows[windowManagerStoryWindowId] === undefined
        ? (Object.keys(snapshot.windows)[0] ?? null)
        : windowManagerStoryWindowId;
    const focused = focusedWindowId ? snapshot.windows[focusedWindowId] : undefined;
    const activeDesktopId =
      focused?.desktop_id ?? snapshot.desktops[0]?.id ?? windowManagerStoryDesktopId;
    const activeTab: Record<string, string> = {};
    for (const desktop of snapshot.desktops) activeTab[desktop.id] = desktop.active_tab_id;
    if (focused) activeTab[focused.desktop_id] = focused.tab_id;
    return {
      ...windowManagerClientFixture(clientId, workspaceId),
      active_desktop_id: activeDesktopId,
      active_tab: activeTab,
      focused_window_id: focusedWindowId,
      focus_order: focusedWindowId ? [focusedWindowId] : [],
    };
  }
}

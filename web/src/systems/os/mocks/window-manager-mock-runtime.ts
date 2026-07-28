import {
  windowManagerClientFixture,
  windowManagerSnapshotFixture,
  windowManagerStoryDesktopId,
  windowManagerStoryWindowId,
} from "./fixtures";

type WindowManagerSnapshot = typeof windowManagerSnapshotFixture;
type WindowManagerClient = ReturnType<typeof windowManagerClientFixture>;

interface OpenWindowSpec {
  id: string;
  app: string;
  instanceKey?: string;
  route: {
    pathname: string;
    search: Record<string, unknown>;
  };
  desktopId: string;
  floatingRect: {
    x: number;
    y: number;
    width: number;
    height: number;
  };
  insertTiled: boolean;
}

export type WindowManagerMockCommandOutcome =
  | {
      kind: "applied";
      snapshot: WindowManagerSnapshot;
      client: WindowManagerClient;
      changes: {
        desktop_ids?: string[];
        window_ids: string[];
      };
    }
  | { kind: "client-not-found" }
  | { kind: "invalid-command"; message: string }
  | { kind: "window-not-found"; windowId: string };

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object";
}

function nonEmptyString(value: unknown): string | null {
  return typeof value === "string" && value.trim() !== "" ? value.trim() : null;
}

function finiteNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function parseOpenWindowSpec(value: unknown): OpenWindowSpec | null {
  if (!isRecord(value)) return null;
  const payload = Reflect.get(value, "payload");
  if (!isRecord(payload)) return null;
  const window = Reflect.get(payload, "window");
  if (!isRecord(window)) return null;

  const id = nonEmptyString(Reflect.get(window, "id"));
  const app = nonEmptyString(Reflect.get(window, "app"));
  const desktopId = nonEmptyString(Reflect.get(window, "desktop_id"));
  const route = Reflect.get(window, "route");
  const floatingRect = Reflect.get(window, "floating_rect");
  const insertTiled = Reflect.get(window, "insert_tiled");
  if (
    id === null ||
    app === null ||
    desktopId === null ||
    !isRecord(route) ||
    !isRecord(floatingRect) ||
    typeof insertTiled !== "boolean"
  ) {
    return null;
  }

  const pathname = nonEmptyString(Reflect.get(route, "pathname"));
  const search = Reflect.get(route, "search");
  const x = finiteNumber(Reflect.get(floatingRect, "x"));
  const y = finiteNumber(Reflect.get(floatingRect, "y"));
  const width = finiteNumber(Reflect.get(floatingRect, "width"));
  const height = finiteNumber(Reflect.get(floatingRect, "height"));
  if (
    pathname === null ||
    !pathname.startsWith("/") ||
    !isRecord(search) ||
    x === null ||
    y === null ||
    width === null ||
    height === null
  ) {
    return null;
  }

  const instanceKey = nonEmptyString(Reflect.get(window, "instance_key"));
  return {
    id,
    app,
    ...(instanceKey === null ? {} : { instanceKey }),
    route: { pathname, search: structuredClone(search) },
    desktopId,
    floatingRect: { x, y, width, height },
    insertTiled,
  };
}

function parseMinimizedWindowId(value: unknown): string | null {
  if (!isRecord(value)) return null;
  const payload = Reflect.get(value, "payload");
  if (!isRecord(payload) || Reflect.get(payload, "minimize") !== true) return null;
  return nonEmptyString(Reflect.get(payload, "window_id"));
}

function commandId(value: unknown): string | null {
  return isRecord(value) ? nonEmptyString(Reflect.get(value, "command_id")) : null;
}

export class StorybookWindowManagerMockRuntime {
  private readonly snapshots = new Map<string, WindowManagerSnapshot>();
  private readonly clients = new Map<string, Map<string, WindowManagerClient>>();

  reset(): void {
    this.snapshots.clear();
    this.clients.clear();
  }

  snapshot(workspaceId: string): WindowManagerSnapshot {
    return structuredClone(this.mutableSnapshot(workspaceId));
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
    if (!this.clients.get(workspaceId)?.has(clientId)) {
      return { kind: "client-not-found" };
    }

    const id = commandId(body);
    if (id === "window.open") {
      return this.openWindow(workspaceId, clientId, body);
    }
    if (id === "window.close") {
      return this.minimizeWindow(workspaceId, clientId, body);
    }
    return {
      kind: "invalid-command",
      message: `Storybook does not simulate ${String(id)}.`,
    };
  }

  private mutableSnapshot(workspaceId: string): WindowManagerSnapshot {
    const existing = this.snapshots.get(workspaceId);
    if (existing) return existing;

    const snapshot = structuredClone(windowManagerSnapshotFixture);
    snapshot.workspace_id = workspaceId;
    this.snapshots.set(workspaceId, snapshot);
    return snapshot;
  }

  private openWindow(
    workspaceId: string,
    clientId: string,
    body: unknown
  ): WindowManagerMockCommandOutcome {
    const window = parseOpenWindowSpec(body);
    if (window === null || window.insertTiled) {
      return {
        kind: "invalid-command",
        message: "Storybook requires a valid floating window.open payload.",
      };
    }

    const snapshot = this.mutableSnapshot(workspaceId);
    const desktop = snapshot.desktops.find(candidate => candidate.id === window.desktopId);
    if (!desktop || snapshot.windows[window.id]) {
      return {
        kind: "invalid-command",
        message: `Storybook cannot open ${window.id} on ${window.desktopId}.`,
      };
    }

    snapshot.windows[window.id] = {
      id: window.id,
      app: window.app,
      ...(window.instanceKey === undefined ? {} : { instance_key: window.instanceKey }),
      route: window.route,
      placement: "floating",
      desktop_id: window.desktopId,
      floating_rect: window.floatingRect,
      minimized: false,
    };
    desktop.floating.push(window.id);
    snapshot.revision += 1;

    const client = this.focusClient(workspaceId, clientId, window.id, window.desktopId, snapshot);
    return {
      kind: "applied",
      snapshot: structuredClone(snapshot),
      changes: {
        desktop_ids: [window.desktopId],
        window_ids: [window.id],
      },
      client,
    };
  }

  private minimizeWindow(
    workspaceId: string,
    clientId: string,
    body: unknown
  ): WindowManagerMockCommandOutcome {
    const windowId = parseMinimizedWindowId(body);
    if (windowId === null) {
      return {
        kind: "invalid-command",
        message: "Storybook only simulates window.close with minimize enabled.",
      };
    }

    const snapshot = this.mutableSnapshot(workspaceId);
    const window = snapshot.windows[windowId];
    if (!window) return { kind: "window-not-found", windowId };

    window.minimized = true;
    snapshot.revision += 1;
    const fallbackId =
      Object.values(snapshot.windows).find(
        candidate => !candidate.minimized && candidate.desktop_id === window.desktop_id
      )?.id ?? null;
    const client = this.focusClient(workspaceId, clientId, fallbackId, window.desktop_id, snapshot);
    return {
      kind: "applied",
      snapshot: structuredClone(snapshot),
      changes: { window_ids: [windowId] },
      client,
    };
  }

  private focusClient(
    workspaceId: string,
    clientId: string,
    focusedWindowId: string | null,
    activeDesktopId: string,
    snapshot: WindowManagerSnapshot
  ): WindowManagerClient {
    const previous =
      this.clients.get(workspaceId)?.get(clientId) ??
      windowManagerClientFixture(clientId, workspaceId);
    const focusOrder = focusedWindowId
      ? [
          focusedWindowId,
          ...previous.focus_order.filter(
            id => id !== focusedWindowId && snapshot.windows[id] !== undefined
          ),
        ]
      : [];
    const client: WindowManagerClient = {
      ...previous,
      presentation_revision: previous.presentation_revision + 1,
      active_desktop_id: activeDesktopId,
      focused_window_id: focusedWindowId,
      focus_order: focusOrder,
    };
    this.clients.get(workspaceId)?.set(clientId, client);
    return structuredClone(client);
  }

  private clientForSnapshot(
    clientId: string,
    workspaceId: string,
    snapshot: WindowManagerSnapshot
  ): WindowManagerClient {
    const focusedWindowId =
      snapshot.windows[windowManagerStoryWindowId] === undefined
        ? (Object.keys(snapshot.windows)[0] ?? null)
        : windowManagerStoryWindowId;
    const activeDesktopId =
      (focusedWindowId ? snapshot.windows[focusedWindowId]?.desktop_id : null) ??
      snapshot.desktops[0]?.id ??
      windowManagerStoryDesktopId;
    return {
      ...windowManagerClientFixture(clientId, workspaceId),
      active_desktop_id: activeDesktopId,
      focused_window_id: focusedWindowId,
      focus_order: focusedWindowId ? [focusedWindowId] : [],
    };
  }
}

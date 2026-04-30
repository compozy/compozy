import { apiBaseUrl } from "@/lib/api-client";

export const WORKSPACE_EVENT_TYPE = "workspace.event";
export const WORKSPACE_HEARTBEAT_TYPE = "workspace.heartbeat";
export const WORKSPACE_OVERFLOW_TYPE = "workspace.overflow";
export const WORKSPACE_ERROR_TYPE = "error";

const initialReconnectDelayMs = 250;
const maxReconnectDelayMs = 5000;

export type WorkspaceEventKind =
  | "run.created"
  | "run.status_changed"
  | "run.terminal"
  | "workflow.sync_completed"
  | "artifact.changed";

export interface WorkspaceEventPayload {
  seq: number;
  ts: string;
  workspace_id: string;
  workflow_id?: string;
  workflow_slug?: string;
  run_id?: string;
  mode?: string;
  status?: string;
  kind: WorkspaceEventKind;
  paths?: string[];
}

export type WorkspaceEventSignal =
  | { type: "open" }
  | { type: "event"; eventId: string | null; payload: WorkspaceEventPayload }
  | { type: "heartbeat"; eventId: string | null; payload: unknown }
  | { type: "overflow"; eventId: string | null; payload: unknown }
  | { type: "error"; error: unknown };

export interface WorkspaceEventController {
  close(): void;
}

export interface WorkspaceEventHandler {
  (signal: WorkspaceEventSignal): void;
}

export interface OpenWorkspaceEventStreamOptions {
  workspaceId: string;
  baseUrl?: string;
}

export interface WorkspaceEventStreamFactory {
  (
    options: OpenWorkspaceEventStreamOptions,
    handler: WorkspaceEventHandler
  ): WorkspaceEventController;
}

export interface WorkspaceSocketLike {
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  close(code?: number, reason?: string): void;
}

export interface WorkspaceSocketConstructor {
  new (url: string): WorkspaceSocketLike;
}

interface WorkspaceSocketEnvelope {
  type: string;
  id?: string;
  payload?: unknown;
}

export function buildWorkspaceSocketUrl(options: OpenWorkspaceEventStreamOptions): string {
  const base = new URL(options.baseUrl ?? apiBaseUrl);
  base.protocol = base.protocol === "https:" ? "wss:" : "ws:";
  base.pathname = `/api/workspaces/${encodeURIComponent(options.workspaceId)}/ws`;
  base.search = "";
  base.hash = "";
  return base.toString();
}

function parseEventData(raw: unknown): unknown {
  if (typeof raw !== "string") {
    return raw;
  }
  const trimmed = raw.trim();
  if (trimmed.length === 0) {
    return null;
  }
  try {
    return JSON.parse(trimmed);
  } catch {
    return raw;
  }
}

function isWorkspaceEventPayload(payload: unknown): payload is WorkspaceEventPayload {
  if (!payload || typeof payload !== "object") {
    return false;
  }
  const kind = Reflect.get(payload, "kind");
  const workspaceId = Reflect.get(payload, "workspace_id");
  return typeof kind === "string" && typeof workspaceId === "string";
}

function isWorkspaceSocketEnvelope(payload: unknown): payload is WorkspaceSocketEnvelope {
  if (!payload || typeof payload !== "object") {
    return false;
  }
  return typeof Reflect.get(payload, "type") === "string";
}

let workspaceEventSocketFactoryOverride: WorkspaceEventStreamFactory | null = null;

export function setWorkspaceEventStreamFactoryOverrideForTests(
  factory: WorkspaceEventStreamFactory | null
): void {
  workspaceEventSocketFactoryOverride = factory;
}

export function createWorkspaceEventSocketFactory(
  socketConstructor: WorkspaceSocketConstructor
): WorkspaceEventStreamFactory {
  return (options, handler) => {
    let closed = false;
    let socket: WorkspaceSocketLike | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let reconnectAttempt = 0;

    const emit = (signal: WorkspaceEventSignal) => {
      try {
        handler(signal);
      } catch {
        // Event handlers must not break socket lifecycle management.
      }
    };

    const clearReconnectTimer = () => {
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    };

    const scheduleReconnect = () => {
      if (closed || reconnectTimer) {
        return;
      }
      const delay = Math.min(initialReconnectDelayMs * 2 ** reconnectAttempt, maxReconnectDelayMs);
      reconnectAttempt += 1;
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connect();
      }, delay);
    };

    const handleEnvelope = (envelope: WorkspaceSocketEnvelope) => {
      reconnectAttempt = 0;
      switch (envelope.type) {
        case WORKSPACE_EVENT_TYPE:
          if (isWorkspaceEventPayload(envelope.payload)) {
            emit({ type: "event", eventId: envelope.id ?? null, payload: envelope.payload });
          }
          return;
        case WORKSPACE_HEARTBEAT_TYPE:
          emit({ type: "heartbeat", eventId: envelope.id ?? null, payload: envelope.payload });
          return;
        case WORKSPACE_OVERFLOW_TYPE:
          emit({ type: "overflow", eventId: envelope.id ?? null, payload: envelope.payload });
          return;
        case WORKSPACE_ERROR_TYPE:
          emit({ type: "error", error: envelope.payload });
          return;
      }
    };

    function connect() {
      if (closed) {
        return;
      }
      try {
        socket = new socketConstructor(buildWorkspaceSocketUrl(options));
      } catch (error) {
        emit({ type: "error", error });
        scheduleReconnect();
        return;
      }
      socket.onopen = () => {
        reconnectAttempt = 0;
        emit({ type: "open" });
      };
      socket.onmessage = event => {
        const envelope = parseEventData(event.data);
        if (isWorkspaceSocketEnvelope(envelope)) {
          handleEnvelope(envelope);
        }
      };
      socket.onerror = event => {
        emit({ type: "error", error: event });
      };
      socket.onclose = () => {
        socket = null;
        scheduleReconnect();
      };
    }

    connect();

    return {
      close() {
        closed = true;
        clearReconnectTimer();
        const activeSocket = socket;
        socket = null;
        activeSocket?.close(1000, "workspace event stream closed");
      },
    };
  };
}

export const defaultWorkspaceEventStreamFactory: WorkspaceEventStreamFactory = (
  options,
  handler
) => {
  if (workspaceEventSocketFactoryOverride) {
    return workspaceEventSocketFactoryOverride(options, handler);
  }
  if (typeof WebSocket === "undefined") {
    handler({ type: "error", error: new Error("WebSocket is not supported") });
    return { close: () => {} };
  }
  return createWorkspaceEventSocketFactory(WebSocket)(options, handler);
};

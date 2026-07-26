export const SESSION_DEBUG_EVENTS = {
  gapRecoveryTriggered: "gap_recovery_triggered",
  sseClose: "sse_close",
  sseOpen: "sse_open",
  sseReconnect: "sse_reconnect",
  threadEmptyWhileActive: "thread_empty_while_active",
  transcriptApplyFailed: "transcript_apply_failed",
  transcriptFetchFailed: "transcript_fetch_failed",
} as const;

export type SessionDebugEventName =
  (typeof SESSION_DEBUG_EVENTS)[keyof typeof SESSION_DEBUG_EVENTS];

export type SessionDebugEventFields = Record<string, boolean | number | string | null | undefined>;

export interface SessionDebugEvent extends SessionDebugEventFields {
  count: number;
  event: SessionDebugEventName;
  timestamp: string;
}

declare global {
  interface Window {
    __AGH_SESSION_DEBUG_COUNTERS__?: Partial<Record<SessionDebugEventName, number>>;
    __AGH_SESSION_DEBUG_EVENTS__?: SessionDebugEvent[];
  }
}

const debugCounters: Partial<Record<SessionDebugEventName, number>> = {};
const debugEvents: SessionDebugEvent[] = [];
const MAX_DEBUG_EVENTS = 200;

function publishDebugState() {
  if (typeof window === "undefined") {
    return;
  }
  window.__AGH_SESSION_DEBUG_COUNTERS__ = debugCounters;
  window.__AGH_SESSION_DEBUG_EVENTS__ = debugEvents;
}

export function formatSessionDebugError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === "string") {
    return error;
  }
  return String(error);
}

export function recordSessionDebugEvent(
  event: SessionDebugEventName,
  fields: SessionDebugEventFields = {}
): SessionDebugEvent {
  const count = (debugCounters[event] ?? 0) + 1;
  debugCounters[event] = count;

  const entry: SessionDebugEvent = {
    ...fields,
    count,
    event,
    timestamp: new Date().toISOString(),
  };
  debugEvents.push(entry);
  if (debugEvents.length > MAX_DEBUG_EVENTS) {
    debugEvents.splice(0, debugEvents.length - MAX_DEBUG_EVENTS);
  }
  publishDebugState();

  if (import.meta.env.DEV && import.meta.env.MODE !== "test") {
    console.debug("[agh:session]", entry);
  }

  return entry;
}

export function getSessionDebugCounters(): Partial<Record<SessionDebugEventName, number>> {
  return { ...debugCounters };
}

export function getSessionDebugEvents(): SessionDebugEvent[] {
  return [...debugEvents];
}

export function resetSessionDebugTelemetry() {
  for (const key of Object.keys(debugCounters) as SessionDebugEventName[]) {
    delete debugCounters[key];
  }
  debugEvents.length = 0;
  publishDebugState();
}

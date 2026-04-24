export interface RunFeedEvent {
  id: string;
  seq: number | null;
  kind: string;
  runId: string | null;
  timestamp: string | null;
  payload: unknown;
  receivedAt: number;
}

const DEFAULT_CAPACITY = 500;
const TERMINAL_KINDS = new Set(["run.completed", "run.failed", "run.cancelled", "run.crashed"]);

type Listener = () => void;

function parsePayload(payload: unknown): Record<string, unknown> | null {
  if (!payload || typeof payload !== "object") {
    return null;
  }
  return payload as Record<string, unknown>;
}

function readString(source: Record<string, unknown> | null, key: string): string | null {
  if (!source) {
    return null;
  }
  const value = source[key];
  return typeof value === "string" && value.length > 0 ? value : null;
}

function readNumber(source: Record<string, unknown> | null, key: string): number | null {
  if (!source) {
    return null;
  }
  const value = source[key];
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

export function normalizeFeedEvent(
  eventId: string | null,
  raw: unknown,
  now: number = Date.now()
): RunFeedEvent | null {
  const envelope = parsePayload(raw);
  if (!envelope) {
    return null;
  }
  const kind = readString(envelope, "kind");
  if (!kind) {
    return null;
  }
  const seq = readNumber(envelope, "seq");
  const runId = readString(envelope, "run_id");
  const timestamp = readString(envelope, "ts");
  const payload = envelope["payload"] ?? null;
  const stableId =
    eventId ?? (seq !== null && runId ? `${runId}:${seq}` : `${kind}:${now}:${Math.random()}`);
  return {
    id: stableId,
    seq,
    kind,
    runId,
    timestamp,
    payload,
    receivedAt: now,
  };
}

export function isTerminalKind(kind: string): boolean {
  return TERMINAL_KINDS.has(kind);
}

export interface RunEventStore {
  append: (eventId: string | null, raw: unknown) => RunFeedEvent | null;
  reset: () => void;
  subscribe: (listener: Listener) => () => void;
  getSnapshot: () => readonly RunFeedEvent[];
  getServerSnapshot: () => readonly RunFeedEvent[];
}

export function createRunEventStore(capacity: number = DEFAULT_CAPACITY): RunEventStore {
  let events: RunFeedEvent[] = [];
  const listeners = new Set<Listener>();
  const emit = () => {
    for (const listener of listeners) {
      listener();
    }
  };
  return {
    append(eventId, raw) {
      const normalized = normalizeFeedEvent(eventId, raw);
      if (!normalized) {
        return null;
      }
      const next = events.concat(normalized);
      events = next.length > capacity ? next.slice(next.length - capacity) : next;
      emit();
      return normalized;
    },
    reset() {
      if (events.length === 0) {
        return;
      }
      events = [];
      emit();
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    getSnapshot() {
      return events;
    },
    getServerSnapshot() {
      return events;
    },
  };
}

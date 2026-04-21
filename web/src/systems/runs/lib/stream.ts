export const RUN_EVENT_NAME = "run.event";
export const RUN_HEARTBEAT_NAME = "run.heartbeat";
export const RUN_OVERFLOW_NAME = "run.overflow";
export const RUN_ERROR_NAME = "error";

export type RunStreamSignal =
  | { type: "open" }
  | { type: "event"; eventId: string | null; payload: unknown }
  | { type: "heartbeat"; eventId: string | null; payload: unknown }
  | { type: "overflow"; eventId: string | null; payload: unknown }
  | { type: "error"; error: unknown };

export interface RunStreamController {
  close(): void;
}

export interface RunStreamHandler {
  (signal: RunStreamSignal): void;
}

export interface OpenRunStreamOptions {
  runId: string;
  baseUrl: string;
  lastEventId?: string | null;
}

export interface RunStreamFactory {
  (options: OpenRunStreamOptions, handler: RunStreamHandler): RunStreamController;
}

function appendCursor(url: string, cursor: string | null | undefined): string {
  if (!cursor) {
    return url;
  }
  const separator = url.includes("?") ? "&" : "?";
  return `${url}${separator}cursor=${encodeURIComponent(cursor)}`;
}

export function buildRunStreamUrl(options: OpenRunStreamOptions): string {
  const base = options.baseUrl.replace(/\/$/, "");
  const path = `/api/runs/${encodeURIComponent(options.runId)}/stream`;
  return appendCursor(`${base}${path}`, options.lastEventId);
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

let runStreamFactoryOverride: RunStreamFactory | null = null;

export function setRunStreamFactoryOverrideForTests(factory: RunStreamFactory | null): void {
  runStreamFactoryOverride = factory;
}

export const defaultRunStreamFactory: RunStreamFactory = (options, handler) => {
  if (runStreamFactoryOverride) {
    return runStreamFactoryOverride(options, handler);
  }
  if (typeof EventSource === "undefined") {
    handler({ type: "error", error: new Error("EventSource is not supported") });
    return { close: () => {} };
  }
  const url = buildRunStreamUrl(options);
  const source = new EventSource(url, { withCredentials: true });
  const emit = (signal: RunStreamSignal) => {
    try {
      handler(signal);
    } catch {
      // swallow — the handler must not break the stream loop
    }
  };
  source.addEventListener("open", () => emit({ type: "open" }));
  source.addEventListener(RUN_EVENT_NAME, (raw: Event) => {
    const message = raw as MessageEvent;
    emit({
      type: "event",
      eventId: message.lastEventId || null,
      payload: parseEventData(message.data),
    });
  });
  source.addEventListener(RUN_HEARTBEAT_NAME, (raw: Event) => {
    const message = raw as MessageEvent;
    emit({
      type: "heartbeat",
      eventId: message.lastEventId || null,
      payload: parseEventData(message.data),
    });
  });
  source.addEventListener(RUN_OVERFLOW_NAME, (raw: Event) => {
    const message = raw as MessageEvent;
    emit({
      type: "overflow",
      eventId: message.lastEventId || null,
      payload: parseEventData(message.data),
    });
  });
  source.addEventListener(RUN_ERROR_NAME, (raw: Event) => {
    const message = raw as MessageEvent;
    emit({ type: "error", error: parseEventData(message.data) });
  });
  source.onerror = () => emit({ type: "error", error: new Error("run stream disconnected") });
  return {
    close: () => source.close(),
  };
};

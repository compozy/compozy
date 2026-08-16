export type BootstrapPhase = "resolve" | "attach" | "provision" | "start" | "ready" | "failed";
export type BootstrapStatus = "started" | "completed" | "retrying" | "failed";

export interface BootstrapEvent {
  readonly type: "bootstrap";
  readonly phase: BootstrapPhase;
  readonly status: BootstrapStatus;
  readonly message: string;
  readonly resolution?: "attach" | "start" | "provision";
  readonly attempt?: number;
  readonly backoff_ms?: number;
  readonly daemon?: { readonly origin: string; readonly version: string; readonly pid: number };
}

const PHASES = new Set<BootstrapPhase>([
  "resolve",
  "attach",
  "provision",
  "start",
  "ready",
  "failed",
]);
const STATUSES = new Set<BootstrapStatus>(["started", "completed", "retrying", "failed"]);
const RESOLUTIONS = new Set(["attach", "start", "provision"]);

function validDaemonOrigin(value: unknown): value is string {
  if (typeof value !== "string") return false;
  try {
    const origin = new URL(value);
    return (
      (origin.protocol === "http:" || origin.protocol === "https:") &&
      (origin.hostname === "127.0.0.1" ||
        origin.hostname === "localhost" ||
        origin.hostname === "[::1]") &&
      origin.username === "" &&
      origin.password === ""
    );
  } catch {
    return false;
  }
}

export function parseBootstrapEvent(line: string): BootstrapEvent | null {
  let value: unknown;
  try {
    value = JSON.parse(line);
  } catch {
    return null;
  }
  if (!value || typeof value !== "object") return null;
  const record = value as Record<string, unknown>;
  if (
    record.type !== "bootstrap" ||
    typeof record.phase !== "string" ||
    !PHASES.has(record.phase as BootstrapPhase) ||
    typeof record.status !== "string" ||
    !STATUSES.has(record.status as BootstrapStatus) ||
    typeof record.message !== "string"
  ) {
    return null;
  }
  if (
    record.resolution !== undefined &&
    (typeof record.resolution !== "string" || !RESOLUTIONS.has(record.resolution))
  ) {
    return null;
  }
  if (record.daemon !== undefined) {
    if (!record.daemon || typeof record.daemon !== "object") return null;
    const daemon = record.daemon as Record<string, unknown>;
    if (
      !validDaemonOrigin(daemon.origin) ||
      typeof daemon.version !== "string" ||
      typeof daemon.pid !== "number" ||
      !Number.isSafeInteger(daemon.pid) ||
      daemon.pid < 1
    ) {
      return null;
    }
  }
  return value as BootstrapEvent;
}

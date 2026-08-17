const BOOTSTRAP_PHASE_VALUES = [
  "resolve",
  "attach",
  "provision",
  "start",
  "ready",
  "failed",
] as const;
export type BootstrapPhase = (typeof BOOTSTRAP_PHASE_VALUES)[number];
const BOOTSTRAP_STATUS_VALUES = ["started", "completed", "retrying", "failed"] as const;
export type BootstrapStatus = (typeof BOOTSTRAP_STATUS_VALUES)[number];
const BOOTSTRAP_RESOLUTION_VALUES = ["attach", "start", "provision"] as const;
export type BootstrapResolution = (typeof BOOTSTRAP_RESOLUTION_VALUES)[number];

export interface BootstrapEvent {
  readonly type: "bootstrap";
  readonly phase: BootstrapPhase;
  readonly status: BootstrapStatus;
  readonly message: string;
  readonly resolution?: BootstrapResolution;
  readonly attempt?: number;
  readonly backoff_ms?: number;
  readonly compatibility?: {
    readonly reason: "runtime_below_minimum" | "app_below_minimum";
    readonly runtime: string;
    readonly needed: string;
  };
  readonly daemon?: {
    readonly origin: string;
    readonly version: string;
    readonly pid: number;
    readonly owned: boolean;
  };
}

const PHASES = new Set<string>(BOOTSTRAP_PHASE_VALUES);
const STATUSES = new Set<string>(BOOTSTRAP_STATUS_VALUES);
const RESOLUTIONS = new Set<string>(BOOTSTRAP_RESOLUTION_VALUES);

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
  if (record.compatibility !== undefined) {
    if (!record.compatibility || typeof record.compatibility !== "object") return null;
    const compatibility = record.compatibility as Record<string, unknown>;
    if (
      (compatibility.reason !== "runtime_below_minimum" &&
        compatibility.reason !== "app_below_minimum") ||
      typeof compatibility.runtime !== "string" ||
      compatibility.runtime.trim() === "" ||
      typeof compatibility.needed !== "string" ||
      compatibility.needed.trim() === ""
    ) {
      return null;
    }
  }
  if (
    record.resolution !== undefined &&
    (typeof record.resolution !== "string" || !RESOLUTIONS.has(record.resolution))
  ) {
    return null;
  }
  if (
    record.attempt !== undefined &&
    (typeof record.attempt !== "number" ||
      !Number.isSafeInteger(record.attempt) ||
      record.attempt < 1 ||
      record.attempt > 5)
  ) {
    return null;
  }
  if (
    record.backoff_ms !== undefined &&
    (typeof record.backoff_ms !== "number" ||
      !Number.isSafeInteger(record.backoff_ms) ||
      record.backoff_ms < 0)
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
      daemon.pid < 1 ||
      typeof daemon.owned !== "boolean"
    ) {
      return null;
    }
  }
  return value as BootstrapEvent;
}

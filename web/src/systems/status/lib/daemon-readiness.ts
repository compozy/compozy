import type { DaemonStatusPayload } from "../types";

/**
 * The runtime's lifecycle phase and current step. Mirrors `DaemonReadinessPayload`
 * (`internal/api/contract/status.go`); runtimes that predate lifecycle phases omit it.
 */
export interface DaemonReadiness {
  phase: string;
  step?: string;
}

/** What the shell says while the runtime serves reads and still resumes sessions. */
export interface DaemonRecovery {
  /** The step in plain words, or the phase's own line for a step with no name here. */
  label: string;
}

const DEFAULT_RECOVERY_LABEL = "Resuming your sessions";

/** Plain words for the recovery steps a user can wait on; other steps use the default line. */
const RECOVERY_STEP_LABELS: Readonly<Record<string, string>> = {
  reconcileBootInstances: "Restoring your agents",
  recoverBootDelegatedSessions: "Reconnecting subagents",
  resumeBootInterruptedSessions: "Resuming interrupted sessions",
  recoverBootMonitors: "Restoring monitors",
  bootFinalize: "Checking the session list",
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** Reads `daemon.readiness` at the adapter boundary; null when the runtime does not report it. */
export function daemonReadiness(daemon: DaemonStatusPayload | undefined): DaemonReadiness | null {
  // The generated status contract gains `readiness` with the next `make codegen`; until then the
  // field is read through this narrow local shape.
  const source: unknown = daemon;
  const readiness = isRecord(source) ? source.readiness : undefined;
  if (!isRecord(readiness) || typeof readiness.phase !== "string" || readiness.phase === "") {
    return null;
  }
  const step = typeof readiness.step === "string" ? readiness.step : "";
  return step === "" ? { phase: readiness.phase } : { phase: readiness.phase, step };
}

/**
 * The recovery the shell shows, or null once the runtime is fully ready. Only the serving phase
 * counts: before it nothing answers, and after it `status` is `running` again.
 */
export function daemonRecovery(daemon: DaemonStatusPayload | undefined): DaemonRecovery | null {
  const readiness = daemonReadiness(daemon);
  if (readiness?.phase !== "serving") return null;
  const named = readiness.step ? RECOVERY_STEP_LABELS[readiness.step] : undefined;
  return { label: named ?? DEFAULT_RECOVERY_LABEL };
}

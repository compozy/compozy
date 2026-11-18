import type { BootProgress } from "../../state/app-state";

/**
 * Plain words for the runtime steps a user can wait on. The runtime reports its own step names;
 * any other step, or a newer runtime's, falls back to the phase line.
 */
const STEP_LINES: Readonly<Record<string, string>> = {
  bootSessionRepair: "Checking your sessions.",
  bootServers: "Opening connections.",
  reconcileBootInstances: "Restoring your agents.",
  recoverBootDelegatedSessions: "Reconnecting subagents.",
  resumeBootInterruptedSessions: "Resuming interrupted sessions.",
  recoverBootMonitors: "Restoring monitors.",
  bootFinalize: "Checking the session list.",
};

const PHASE_LINES: Readonly<Record<string, string>> = {
  starting: "Starting the runtime with your data.",
  serving: "Resuming your sessions.",
};

/** The line for the runtime's current lifecycle step, or null when it reports none we name. */
export function readinessLine(boot: BootProgress | undefined): string | null {
  if (!boot?.readiness) return null;
  return (boot.step ? STEP_LINES[boot.step] : undefined) ?? PHASE_LINES[boot.readiness] ?? null;
}

/** The line shown while another process's update, restart or start holds the runtime. */
export function waitLine(boot: BootProgress | undefined): string | null {
  if (boot?.wait !== "runtime_mutation") return null;
  const owner = boot.owner_pid === undefined ? "" : ` (process ${boot.owner_pid})`;
  return `Another update or restart is in progress${owner}. CompozyOS continues when it finishes.`;
}

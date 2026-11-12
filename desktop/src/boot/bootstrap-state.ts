import type { BootstrapEvent } from "../bootstrap/bootstrap-event";
import type { ShellSnapshot } from "../state/app-state";
import { publicSafeText } from "../state/public-safe-text";

/** Migration stages whose unit is a session database; every other stage counts records. */
const SESSION_MIGRATIONS = new Set(["session_history_upgrade"]);

function skewSnapshot(event: BootstrapEvent): ShellSnapshot | null {
  if (!event.compatibility) return null;
  return {
    state: "skew",
    runtime: event.compatibility.runtime,
    needed: event.compatibility.needed,
    newer: event.compatibility.reason === "app_below_minimum",
  };
}

function failureCode(event: BootstrapEvent): string {
  // The CLI reports a stalled data upgrade as a start failure and again as the provision failure.
  if (/data upgrade .* stopped progressing/iu.test(event.message)) return "runtime_upgrade_stalled";
  if (event.phase === "provision") {
    if (/permission|operation not permitted|access denied/iu.test(event.message)) {
      return "provision_permission";
    }
    if (/no space|disk full/iu.test(event.message)) return "provision_disk_space";
    return "provision_network";
  }
  if (event.phase === "attach" || event.phase === "ready") return "runtime_unhealthy";
  return "runtime_start_failed";
}

/**
 * The runtime transition this boot performs. A provisioning event names it; later events of the
 * same boot keep it, so the window can say what changed until the product opens.
 */
function versions(
  event: BootstrapEvent,
  previous: ShellSnapshot | undefined
): Pick<ShellSnapshot, "from_version" | "to_version"> {
  if (event.versions) {
    return {
      ...(event.versions.from ? { from_version: event.versions.from } : {}),
      to_version: event.versions.to,
    };
  }
  return {
    ...(previous?.from_version ? { from_version: previous.from_version } : {}),
    ...(previous?.to_version ? { to_version: previous.to_version } : {}),
  };
}

function migrationWork(event: BootstrapEvent): Partial<ShellSnapshot> {
  return {
    ...(event.pct === undefined ? {} : { pct: event.pct }),
    ...(event.completed === undefined ? {} : { completed: event.completed }),
    ...(event.total === undefined ? {} : { total: event.total }),
    ...(event.migration === undefined
      ? {}
      : { work: SESSION_MIGRATIONS.has(event.migration) ? "sessions" : "records" }),
  };
}

export function bootstrapSnapshot(
  event: BootstrapEvent,
  logPath = "desktop-bootstrap.jsonl",
  previous?: ShellSnapshot
): ShellSnapshot {
  const currentAttempt = previous?.attempt ?? 1;
  if (event.status === "failed" || event.phase === "failed") {
    const skew = skewSnapshot(event);
    if (skew) return skew;
    return {
      state: "error",
      ...versions(event, previous),
      error: {
        code: failureCode(event),
        safe_message: publicSafeText(event.message, "The CompozyOS runtime could not start."),
        log_path: logPath,
      },
    };
  }
  switch (event.phase) {
    case "resolve":
      return { state: "resolving" };
    case "attach":
      return { state: "attaching", ...versions(event, previous) };
    case "provision":
      if (event.stage === "migrate") {
        return {
          state: "provisioning",
          stage: "migrate",
          ...versions(event, previous),
          ...migrationWork(event),
        };
      }
      return {
        state: "provisioning",
        stage: event.status === "completed" ? "verify" : "install",
        ...versions(event, previous),
      };
    case "start":
      return {
        state: "starting",
        stage: "start",
        attempt: event.attempt ?? currentAttempt,
        ...versions(event, previous),
      };
    case "ready":
      return event.resolution === "attach"
        ? { state: "attaching", stage: "ready", ...versions(event, previous) }
        : {
            state: "starting",
            stage: "ready",
            attempt: event.attempt ?? currentAttempt,
            ...versions(event, previous),
          };
  }
}

import type { BootstrapEvent } from "../bootstrap/bootstrap-event";
import type { BootProgress, ShellSnapshot } from "../state/app-state";
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

/** Unknown keys named in the detail line; the rest are counted so the file path stays visible. */
const NAMED_UNKNOWN_KEYS = 5;

/**
 * Says what is wrong with the config file and where, from the typed failure. A failure with no
 * typed cause keeps the bootstrap's own message.
 */
function configFailureMessage(event: BootstrapEvent): string {
  const config = event.config;
  const path = config?.path?.trim() || "config.toml";
  const keys = config?.unknown_keys ?? [];
  if (keys.length > 0) {
    const named = keys.slice(0, NAMED_UNKNOWN_KEYS).join(", ");
    const rest = keys.length - NAMED_UNKNOWN_KEYS;
    const list = rest > 0 ? `${named} and ${rest} more` : named;
    return keys.length === 1
      ? `Your config file has a setting CompozyOS does not recognize: ${list}. Remove or rename it in ${path}, then try again.`
      : `Your config file has settings CompozyOS does not recognize: ${list}. Remove or rename them in ${path}, then try again.`;
  }
  if (config?.line !== undefined) {
    const column = config.column === undefined ? "" : `, column ${config.column}`;
    return `Your config file is not valid TOML at line ${config.line}${column}. Fix ${path}, then try again.`;
  }
  return event.message;
}

/** The bootstrap gave up waiting for another process's runtime update, restart or start. */
const MUTATION_BUSY = "mutation-in-progress";

function failureCode(event: BootstrapEvent): string {
  if (event.config) return "config_invalid";
  if (event.classification === MUTATION_BUSY) return "update_lock_held";
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

function failureMessage(event: BootstrapEvent): string {
  if (event.config) return configFailureMessage(event);
  if (event.classification === MUTATION_BUSY) {
    return "Another CompozyOS update or restart is still running. Wait for it to finish, then try again.";
  }
  return event.message;
}

/** What the runtime or the bootstrap is waiting on, when the event says. */
function bootProgress(event: BootstrapEvent): Pick<ShellSnapshot, "boot"> {
  const waiting = event.wait?.reason === "runtime_mutation" ? event.wait : undefined;
  const boot: BootProgress = {
    ...(event.readiness ? { readiness: event.readiness.phase } : {}),
    ...(event.readiness?.step ? { step: event.readiness.step } : {}),
    ...(waiting ? { wait: "runtime_mutation" as const } : {}),
    ...(waiting?.owner_pid === undefined ? {} : { owner_pid: waiting.owner_pid }),
  };
  return Object.keys(boot).length > 0 ? { boot } : {};
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
    // A terminal failure omits the attempt; the window keeps the one the start reached. A config
    // failure or a wait on another process is not a start attempt, so it carries none.
    const reached = event.attempt ?? previous?.attempt;
    const attempt = event.config || event.classification === MUTATION_BUSY ? undefined : reached;
    return {
      state: "error",
      ...(attempt === undefined ? {} : { attempt }),
      ...versions(event, previous),
      error: {
        code: failureCode(event),
        safe_message: publicSafeText(
          failureMessage(event),
          "The CompozyOS runtime could not start."
        ),
        log_path: logPath,
      },
    };
  }
  switch (event.phase) {
    case "resolve":
      return { state: "resolving" };
    case "attach":
      return { state: "attaching", ...versions(event, previous), ...bootProgress(event) };
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
        ...bootProgress(event),
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

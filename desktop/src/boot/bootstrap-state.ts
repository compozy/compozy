import type { BootstrapEvent } from "../bootstrap/bootstrap-event";
import type { ShellSnapshot } from "../state/app-state";
import { publicSafeText } from "../state/public-safe-text";

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

export function bootstrapSnapshot(
  event: BootstrapEvent,
  logPath = "desktop-bootstrap.jsonl"
): ShellSnapshot {
  if (event.status === "failed" || event.phase === "failed") {
    const skew = skewSnapshot(event);
    if (skew) return skew;
    return {
      state: "error",
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
      return { state: "attaching" };
    case "provision":
      return {
        state: "provisioning",
        stage: event.status === "completed" ? "verify" : "install",
      };
    case "start":
      return {
        state: "starting",
        stage: "start",
        ...(event.attempt ? { attempt: event.attempt } : {}),
      };
    case "ready":
      return event.resolution === "attach"
        ? { state: "attaching", stage: "ready" }
        : { state: "starting", stage: "ready" };
  }
}

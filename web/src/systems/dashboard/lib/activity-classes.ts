import type { HomeActivityEvent } from "../types";

const QUIET_TYPE_PREFIXES = ["tool.", "config.", "memory.", "hook.", "stream."] as const;
const QUIET_TYPES = new Set(["tool_call", "usage_update", "current_mode_update", "plan"]);

/**
 * Quiet events fold behind the activity feed's "quieter events" disclosure:
 * per-call tool traffic, config reads, and memory bookkeeping stay truthful
 * but never compete with lifecycle events for attention.
 */
export function isQuietActivityEvent(event: Pick<HomeActivityEvent, "type" | "outcome">): boolean {
  if (event.outcome === "failure" || event.outcome === "warning") {
    return false;
  }
  if (QUIET_TYPES.has(event.type)) {
    return true;
  }
  return QUIET_TYPE_PREFIXES.some(prefix => event.type.startsWith(prefix));
}

export type HomeActivityTone = "neutral" | "success" | "warning" | "danger";

export function homeActivityTone(event: Pick<HomeActivityEvent, "outcome">): HomeActivityTone {
  switch (event.outcome) {
    case "success":
      return "success";
    case "warning":
      return "warning";
    case "failure":
      return "danger";
    default:
      return "neutral";
  }
}

/** Task-family events that must refresh attention, outcome, and today aggregates immediately. */
export function isTaskLifecycleEvent(event: Pick<HomeActivityEvent, "type">): boolean {
  return event.type.startsWith("task.");
}

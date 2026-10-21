import type { SessionMonitorPayload } from "../adapters/session-monitor-api";

/** Composer trigger aria: "Monitors, {N} active". */
export function monitorControlLabel(active: number): string {
  return `Monitors, ${active} active`;
}

/** What one monitor waits for, in plain words. */
export function monitorWatchLabel(monitor: SessionMonitorPayload): string {
  if (monitor.kind === "timer") return "Timer";
  if (monitor.until === "match" && monitor.pattern) {
    return `Terminal output matches ${monitor.pattern}`;
  }
  return "Terminal exits";
}

/** Delivering monitors already fired and are waking the session. */
export function monitorIsWaking(monitor: SessionMonitorPayload): boolean {
  return monitor.status === "delivering";
}

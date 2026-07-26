import { useSyncExternalStore } from "react";

import { formatDuration } from "@agh/ui";

let clockNow = Date.now();
let clockTimer: number | null = null;
const clockListeners = new Set<() => void>();

function notifyClockListeners() {
  clockNow = Date.now();
  for (const listener of clockListeners) listener();
}

function subscribeClock(listener: () => void): () => void {
  clockListeners.add(listener);
  notifyClockListeners();
  clockTimer ??= window.setInterval(notifyClockListeners, 1000);
  return () => {
    clockListeners.delete(listener);
    if (clockListeners.size === 0 && clockTimer !== null) {
      window.clearInterval(clockTimer);
      clockTimer = null;
    }
  };
}

function subscribePausedClock(): () => void {
  return () => undefined;
}

function clockSnapshot(): number {
  return clockNow;
}

/** Returns an elapsed label that ticks once per second while the run is active. */
export function useLiveElapsed(startedAt?: string | null, active = false): string | undefined {
  const now = useSyncExternalStore(active ? subscribeClock : subscribePausedClock, clockSnapshot);

  if (!startedAt) return undefined;
  const startedMs = Date.parse(startedAt);
  if (Number.isNaN(startedMs)) return undefined;
  return formatDuration(Math.max(0, now - startedMs));
}

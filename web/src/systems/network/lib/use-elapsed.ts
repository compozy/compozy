import { useSyncExternalStore } from "react";

/**
 * Live ticker shared across work chips and other duration displays. A single
 * 1-second `setInterval` is preferable to per-component timers because many
 * `working` chips can render simultaneously in busy timelines.
 */
const TICK_MS = 1_000;

let listeners: Set<() => void> | null = null;
let timer: ReturnType<typeof setInterval> | null = null;
let cachedNow = Date.now();

function startTimer() {
  if (timer != null || typeof window === "undefined") {
    return;
  }
  timer = setInterval(() => {
    cachedNow = Date.now();
    if (!listeners) {
      return;
    }
    for (const listener of listeners) {
      listener();
    }
  }, TICK_MS);
}

function stopTimer() {
  if (timer != null && (listeners == null || listeners.size === 0)) {
    clearInterval(timer);
    timer = null;
  }
}

function subscribe(listener: () => void): () => void {
  if (listeners == null) {
    listeners = new Set();
  }
  listeners.add(listener);
  startTimer();
  return () => {
    listeners?.delete(listener);
    stopTimer();
  };
}

export interface UseElapsedOptions {
  /** Disable the live ticker for terminal states. */
  enabled?: boolean;
}

/**
 * Returns the seconds elapsed between `start` and the shared 1Hz tick. Returns
 * `null` when `start` is missing or invalid.
 */
export function useElapsedSeconds(
  start: string | Date | null | undefined,
  { enabled = true }: UseElapsedOptions = {}
): number | null {
  const now = useSyncExternalStore(
    enabled ? subscribe : () => () => undefined,
    () => cachedNow,
    () => cachedNow
  );

  if (start == null) {
    return null;
  }
  const startDate = start instanceof Date ? start : new Date(start);
  const startMs = startDate.getTime();
  if (Number.isNaN(startMs)) {
    return null;
  }
  return Math.max(0, Math.floor((now - startMs) / 1_000));
}

export function formatElapsedSeconds(seconds: number | null): string {
  if (seconds == null) {
    return "";
  }
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    const remainder = seconds % 60;
    return remainder === 0 ? `${minutes}m` : `${minutes}m ${remainder}s`;
  }
  const hours = Math.floor(minutes / 60);
  const remainder = minutes % 60;
  return remainder === 0 ? `${hours}h` : `${hours}h ${remainder}m`;
}

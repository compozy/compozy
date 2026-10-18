import { useSyncExternalStore } from "react";

const TICK_MS = 1_000;
const listeners = new Set<() => void>();
let timer: ReturnType<typeof setInterval> | null = null;
let nowMs = Date.now();

function publishTick(): void {
  nowMs = Date.now();
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  if (listeners.size === 1) {
    publishTick();
    timer = setInterval(publishTick, TICK_MS);
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && timer !== null) {
      clearInterval(timer);
      timer = null;
    }
  };
}

/**
 * The same 1 Hz clock for non-React consumers (labels that write their own
 * text): the listener reads the shared instant, so every live label on the
 * page ticks in phase with `useSecondClock`.
 */
export function subscribeSecondClock(listener: (nowMs: number) => void): () => void {
  return subscribe(() => listener(nowMs));
}

function subscribePaused(): () => void {
  return () => undefined;
}

function snapshot(): number {
  return nowMs;
}

/** One process-wide 1 Hz browser clock, active only while a visible consumer subscribes. */
export function useSecondClock(enabled = true): number {
  return useSyncExternalStore(enabled ? subscribe : subscribePaused, snapshot, snapshot);
}

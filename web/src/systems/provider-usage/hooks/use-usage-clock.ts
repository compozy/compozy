"use client";

import { useSyncExternalStore } from "react";

export const PROVIDER_USAGE_CLOCK_INTERVAL_MS = 30_000;

// One ticker for every countdown on screen. It only re-renders text derived
// from absolute reset times; it never triggers a fetch.
const listeners = new Set<() => void>();
let now = Date.now();
let timer: ReturnType<typeof setInterval> | null = null;

function tick() {
  now = Date.now();
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  if (listeners.size === 0) {
    now = Date.now();
    timer = setInterval(tick, PROVIDER_USAGE_CLOCK_INTERVAL_MS);
  }
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && timer !== null) {
      clearInterval(timer);
      timer = null;
    }
  };
}

function getSnapshot(): number {
  return now;
}

/** Client time for usage countdowns, advanced every 30 s while anything listens. */
export function useUsageClock(): number {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

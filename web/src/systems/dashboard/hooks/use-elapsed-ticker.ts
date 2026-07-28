import { useSyncExternalStore } from "react";

const listeners = new Set<() => void>();
let intervalId: ReturnType<typeof setInterval> | undefined;
let nowSeconds = Math.floor(Date.now() / 1000);

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  if (intervalId === undefined) {
    nowSeconds = Math.floor(Date.now() / 1000);
    intervalId = setInterval(() => {
      nowSeconds = Math.floor(Date.now() / 1000);
      for (const notify of listeners) {
        notify();
      }
    }, 1000);
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && intervalId !== undefined) {
      clearInterval(intervalId);
      intervalId = undefined;
    }
  };
}

function getSnapshot(): number {
  return nowSeconds;
}

function getServerSnapshot(): number {
  return 0;
}

/**
 * One shared 1s interval for every elapsed timer on the page — run cards must
 * tick in lockstep instead of each mounting its own interval.
 */
export function useElapsedNowSeconds(): number {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

export function elapsedSecondsSince(nowSecondsValue: number, startedAtIso: string): number {
  const startedAt = Date.parse(startedAtIso);
  if (Number.isNaN(startedAt)) {
    return 0;
  }
  return Math.max(0, nowSecondsValue - Math.floor(startedAt / 1000));
}

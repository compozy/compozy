import { useSecondClock } from "@/hooks/use-second-clock";

/**
 * One shared 1s interval for every elapsed timer on the page — run cards must
 * tick in lockstep instead of each mounting its own interval.
 */
export function useElapsedNowSeconds(enabled = true): number {
  return Math.floor(useSecondClock(enabled) / 1_000);
}

export function elapsedSecondsSince(nowSecondsValue: number, startedAtIso: string): number {
  const startedAt = Date.parse(startedAtIso);
  if (Number.isNaN(startedAt)) {
    return 0;
  }
  return Math.max(0, nowSecondsValue - Math.floor(startedAt / 1000));
}

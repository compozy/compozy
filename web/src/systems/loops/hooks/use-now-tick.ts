import { useSecondClock } from "@/hooks/use-second-clock";

/**
 * One shared 1s clock while the run ticks; parked and terminal spans stay
 * timestamp-frozen. Returns the current epoch ms, re-rendering once per second
 * only while `enabled`.
 */
export function useNowTick(enabled: boolean): number {
  return useSecondClock(enabled);
}

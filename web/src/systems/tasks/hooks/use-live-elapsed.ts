import { formatDuration } from "@compozy/ui";
import { useSecondClock } from "@/hooks/use-second-clock";

/** Returns an elapsed label that ticks once per second while the run is active. */
export function useLiveElapsed(startedAt?: string | null, active = false): string | undefined {
  const now = useSecondClock(active);

  if (!startedAt) return undefined;
  const startedMs = Date.parse(startedAt);
  if (Number.isNaN(startedMs)) return undefined;
  return formatDuration(Math.max(0, now - startedMs));
}

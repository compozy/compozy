import { useSecondClock } from "@/hooks/use-second-clock";
import { useNetworkLiveDataEnabled } from "../hooks/use-network-live-data-enabled";

/** Controls whether this elapsed value subscribes to the shared 1 Hz clock. */
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
  const liveDataEnabled = useNetworkLiveDataEnabled();
  const now = useSecondClock(enabled && liveDataEnabled);

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

import { formatDuration } from "@agh/ui";

/** Format summed session elapsed seconds for Overview/Sessions metric display. */
export function formatAgentRuntimeDuration(totalSeconds: number): string | null {
  return formatDuration(Math.floor(totalSeconds) * 1_000);
}

/** Compact token counts for KPI and chart figures: 1.4M, 2M, 320K, 950. */
export function formatHomeTokens(total: number): string {
  if (total >= 1_000_000) {
    const millions = total / 1_000_000;
    return `${Number.isInteger(millions) ? millions : millions.toFixed(1)}M`;
  }
  if (total >= 1_000) {
    return `${Math.round(total / 1_000)}K`;
  }
  return String(total);
}

/**
 * Shared second→label formatter for the home surfaces. `compact` renders coarse
 * `2h 5m` / `12m` spans (pulse insights); `clock` renders zero-padded live
 * timers `1h 05m` / `3m 09s` (Working Now).
 */
export function formatHomeDurationSeconds(
  totalSeconds: number,
  style: "compact" | "clock"
): string {
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  if (style === "compact") {
    return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`;
  }
  if (totalSeconds >= 3600) {
    return `${hours}h ${String(minutes).padStart(2, "0")}m`;
  }
  const seconds = totalSeconds % 60;
  return `${minutes}m ${String(seconds).padStart(2, "0")}s`;
}

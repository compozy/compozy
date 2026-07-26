/**
 * Time formatters consumed by Tasks/Bridges/Knowledge/Settings runtime surfaces.
 *
 * The canonical implementation lives in `@agh/ui` (`packages/ui/src/lib/format-time.ts`)
 * because the `<Time>` primitive must consume them without crossing the
 * `@agh/ui` → `web/` package boundary. This module is a thin re-export so
 * runtime callsites can keep their `@/lib/format-time` import path.
 *
 * `formatUptimeSeconds` is web-owned (daemon health + extension runtime rails).
 */
export {
  FORMAT_TIME_FALLBACK,
  formatAbsoluteTime,
  formatDuration,
  formatRelativeTime,
} from "@agh/ui";

const SECOND = 1;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/** Humanize daemon/extension uptime seconds for operator rails (e.g. `5h 7m`). */
export function formatUptimeSeconds(seconds: number | null | undefined): string {
  if (typeof seconds !== "number" || !Number.isFinite(seconds) || seconds < 0) {
    return "—";
  }

  if (seconds < MINUTE) {
    return `${Math.round(seconds)}s`;
  }

  if (seconds < HOUR) {
    const minutes = Math.floor(seconds / MINUTE);
    const remainder = Math.floor(seconds % MINUTE);
    return remainder === 0 ? `${minutes}m` : `${minutes}m ${remainder}s`;
  }

  if (seconds < DAY) {
    const hours = Math.floor(seconds / HOUR);
    const remainder = Math.floor((seconds % HOUR) / MINUTE);
    return remainder === 0 ? `${hours}h` : `${hours}h ${remainder}m`;
  }

  const days = Math.floor(seconds / DAY);
  const remainder = Math.floor((seconds % DAY) / HOUR);
  return remainder === 0 ? `${days}d` : `${days}d ${remainder}h`;
}

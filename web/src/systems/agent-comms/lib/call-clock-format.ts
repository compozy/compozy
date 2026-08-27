/**
 * Clock copy that belongs on the call record, not on `<Time>`'s defaults.
 *
 * Timeline hours are UTC `HH:MM:SS` so the rail matches the daemon's ISO
 * instants. Idle TTL states its physics (`suspended` / `expires in …`) instead
 * of a bare countdown. Settled duration is `settled − created`.
 */
import { formatCompactRelativeTime, formatDuration, formatRelativeTime } from "@compozy/ui";

import type { CallIdleTtl } from "./call-detail-view-model";

export function formatUtcClock(iso: string): string {
  const parsed = Date.parse(iso);
  if (!Number.isFinite(parsed)) return "—";
  return new Date(parsed).toISOString().slice(11, 19);
}

export function formatIdleTtlCopy(ttl: CallIdleTtl, now: number = Date.now()): string | null {
  if (ttl.kind === "suspended") return "suspended while running";
  if (ttl.kind === "counting") {
    const expiresAt = Date.parse(ttl.expiresAt);
    if (Number.isFinite(expiresAt) && expiresAt > now) {
      return `expires ${formatCompactRelativeTime(ttl.expiresAt, now)}`;
    }
    return formatRelativeTime(ttl.expiresAt, now);
  }
  return null;
}

export function formatSettledDuration(createdAt: string, settledAt: string): string | null {
  const created = Date.parse(createdAt);
  const settled = Date.parse(settledAt);
  if (!Number.isFinite(created) || !Number.isFinite(settled) || settled <= created) {
    return null;
  }
  return formatDuration(settled - created);
}

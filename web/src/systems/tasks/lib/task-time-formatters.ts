import { formatDuration } from "@agh/ui";

const SECOND = 1000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

export function formatRelativeTime(value?: string | null, now: Date = new Date()): string {
  if (!value) {
    return "—";
  }

  const ts = Date.parse(value);
  if (Number.isNaN(ts)) {
    return "—";
  }

  const delta = Math.max(0, now.getTime() - ts);
  if (delta < MINUTE) {
    return "now";
  }

  if (delta < HOUR) {
    const minutes = Math.floor(delta / MINUTE);
    return `${minutes}m`;
  }

  if (delta < DAY) {
    const hours = Math.floor(delta / HOUR);
    return `${hours}h`;
  }

  const days = Math.floor(delta / DAY);
  return `${days}d`;
}

export function formatDurationMs(ms?: number | null): string {
  if (typeof ms !== "number" || !Number.isFinite(ms) || ms < 0) {
    return "—";
  }

  if (ms < SECOND) {
    return `${Math.round(ms)}ms`;
  }

  return formatDuration(ms);
}

export function formatPercent(value?: number | null): string {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return "—";
  }

  const rounded = Math.max(0, Math.min(100, Math.round(value)));
  return `${rounded}%`;
}

export function formatAttemptLabel(current?: number | null, total?: number | null): string | null {
  if (typeof current !== "number") {
    return null;
  }

  if (typeof total === "number" && total > 0) {
    return `attempt ${current} of ${total}`;
  }

  return `attempt ${current}`;
}

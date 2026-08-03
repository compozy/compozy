const SHORT_DATE = new Intl.DateTimeFormat("en-US", {
  month: "short",
  day: "2-digit",
  year: "numeric",
  timeZone: "UTC",
});
const COMPACT_DATE = new Intl.DateTimeFormat("en-US", {
  month: "short",
  day: "2-digit",
  timeZone: "UTC",
});

export function formatDate(iso: string): string {
  return SHORT_DATE.format(new Date(iso));
}

export function formatDateCompact(iso: string): string {
  return COMPACT_DATE.format(new Date(iso));
}

export function formatReadingTime(minutes: number): string {
  const rounded = Math.max(1, Math.round(minutes));
  return `${rounded} min`;
}

export function categoryLabel(slug: string): string {
  return slug.charAt(0).toUpperCase() + slug.slice(1);
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB"];
  let value = bytes / 1024;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)} ${units[unitIndex]}`;
}

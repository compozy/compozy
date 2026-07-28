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

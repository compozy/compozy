export const SIZE = { width: 1200, height: 630 } as const;

export const COLORS = {
  canvas: "#141312",
  surface: "#1E1C1B",
  border: "#3C3A39",
  accent: "#E8572A",
  textPrimary: "#E5E5E7",
  textSecondary: "#8E8E93",
  textTertiary: "#636366",
  textLabel: "#98989D",
} as const;

export const FONTS = {
  inter: "Inter",
  display: "Playfair Display",
  mono: "JetBrains Mono",
} as const;

export function truncate(value: string | undefined, max: number): string {
  if (!value) return "";
  if (max <= 0) return "";
  if (value.length <= max) return value;
  if (max <= 3) return ".".repeat(max);
  return `${value.slice(0, max - 3).trimEnd()}...`;
}

export function formatBlogDate(iso: string | undefined): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return date
    .toLocaleDateString("en-US", {
      month: "short",
      day: "2-digit",
      year: "numeric",
      timeZone: "UTC",
    })
    .toUpperCase();
}

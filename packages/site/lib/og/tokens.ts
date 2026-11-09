export const SIZE = { width: 1200, height: 630 } as const;

// Identity v1.2 values, mirrored from @compozy/ui tokens.css (Satori takes literal colours).
// Neutral ink ladder; on the dark cards the brand is signal blue (a fill/mark colour), and brand
// used as text takes the lighter `brandText`.
export const COLORS = {
  canvas: "#0a0a0a",
  surface: "#1b1b1b",
  border: "#383838",
  brand: "#1f87fc",
  brandText: "#4a9dff",
  textPrimary: "#ececec",
  textSecondary: "#989898",
  textTertiary: "#5e5e5e",
  textLabel: "#8e8e8e",
} as const;

export const FONTS = {
  sans: "Figtree",
  display: "Instrument Sans",
  mono: "Geist Mono",
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

export const SIZE = { width: 1200, height: 630 } as const;

// v1.2 redesign values (Satori takes literal colours): the #0a0a0a site surface, the shared
// neutral text ramp, translucent white hairlines, and the brand beam as the one accent.
export const COLORS = {
  canvas: "#0a0a0a",
  surface: "#131313",
  hairline: "rgba(255, 255, 255, 0.08)",
  edge: "rgba(255, 255, 255, 0.12)",
  beamFrom: "#1df0d7",
  beamTo: "#1f87fc",
  brandText: "#4a9dff",
  textPrimary: "#ececec",
  textSecondary: "#989898",
  textTertiary: "#8e8e8e",
} as const;

/** The brand beam, as a horizontal accent rule. */
export const BEAM = `linear-gradient(90deg, ${COLORS.beamFrom}, ${COLORS.beamTo})`;
/** The blue glow the product window casts on the landing, in the top-right corner. */
export const GLOW =
  "radial-gradient(circle at 100% 0%, rgba(31, 135, 252, 0.28), rgba(31, 135, 252, 0) 55%)";

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

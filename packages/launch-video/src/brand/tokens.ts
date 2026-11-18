/**
 * Design tokens mirrored from packages/ui/src/tokens.css.
 *
 * Remotion renders outside the product's Tailwind pipeline, so the values are inlined here as
 * plain constants. Treat tokens.css as the source of truth: when it changes, update this file.
 */

export const color = {
  /** --color-rail */
  rail: "#0a0a0a",
  /** --color-canvas */
  canvas: "#131313",
  /** --color-canvas-soft */
  canvasSoft: "#1a1a1a",
  /** --color-elevated */
  elevated: "#1b1b1b",
  /** --color-fg */
  fg: "#ececec",
  /** --color-fg-strong */
  fgStrong: "#ffffff",
  /** --color-muted-foreground */
  mutedForeground: "#989898",
  /** --color-brand */
  brand: "#2f5bff",
  /** --color-brand-ink */
  brandInk: "#ffffff",
  /** --color-brand-tint-strong */
  brandTintStrong: "rgba(47, 91, 255, 0.16)",
  /** --color-line */
  line: "#262626",
  /** --color-line-strong */
  lineStrong: "#383838",
  /** --wallpaper-teal — the only second hue, wallpaper depth only */
  wallpaperTeal: "#225555",
  /** --wallpaper-grid dot ink */
  gridDot: "rgba(255, 255, 255, 0.035)",
} as const;

export const radius = {
  /** --radius-chip */
  chip: 7,
  /** --radius-md */
  md: 10,
} as const;

/** --font-weight-medium is 500; Figtree's UI weight (450) stays a product-only step. */
export const weight = {
  normal: 400,
  medium: 500,
  semibold: 600,
} as const;

export const tracking = {
  /** --tracking-tight */
  tight: "-0.014em",
  /** --tracking-body */
  body: "-0.01em",
  /** --tracking-eyebrow-caps, also the mono metadata rail tracking */
  wide: "0.06em",
} as const;

/**
 * --ease-out: content-tier motion inside a window body.
 * Remotion's Easing.bezier takes the four control-point numbers.
 */
export const EASE_OUT = [0.22, 1, 0.36, 1] as const;

/** --ease-spring: the shell tier, for chrome-scale spatial transitions. */
export const EASE_SPRING = [0.32, 0.72, 0.28, 1] as const;

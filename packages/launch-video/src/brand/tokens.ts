/**
 * Design tokens mirrored from packages/ui/src/tokens.css.
 *
 * Remotion renders outside the product's Tailwind pipeline, so the values are inlined here as
 * plain constants. Treat tokens.css as the source of truth: when it changes, update this file.
 */

export const color = {
  /** --color-rail */
  rail: "#0c0b0b",
  /** --color-canvas */
  canvas: "#171615",
  /** --color-canvas-soft */
  canvasSoft: "#1f1e1c",
  /** --color-elevated */
  elevated: "#2a2927",
  /** --color-fg */
  fg: "#eeedeb",
  /** --color-fg-strong */
  fgStrong: "#f7f6f4",
  /** --color-muted */
  muted: "#a4a29e",
  /** --color-accent */
  accent: "#e8572a",
  /** --color-accent-ink */
  accentInk: "#17110f",
  /** --color-accent-tint-strong */
  accentTintStrong: "rgba(232, 87, 42, 0.16)",
  /** --color-line */
  line: "rgba(255, 255, 255, 0.055)",
  /** --color-line-strong */
  lineStrong: "rgba(255, 255, 255, 0.09)",
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

/**
 * The product's --font-weight-medium is 510, a half-step only a variable Geist can hit. Google
 * Fonts ships Geist as static instances (100..900 by hundreds), so the renderable medium here is
 * 500 — asking for 510 would silently snap to it anyway.
 */
export const weight = {
  normal: 400,
  medium: 500,
  semibold: 600,
} as const;

export const tracking = {
  /** --tracking-tight */
  tight: "-0.014em",
  /** --tracking-body */
  body: "-0.006em",
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

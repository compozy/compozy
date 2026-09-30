export type PillTone = "neutral" | "accent" | "success" | "warning" | "danger" | "info";
/** `count` is the compact numeric badge (unread/needs-you counts); prefer `PillCount`. */
export type PillSize = "xs" | "sm" | "md" | "count";
/**
 * Tint is the default fill. Hollow is an outline plate. Plain is a status
 * indicator — no plate and no padding, `muted` ink, the tone riding only the
 * inner `Pill.Dot` / glyph — so a state ("Idle", "Running") never reads as a
 * button.
 */
export type PillForm = "tint" | "hollow" | "plain";

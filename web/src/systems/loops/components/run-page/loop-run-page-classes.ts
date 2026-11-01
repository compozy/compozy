// Class strings the run page's real sections and their skeletons share, so a placeholder is the
// size of the thing it stands in for. Kept in a module of their own: a component file that also
// exports constants cannot be hot-reloaded without losing component state.

// Needs-you leads in the accent (amber is kept for real warnings); a
// calm verdict sits in the sunken inset.
export const WEIGHT_CLASS = {
  calm: "border-transparent bg-sunken",
  lead: "border-brand/40 bg-brand-tint",
  destructive: "border-destructive/40 bg-destructive-tint",
} as const;

/** The briefing strip's frame. */
export const BRIEFING_FRAME_CLASS = "flex items-start gap-3 rounded-lg border px-4 py-4";

/** The progress card, its heading and its counts line. */
export const PROGRESS_CARD_CLASS = "overflow-hidden rounded-lg bg-card shadow-card";
export const PROGRESS_HEADING_CLASS =
  "text-item-title font-medium tracking-tight text-pretty text-fg-strong";
export const PROGRESS_META_CLASS =
  "mt-2 flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-form-label text-muted-foreground";

/** A step row of the progress list. */
export const STEP_ROW_CLASS =
  "flex items-start gap-2.5 border-t border-line-soft py-2 first:border-t-0";

/** The activity card and one beat of it. */
export const STORY_CARD_CLASS = "overflow-hidden rounded-lg bg-card shadow-card px-4 py-2";
export const BEAT_ROW_CLASS = "flex items-start gap-2.5 py-2";

/** The rail's card of usage, about and Inspect. */
export const RAIL_CARD_CLASS = "rounded-lg bg-card shadow-card";

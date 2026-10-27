// Frames that a real task surface and its loading skeleton both wear. They live here, in one
// place, so a placeholder measures the same box as the content that replaces it and the two
// cannot drift when the frame changes.

/** The card the activity feed, the run's feed and their skeletons sit in. */
export const TASK_TIMELINE_CARD_CLASS = "rounded-lg bg-card px-4 pt-3 shadow-card";

/** The demoted meta line under the head, on the task, the run and their skeletons. */
export const TASK_SUBHEAD_CLASS =
  "mb-5 flex min-w-0 flex-wrap items-center gap-2 border-b border-line pb-4 text-form-label text-subtle";

/** The card the task and run rails sit in, and the footer that closes it. */
export const TASK_RAIL_CLASS = "overflow-hidden rounded-lg bg-card shadow-card";
export const TASK_RAIL_FOOTER_CLASS =
  "flex items-center gap-2 border-t border-line-soft px-3 py-2.5";

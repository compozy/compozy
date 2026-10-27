/** The comparison column, shared by the loaded view and its skeleton. */
export const DIFF_VIEW_CLASS = "flex min-w-0 flex-col gap-4";

/** A change group's card, shared with the skeleton so it is the same size and radius. */
export const DIFF_GROUP_CLASS = "overflow-hidden rounded-lg bg-card shadow-card";

/** A diff row's frame, shared with the skeleton so a placeholder row is the same height. */
export const DIFF_ROW_CLASS =
  "flex items-start gap-2.5 border-t border-line-soft px-3 py-2.5 first:border-t-0";

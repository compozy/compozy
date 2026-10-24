// Shared by the real rows and their skeleton so a run row is the same height either way. In a module
// of their own so the skeleton does not bring the live rows' helpers with it.
export const RECENT_RUNS_CLASS = "flex flex-col rounded-lg bg-card shadow-card";
export const RECENT_RUN_ROW_CLASS =
  "grid grid-cols-[auto_minmax(0,1fr)_auto_auto] items-center gap-3 border-t border-line-soft px-4 py-3 first:border-t-0";

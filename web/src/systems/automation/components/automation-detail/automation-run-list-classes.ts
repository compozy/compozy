// The run list's panel and row grid, shared by the live list and its skeleton. In a module of their
// own so the skeleton does not bring the live list's helpers with it.
export const PANEL_SHELL = "overflow-hidden rounded-lg bg-card shadow-card";

export const RUN_ROW_GRID =
  "grid w-full grid-cols-[6.5rem_minmax(0,1fr)_18px] items-center gap-3 px-4 py-2.75 md:grid-cols-[6.75rem_minmax(0,1fr)_4.5rem_5.5rem_18px]";

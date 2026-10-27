// The inventory rows' frame, row and foot, shared with the skeleton so a placeholder row is the
// same height. In a module of their own: a component file that also exports constants cannot be
// hot-reloaded without losing component state.

export const INVENTORY_ROWS_CLASS = "overflow-hidden rounded-lg bg-card shadow-card";
export const INVENTORY_ROW_CLASS = "flex items-center gap-4 px-4 py-3";
export const INVENTORY_FOOT_CLASS =
  "flex items-center justify-between gap-3 border-t border-line-soft px-4 py-2.5";
export const INVENTORY_CARDS_CLASS = "grid grid-cols-[repeat(auto-fill,minmax(250px,1fr))] gap-3";

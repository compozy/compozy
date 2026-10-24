// The kanban card's surface, shared by the card and its skeleton so a placeholder is the same box.
// In a module of their own so the skeleton does not bring the live card's helpers with it.

export const CARD_SURFACE_CLASS =
  "relative flex w-full min-w-0 shrink-0 flex-col gap-2 overflow-hidden rounded-lg bg-card p-3 text-left shadow-card";

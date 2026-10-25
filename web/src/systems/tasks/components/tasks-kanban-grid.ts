import type { CSSProperties } from "react";

// The board's grid and its loading cards, shared by the live board and its skeleton. In a module of
// their own so the skeleton does not bring the live cards' helpers with it.

/** The board: one track per column, scrolling on its own. */
export const KANBAN_BOARD_CLASS = "grid min-h-0 flex-1 gap-3 overflow-y-auto px-4 pt-4 pb-6";

/**
 * One track per column, derived from the canonical column set so it can never drift from
 * `KANBAN_COLUMNS`: a new status column widens the grid instead of wrapping the last column onto a
 * broken second row.
 */
export function kanbanBoardStyle(columnCount: number): CSSProperties {
  return { gridTemplateColumns: `repeat(${Math.max(columnCount, 1)}, minmax(0, 1fr))` };
}

/**
 * Placeholder cards per column while the board loads, in column order: a backlog-heavy shape that
 * fills the first columns and thins out, with some two-line titles among the one-line ones. Ids
 * are static; the placeholders carry no identity of their own.
 */
export const KANBAN_LOADING_CARDS = [
  [
    { id: "a", titleLines: 2 },
    { id: "b", titleLines: 1 },
    { id: "c", titleLines: 1 },
  ],
  [
    { id: "a", titleLines: 1 },
    { id: "b", titleLines: 2 },
  ],
  [{ id: "a", titleLines: 1 }],
  [{ id: "a", titleLines: 2 }],
  [
    { id: "a", titleLines: 1 },
    { id: "b", titleLines: 1 },
    { id: "c", titleLines: 2 },
  ],
] as const;

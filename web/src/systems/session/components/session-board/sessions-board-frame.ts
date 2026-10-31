// The geometry of the Sessions board, shared by the live cards and columns and by their loading
// placeholders (`sessions-board-skeleton`) so the two cannot drift. Classes only: nothing here
// reads a query or a store, which keeps the skeleton cheap to render anywhere.

/** One card's surface: the meta, title and footer rows stack at an 8 px gap on a 12 px pad. */
export const SESSION_BOARD_CARD_FRAME =
  "relative flex w-full min-w-0 shrink-0 flex-col gap-2 rounded-lg bg-card p-3 shadow-card";

/** Card row 1: where the work lives (a pull request or branch) and its one status. */
export const SESSION_BOARD_CARD_META_ROW = "flex h-4.5 min-w-0 items-center gap-2";

/** Card row 2: the title, with the unread dot ahead of it. */
export const SESSION_BOARD_CARD_TITLE_ROW = "flex min-w-0 items-start gap-1.75";

/** The title's type: a placeholder inside it takes the same line box as the clamped title. */
export const SESSION_BOARD_CARD_TITLE_TYPE = "text-small-body leading-snug font-medium text-fg";

/** Card row 3: the agent and when it last moved. */
export const SESSION_BOARD_CARD_FOOTER_ROW =
  "flex h-4.5 min-w-0 items-center gap-2 text-eyebrow tracking-eyebrow text-subtle";

/**
 * A selectable card's checkbox, over row 3's leading 16 px slot: the 12 px pad plus the
 * 1 px that centres the 16 px box in the 18 px row.
 */
export const SESSION_BOARD_CARD_CHECKBOX = "absolute bottom-3.25 left-3 z-10";

/** A flat `sunken` column panel; any column folds it into a 44 px rail. */
export const SESSIONS_BOARD_COLUMN_FRAME =
  "flex min-h-0 w-74 shrink-0 flex-col overflow-hidden rounded-lg bg-sunken";

export const SESSIONS_BOARD_COLUMN_HEADER = "flex shrink-0 items-center gap-2 px-3 pt-3 pb-2";

/** A folded column: one full-height button stacking the mark, the vertical title and the count. */
export const SESSIONS_BOARD_COLUMN_RAIL =
  "flex min-h-40 w-full flex-1 cursor-pointer flex-col items-center gap-2.5 rounded-lg px-0 py-3 outline-none hover:bg-hover focus-visible:shadow-focus-inset";

export const SESSIONS_BOARD_COLUMN_TITLE = "text-item-title font-medium text-fg";

/** The facet count after the title: its line box is the header's tallest, so a header reserves it. */
export const SESSIONS_BOARD_COLUMN_COUNT = "text-small-body text-subtle tabular-nums";

export const SESSIONS_BOARD_COLUMN_BODY =
  "flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-2 pt-1 pb-3";

/** The four columns side by side: stretched to the canvas (`fill`) or sized to their cards (`lane`). */
export const SESSIONS_BOARD_COLUMNS_ROW = {
  fill: "flex min-h-full w-max items-stretch gap-3",
  lane: "flex w-max items-start gap-3",
} as const;

/** Group-by swimlanes stacked on the canvas, each a header over its own row of columns. */
export const SESSIONS_BOARD_LANES = "flex w-max min-w-full flex-col gap-6";
export const SESSIONS_BOARD_LANE = "flex flex-col gap-2";
export const SESSIONS_BOARD_LANE_HEADER = "flex h-7 items-center gap-2 px-1";
export const SESSIONS_BOARD_LANE_COUNT = "text-small-body text-subtle tabular-nums";

/** The Archive bar under the board: its ruled frame and the 41 px row inside it. */
export const SESSIONS_BOARD_ARCHIVE_FRAME = "shrink-0 border-t border-line bg-canvas";
export const SESSIONS_BOARD_ARCHIVE_BAR =
  "flex h-tabs-list w-full items-center gap-2 px-4 text-small-body text-muted";

/** The board's own frame: the canvas, and the scroller the columns or lanes sit in. */
export const SESSIONS_BOARD_FRAME = "flex min-h-0 min-w-0 flex-1 flex-col bg-canvas";
export const SESSIONS_BOARD_SCROLLER = "min-h-0 flex-1 overflow-auto p-4";

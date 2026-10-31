import { Archive } from "lucide-react";

import { Skeleton, SkeletonRegion, SkeletonText, cn } from "@compozy/ui";

import { SESSION_BOARD_COLUMNS, type SessionBoardColumnId } from "../../lib/session-board-contract";
import { SessionsBoardColumnGlyph } from "./sessions-board-column-glyph";
import {
  SESSIONS_BOARD_SHELF_BAR,
  SESSIONS_BOARD_SHELF_FRAME,
  SESSIONS_BOARD_COLUMN_BODY,
  SESSIONS_BOARD_COLUMN_COUNT,
  SESSIONS_BOARD_COLUMN_FRAME,
  SESSIONS_BOARD_COLUMN_HEADER,
  SESSIONS_BOARD_COLUMN_TITLE,
  SESSIONS_BOARD_COLUMNS_ROW,
  SESSIONS_BOARD_FRAME,
  SESSIONS_BOARD_LANE,
  SESSIONS_BOARD_LANE_COUNT,
  SESSIONS_BOARD_LANE_HEADER,
  SESSIONS_BOARD_LANES,
  SESSIONS_BOARD_SCROLLER,
  SESSION_BOARD_CARD_FOOTER_ROW,
  SESSION_BOARD_CARD_FRAME,
  SESSION_BOARD_CARD_META_ROW,
  SESSION_BOARD_CARD_TITLE_ROW,
  SESSION_BOARD_CARD_TITLE_TYPE,
} from "./sessions-board-frame";

// Content-shaped placeholders for the Sessions board. A card repeats the live card's rows
// (`sessions-board-frame`) with `SkeletonText` where the PR or branch, the title, the agent and
// the time go, and a column repeats the live column's panel and header, so a placeholder is the
// size of what replaces it. Static and query-free: the shapes import no hook, store or menu, so a
// window that shows them before the board's code has loaded stays cheap. Every shape is
// presentational; whoever renders them wraps the area in the one `SkeletonRegion` that carries
// the loading semantics.

interface CardShape {
  id: string;
  /** Title lines the placeholder stands in for: the live title clamps at two. */
  titleLines: 1 | 2;
  /** The meta row (a pull request or branch, and a status), which most cards carry. */
  meta: boolean;
}

/** Cards per column while it loads, a working-heavy board that thins out toward Needs you. */
const COLUMN_CARDS: Record<SessionBoardColumnId, readonly CardShape[]> = {
  needs_you: [
    { id: "a", titleLines: 2, meta: true },
    { id: "b", titleLines: 1, meta: true },
  ],
  working: [
    { id: "a", titleLines: 1, meta: false },
    { id: "b", titleLines: 2, meta: true },
    { id: "c", titleLines: 1, meta: true },
  ],
  waiting: [{ id: "a", titleLines: 1, meta: true }],
  in_review: [
    { id: "a", titleLines: 2, meta: true },
    { id: "b", titleLines: 1, meta: true },
  ],
  done: [
    { id: "a", titleLines: 1, meta: true },
    { id: "b", titleLines: 2, meta: false },
    { id: "c", titleLines: 1, meta: true },
  ],
};

/** A grouped board while its lanes load: two swimlanes, the first fuller than the second. */
const LANES = [
  { id: "a", cardsPerColumn: 2 },
  { id: "b", cardsPerColumn: 1 },
] as const;

const META_WIDTHS = ["w-24", "w-16", "w-20"] as const;
const AGENT_WIDTHS = ["w-14", "w-10", "w-16"] as const;

/** Placeholders have no identity, so a card's position is what its widths vary by. */
function pick<T>(widths: readonly T[], position: number): T {
  return widths[position % widths.length];
}

/** One placeholder card: the meta row, the title (one or two lines) and the agent and time. */
function SessionBoardCardSkeleton({ shape, position }: { shape: CardShape; position: number }) {
  return (
    <div
      aria-hidden="true"
      className={cn(SESSION_BOARD_CARD_FRAME, "animate-skeleton-reveal")}
      data-slot="session-board-card-skeleton"
      data-testid="sessions-board-card-skeleton"
    >
      {shape.meta ? (
        <span className={SESSION_BOARD_CARD_META_ROW}>
          <span className="flex min-w-0 flex-1 items-center gap-1.25">
            <Skeleton className="size-3 shrink-0 rounded-xs" />
            <SkeletonText className={cn("font-mono text-mono-id", pick(META_WIDTHS, position))} />
          </span>
          <Skeleton className="h-3.5 w-12 shrink-0 rounded-pill" />
        </span>
      ) : null}
      <span className={SESSION_BOARD_CARD_TITLE_ROW}>
        <span className={cn("min-w-0 flex-1", SESSION_BOARD_CARD_TITLE_TYPE)}>
          <SkeletonText lastLineWidth={0.55} lines={shape.titleLines} />
        </span>
      </span>
      <span className={SESSION_BOARD_CARD_FOOTER_ROW}>
        <span className="flex max-w-28 min-w-0 shrink-0 items-center gap-1.25 overflow-hidden">
          <Skeleton className="size-3 shrink-0 rounded-xs" />
          <SkeletonText className={pick(AGENT_WIDTHS, position)} />
        </span>
        <SkeletonText className="ml-auto w-8 font-mono text-mono-id" />
      </span>
    </div>
  );
}

/** A column's placeholder cards, siblings: put them where the live column's cards go. */
function ColumnCardsSkeleton({ id, count }: { id: SessionBoardColumnId; count?: number }) {
  return COLUMN_CARDS[id]
    .slice(0, count)
    .map((shape, position) => (
      <SessionBoardCardSkeleton key={shape.id} position={position} shape={shape} />
    ));
}

/**
 * A live column's body while its first page loads: the column's own panel and header are real,
 * so only the cards are placeholders, under the one status region of this column.
 */
export function SessionsBoardColumnCardsSkeleton({ id }: { id: SessionBoardColumnId }) {
  return (
    <SkeletonRegion
      className="flex flex-col gap-2"
      data-testid={`sessions-board-column-skeleton-${id}`}
      label="Loading sessions"
    >
      <ColumnCardsSkeleton id={id} />
    </SkeletonRegion>
  );
}

/** A column's count before the facets answer: the figure's line box as a bar, so the header keeps its height. */
export function SessionsBoardColumnCountSkeleton() {
  return (
    <span className={SESSIONS_BOARD_COLUMN_COUNT}>
      <SkeletonText className="w-4" />
    </span>
  );
}

/** A whole column before any of it is known: the live panel and header over placeholder cards. */
function SessionsBoardColumnSkeleton({
  id,
  label,
  count,
}: {
  id: SessionBoardColumnId;
  label: string;
  count?: number;
}) {
  return (
    <div
      aria-hidden="true"
      className={SESSIONS_BOARD_COLUMN_FRAME}
      data-slot="sessions-board-column-skeleton"
    >
      <div className={SESSIONS_BOARD_COLUMN_HEADER}>
        <SessionsBoardColumnGlyph id={id} />
        <span className={SESSIONS_BOARD_COLUMN_TITLE}>{label}</span>
        <SessionsBoardColumnCountSkeleton />
        {id === "done" ? (
          <span className="ml-auto grid size-button-icon-xs shrink-0 place-items-center">
            <Skeleton className="size-3.5 rounded-xs" />
          </span>
        ) : null}
      </div>
      <div className={SESSIONS_BOARD_COLUMN_BODY}>
        <ColumnCardsSkeleton count={count} id={id} />
      </div>
    </div>
  );
}

function ColumnsRowSkeleton({
  layout,
  count,
  className,
}: {
  layout: keyof typeof SESSIONS_BOARD_COLUMNS_ROW;
  count?: number;
  className?: string;
}) {
  return (
    <div className={cn(SESSIONS_BOARD_COLUMNS_ROW[layout], className)}>
      {SESSION_BOARD_COLUMNS.map(column => (
        <SessionsBoardColumnSkeleton
          count={count}
          id={column.id}
          key={column.id}
          label={column.label}
        />
      ))}
    </div>
  );
}

/** A swimlane before its group is known: the header's name and count over its row of columns. */
function SessionsBoardLaneSkeleton({ cardsPerColumn }: { cardsPerColumn: number }) {
  return (
    <div
      aria-hidden="true"
      className={SESSIONS_BOARD_LANE}
      data-slot="sessions-board-lane-skeleton"
    >
      <div className={SESSIONS_BOARD_LANE_HEADER}>
        <span className="inline-flex min-w-0 items-center text-body">
          <SkeletonText className="w-32" />
        </span>
        <span className={SESSIONS_BOARD_LANE_COUNT}>
          <SkeletonText className="w-4" />
        </span>
      </div>
      <ColumnsRowSkeleton count={cardsPerColumn} layout="lane" />
    </div>
  );
}

function LanesSkeleton() {
  return LANES.map(lane => (
    <SessionsBoardLaneSkeleton cardsPerColumn={lane.cardsPerColumn} key={lane.id} />
  ));
}

/**
 * A grouped board's body while its lanes load, inside the board's own scroller: lane headers over
 * rows of columns. One status region for the whole body, which fades in together after the beat
 * so a load that settles at once never flashes empty panels.
 */
export function SessionsBoardLanesSkeleton() {
  return (
    <SkeletonRegion
      className={cn(SESSIONS_BOARD_LANES, "animate-skeleton-reveal")}
      data-testid="sessions-board-lanes-skeleton"
      label="Loading sessions"
    >
      <LanesSkeleton />
    </SkeletonRegion>
  );
}

export interface SessionsBoardSkeletonProps {
  /** `columns` is the ungrouped board, `lanes` the Group-by swimlanes. Defaults to `columns`. */
  layout?: "columns" | "lanes";
}

/**
 * The whole Sessions board before anything is known, for a caller that has no board yet (the
 * Sessions window's loading fallback): the canvas and its scroller, the four columns or the
 * swimlanes, and the Archive bar under them, in the geometry the loaded board mounts into, so the
 * swap moves nothing. One status region for the whole board, and no queries.
 */
export function SessionsBoardSkeleton({ layout = "columns" }: SessionsBoardSkeletonProps) {
  return (
    <SkeletonRegion
      className={SESSIONS_BOARD_FRAME}
      data-testid="sessions-board-skeleton"
      label="Loading sessions"
    >
      <div className={SESSIONS_BOARD_SCROLLER}>
        {layout === "lanes" ? (
          <div className={cn(SESSIONS_BOARD_LANES, "animate-skeleton-reveal")}>
            <LanesSkeleton />
          </div>
        ) : (
          <ColumnsRowSkeleton className="animate-skeleton-reveal" layout="fill" />
        )}
      </div>
      <div aria-hidden="true" className={SESSIONS_BOARD_SHELF_FRAME}>
        <div className={SESSIONS_BOARD_SHELF_BAR}>
          <span className="size-3.5 shrink-0" />
          <Archive aria-hidden="true" className="size-3.5 text-subtle" />
          <span>Archive</span>
          <SkeletonText className="w-4" />
        </div>
      </div>
    </SkeletonRegion>
  );
}

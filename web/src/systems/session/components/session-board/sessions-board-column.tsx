import { ChevronLeft, CircleSlash } from "lucide-react";
import { use, useId, useRef } from "react";

import { Alert, AlertActions, AlertDescription, Button, Spinner } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionsBoardColumnView, SessionsBoardFailure } from "../../hooks/use-sessions-board";
import { SessionsBoardRowsStoreContext } from "../../contexts/sessions-board-selection-context-value";
import { usePublishSessionListGroupRows } from "../../hooks/use-session-list-group-rows";
import type { SessionBoardColumnId, SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionBoardCardSlot, slottedCard } from "./session-board-card-slot";
import { SessionsBoardColumnGlyph } from "./sessions-board-column-glyph";
import { SessionsBoardSelectAll } from "./sessions-board-select-all";
import {
  SESSIONS_BOARD_COLUMN_BODY,
  SESSIONS_BOARD_COLUMN_COUNT,
  SESSIONS_BOARD_COLUMN_FRAME,
  SESSIONS_BOARD_COLUMN_HEADER,
  SESSIONS_BOARD_COLUMN_RAIL,
  SESSIONS_BOARD_COLUMN_TITLE,
} from "./sessions-board-frame";
import {
  SessionsBoardColumnCardsSkeleton,
  SessionsBoardColumnCountSkeleton,
} from "./sessions-board-skeleton";

const EMPTY_COPY: Record<SessionBoardColumnId, string> = {
  needs_you: "Nothing needs you",
  working: "Nothing running",
  waiting: "Nothing waiting on a wake",
  in_review: "No open pull requests",
  done: "Nothing finished yet",
};

export interface SessionsBoardColumnProps {
  column: SessionsBoardColumnView;
  currentSessionId?: string | null;
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
  onOpen: (session: SessionBoardPayload) => void;
  /** The column folds into a 44 px rail: its mark, vertical title and count expand it again. */
  collapsible?: { collapsed: boolean; onToggle: () => void };
  /** Session list verbs every card offers (context menu and `…`). */
  actions?: SessionLifecycleActionHandlers;
  /**
   * `fill`: the ungrouped board, columns stretch to the canvas. `lane`: a
   * swimlane row, columns size to their cards and scroll past a cap.
   */
  layout?: "fill" | "lane";
  /** Swimlanes repeat the columns: the lane names the region and prefixes test ids. */
  regionPrefix?: string;
  testIdPrefix?: string;
  /** Where the column joins the board selection: its key and on-screen rank. */
  selectionRows?: SessionsBoardSelectionRows;
}

/** One column's entry in the board selection's published rows. */
export interface SessionsBoardSelectionRows {
  key: string;
  rank: string;
}

function sessionCountLabel(count: number): string {
  return `${count} ${count === 1 ? "session" : "sessions"}`;
}

/** The facet count after the title; a bar of its size until the counts load, nothing if they never do. */
function SessionsBoardColumnCount({
  column,
  testId,
}: {
  column: SessionsBoardColumnView;
  testId: string;
}) {
  if (column.loading) return <SessionsBoardColumnCountSkeleton />;
  if (column.count === undefined) return null;
  return (
    <span className={SESSIONS_BOARD_COLUMN_COUNT} data-testid={testId}>
      {column.count}
    </span>
  );
}

/**
 * One board column (`_uiux.md` S4): a flat `sunken` panel whose labelled
 * region names its count. The count comes from `facets.board`, so it covers
 * cards that are not loaded yet.
 */
export function SessionsBoardColumn({
  column,
  currentSessionId = null,
  workspaceNameOf,
  onOpen,
  collapsible,
  actions,
  layout = "fill",
  regionPrefix,
  testIdPrefix = "sessions-board",
  selectionRows,
}: SessionsBoardColumnProps) {
  const bodyId = useId();
  const collapsed = collapsible?.collapsed === true;
  const count = column.count;
  const label = regionPrefix ? `${regionPrefix}, ${column.label}` : column.label;
  // A folded column keeps its selected cards, counted as hidden: they leave only the order.
  const shownIds = collapsed ? [] : column.sessions.map(session => session.id);
  const rows = use(SessionsBoardRowsStoreContext);
  usePublishSessionListGroupRows(
    selectionRows && rows ? rows : undefined,
    selectionRows?.key ?? column.id,
    {
      rank: selectionRows?.rank ?? "",
      sessions: column.sessions,
      visibleOrder: shownIds,
      // Folder moves resolve each selected session's own pair (`useSessionsBoardSelection`).
      pair: null,
      fetching: column.fetching,
      failed: column.failure !== null,
    }
  );
  const regionLabel = count === undefined ? label : `${label}, ${sessionCountLabel(count)}`;
  return (
    <section
      aria-label={regionLabel}
      className={cn(SESSIONS_BOARD_COLUMN_FRAME, collapsed && "w-11")}
      data-collapsed={collapsed ? "true" : undefined}
      data-testid={`${testIdPrefix}-column-${column.id}`}
    >
      <SessionsBoardColumnHeader
        bodyId={bodyId}
        collapsible={collapsible}
        column={column}
        label={label}
        shownIds={shownIds}
        testIdPrefix={testIdPrefix}
      />
      <div
        className={cn(
          SESSIONS_BOARD_COLUMN_BODY,
          layout === "lane" && "max-h-96",
          collapsed && "hidden"
        )}
        data-testid={`${testIdPrefix}-column-body-${column.id}`}
        id={bodyId}
      >
        <SessionsBoardColumnBody
          actions={actions}
          column={column}
          currentSessionId={currentSessionId}
          onOpen={onOpen}
          testIdPrefix={testIdPrefix}
          workspaceNameOf={workspaceNameOf}
        />
      </div>
    </section>
  );
}

/**
 * The column's mark (or its select-all while cards can be selected), title, facet count, and
 * fold. A folded column's header is the whole rail: one button that expands it again.
 */
function SessionsBoardColumnHeader({
  column,
  label,
  collapsible,
  bodyId,
  shownIds,
  testIdPrefix,
}: {
  column: SessionsBoardColumnView;
  label: string;
  collapsible: SessionsBoardColumnProps["collapsible"];
  bodyId: string;
  shownIds: readonly string[];
  testIdPrefix: string;
}) {
  // The pressed toggle unmounts with the fold it flips: its replacement takes focus.
  const moveFocus = useRef(false);
  const onToggle = () => {
    moveFocus.current = true;
    collapsible?.onToggle();
  };
  const focusIfMoved = (element: HTMLButtonElement | null) => {
    if (!element || !moveFocus.current) return;
    moveFocus.current = false;
    element.focus();
  };
  const count = (
    <SessionsBoardColumnCount
      column={column}
      testId={`${testIdPrefix}-column-count-${column.id}`}
    />
  );
  const toggle = collapsible
    ? {
        "aria-controls": bodyId,
        "aria-expanded": !collapsible.collapsed,
        "aria-label": `${collapsible.collapsed ? "Expand" : "Collapse"} ${column.label}`,
        "data-testid": `${testIdPrefix}-column-toggle-${column.id}`,
        type: "button" as const,
      }
    : null;
  if (collapsible?.collapsed && toggle) {
    return (
      <button
        {...toggle}
        onClick={onToggle}
        ref={focusIfMoved}
        className={cn(
          SESSIONS_BOARD_COLUMN_RAIL,
          "transition-colors duration-fast ease-out motion-reduce:transition-none"
        )}
        title={`Expand ${column.label}`}
      >
        <SessionsBoardColumnGlyph id={column.id} />
        <span className={cn(SESSIONS_BOARD_COLUMN_TITLE, "[writing-mode:vertical-rl]")}>
          {column.label}
        </span>
        {count}
      </button>
    );
  }
  return (
    <header className={cn(SESSIONS_BOARD_COLUMN_HEADER, "group/board-select")}>
      <SessionsBoardSelectAll
        ids={shownIds}
        label={label}
        testId={`${testIdPrefix}-column-select-${column.id}`}
      >
        <SessionsBoardColumnGlyph id={column.id} />
      </SessionsBoardSelectAll>
      <h2 className={SESSIONS_BOARD_COLUMN_TITLE}>{column.label}</h2>
      {count}
      {toggle ? (
        <Button
          {...toggle}
          className="ml-auto text-subtle"
          onClick={onToggle}
          ref={focusIfMoved}
          size="icon-xs"
          variant="quiet"
        >
          <ChevronLeft />
        </Button>
      ) : null}
    </header>
  );
}

function SessionsBoardColumnBody({
  column,
  currentSessionId,
  workspaceNameOf,
  onOpen,
  actions,
  testIdPrefix,
}: Omit<SessionsBoardColumnProps, "collapsible" | "layout" | "regionPrefix"> & {
  testIdPrefix: string;
}) {
  const BoardCard = slottedCard(use(SessionBoardCardSlot));
  if (column.loading) return <SessionsBoardColumnCardsSkeleton id={column.id} />;
  if (column.failure === "initial") {
    return (
      <div
        className="flex flex-1 flex-col items-center justify-center gap-2.5 px-3 py-8 text-center text-small-body text-muted"
        data-testid={`${testIdPrefix}-column-error-${column.id}`}
        role="alert"
      >
        <CircleSlash aria-hidden="true" className="size-4 text-subtle" />
        <span>Couldn't load {column.label.toLowerCase()} sessions.</span>
        <Button onClick={column.retry} size="sm" type="button" variant="secondary">
          Retry
        </Button>
      </div>
    );
  }
  const readError = (failure: RetainedFailure) =>
    column.failure === failure ? (
      <SessionsBoardReadError
        failure={failure}
        onRetry={column.retry}
        retrying={column.retrying}
        testId={`${testIdPrefix}-read-error-${column.id}`}
      />
    ) : null;
  return (
    <>
      {readError("refresh")}
      {column.sessions.length === 0 ? (
        <div
          className="flex flex-1 items-center justify-center px-3 py-8 text-center text-small-body text-subtle"
          data-testid={`${testIdPrefix}-column-empty-${column.id}`}
        >
          {EMPTY_COPY[column.id]}
        </div>
      ) : (
        column.sessions.map(session => (
          <BoardCard
            actions={actions}
            current={session.id === currentSessionId}
            key={session.id}
            onOpen={onOpen}
            session={session}
            workspaceName={workspaceNameOf(session)}
          />
        ))
      )}
      {readError("more")}
      {column.hasMore && column.failure !== "more" ? (
        <SessionsBoardLoadMore
          count={column.count}
          loaded={column.sessions.length}
          loadingMore={column.loadingMore}
          onLoadMore={column.loadMore}
          testId={`${testIdPrefix}-load-more-${column.id}`}
        />
      ) : null}
    </>
  );
}

type RetainedFailure = Exclude<SessionsBoardFailure, "initial">;

const READ_ERROR_COPY: Record<RetainedFailure, string> = {
  more: "Couldn't load more sessions.",
  refresh: "Couldn't refresh. Showing the last loaded sessions.",
};

/**
 * A failed later read over cards that stay usable: the next page, or a
 * background re-read. The retry is local to this column or bar.
 */
export function SessionsBoardReadError({
  failure,
  retrying,
  onRetry,
  testId,
  className,
}: {
  failure: RetainedFailure;
  retrying: boolean;
  onRetry: () => void;
  testId: string;
  className?: string;
}) {
  return (
    <Alert
      className={cn("shrink-0", className)}
      data-testid={testId}
      role={failure === "more" ? "alert" : "status"}
      variant="neutral"
    >
      <CircleSlash aria-hidden="true" />
      <AlertDescription className="text-small-body">{READ_ERROR_COPY[failure]}</AlertDescription>
      <AlertActions>
        <Button
          aria-busy={retrying || undefined}
          disabled={retrying}
          onClick={onRetry}
          size="sm"
          type="button"
          variant="secondary"
        >
          {retrying ? <Spinner aria-hidden="true" data-icon="inline-start" /> : null}
          Retry
        </Button>
      </AlertActions>
    </Alert>
  );
}

export function SessionsBoardLoadMore({
  loaded,
  count,
  loadingMore,
  onLoadMore,
  testId,
}: {
  loaded: number;
  count: number | undefined;
  loadingMore: boolean;
  onLoadMore: () => void;
  testId: string;
}) {
  return (
    <Button
      aria-busy={loadingMore || undefined}
      className="mt-1 shrink-0 self-center"
      data-testid={testId}
      disabled={loadingMore}
      onClick={onLoadMore}
      size="sm"
      type="button"
      variant="quiet"
    >
      {loadingMore ? <Spinner aria-hidden="true" data-icon="inline-start" /> : null}
      Load more
      {count !== undefined ? (
        <span className="font-mono text-mono-id text-faint tabular-nums">
          {loaded} of {count}
        </span>
      ) : null}
    </Button>
  );
}

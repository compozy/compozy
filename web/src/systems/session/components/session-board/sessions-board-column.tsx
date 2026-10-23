import { ChevronRight, CircleSlash, GitPullRequest } from "lucide-react";
import { useId } from "react";

import { Button, Skeleton, Spinner, StateGlyph } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionsBoardColumnView } from "../../hooks/use-sessions-board";
import type { SessionBoardColumnId, SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionBoardCard } from "./session-board-card";

const SKELETON_KEYS = ["a", "b", "c"] as const;

const EMPTY_COPY: Record<SessionBoardColumnId, string> = {
  needs_you: "Nothing needs you",
  working: "Nothing running",
  in_review: "No open pull requests",
  done: "Nothing finished yet",
};

export interface SessionsBoardColumnProps {
  column: SessionsBoardColumnView;
  currentSessionId?: string | null;
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
  onOpen: (session: SessionBoardPayload) => void;
  /** Done only: the column folds into a 44 px rail behind a disclosure button. */
  collapsible?: { collapsed: boolean; onToggle: () => void };
}

function ColumnGlyph({ id }: { id: SessionBoardColumnId }) {
  if (id === "in_review") {
    return <GitPullRequest aria-hidden="true" className="size-3.5 shrink-0 text-info" />;
  }
  const state = id === "needs_you" ? "attention" : id === "working" ? "running" : "done";
  return <StateGlyph state={state} />;
}

function sessionCountLabel(count: number): string {
  return `${count} ${count === 1 ? "session" : "sessions"}`;
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
}: SessionsBoardColumnProps) {
  const bodyId = useId();
  const collapsed = collapsible?.collapsed === true;
  const count = column.count;
  const regionLabel =
    count === undefined ? column.label : `${column.label}, ${sessionCountLabel(count)}`;
  return (
    <section
      aria-label={regionLabel}
      className={cn(
        "flex min-h-0 w-74 shrink-0 flex-col overflow-hidden rounded-lg bg-sunken",
        collapsed && "w-11"
      )}
      data-collapsed={collapsed ? "true" : undefined}
      data-testid={`sessions-board-column-${column.id}`}
    >
      <header
        className={cn(
          "flex shrink-0 items-center gap-2 px-3 pt-3 pb-2",
          collapsed && "flex-col gap-2.5 px-0 py-2.5"
        )}
      >
        <ColumnGlyph id={column.id} />
        <h2
          className={cn(
            "text-item-title font-medium text-fg",
            collapsed && "[writing-mode:vertical-rl]"
          )}
        >
          {column.label}
        </h2>
        {count !== undefined && !column.loading ? (
          <span
            className="text-small-body text-subtle tabular-nums"
            data-testid={`sessions-board-column-count-${column.id}`}
          >
            {count}
          </span>
        ) : null}
        {collapsible ? (
          <Button
            aria-controls={bodyId}
            aria-expanded={!collapsed}
            aria-label={`${collapsed ? "Expand" : "Collapse"} ${column.label}`}
            className={cn("ml-auto aria-expanded:bg-transparent", collapsed && "order-first m-0")}
            data-testid={`sessions-board-column-toggle-${column.id}`}
            onClick={collapsible.onToggle}
            size="icon-xs"
            type="button"
            variant="quiet"
          >
            <ChevronRight
              className={cn(
                "transition-transform duration-fast ease-out motion-reduce:transition-none",
                !collapsed && "rotate-90"
              )}
            />
          </Button>
        ) : null}
      </header>
      <div
        className={cn(
          "flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-2 pt-1 pb-3",
          collapsed && "hidden"
        )}
        data-testid={`sessions-board-column-body-${column.id}`}
        id={bodyId}
      >
        <SessionsBoardColumnBody
          column={column}
          currentSessionId={currentSessionId}
          onOpen={onOpen}
          workspaceNameOf={workspaceNameOf}
        />
      </div>
    </section>
  );
}

function SessionsBoardColumnBody({
  column,
  currentSessionId,
  workspaceNameOf,
  onOpen,
}: Omit<SessionsBoardColumnProps, "collapsible">) {
  if (column.loading) {
    return SKELETON_KEYS.map(key => <SessionBoardCardSkeleton key={key} />);
  }
  if (column.failed) {
    return (
      <div
        className="flex flex-1 flex-col items-center justify-center gap-2.5 px-3 py-8 text-center text-small-body text-muted"
        data-testid={`sessions-board-column-error-${column.id}`}
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
  if (column.sessions.length === 0) {
    return (
      <div
        className="flex flex-1 items-center justify-center px-3 py-8 text-center text-small-body text-subtle"
        data-testid={`sessions-board-column-empty-${column.id}`}
      >
        {EMPTY_COPY[column.id]}
      </div>
    );
  }
  return (
    <>
      {column.sessions.map(session => (
        <SessionBoardCard
          current={session.id === currentSessionId}
          key={session.id}
          onOpen={onOpen}
          session={session}
          workspaceName={workspaceNameOf(session)}
        />
      ))}
      {column.hasMore ? (
        <SessionsBoardLoadMore
          count={column.count}
          loaded={column.sessions.length}
          loadingMore={column.loadingMore}
          onLoadMore={column.loadMore}
          testId={`sessions-board-load-more-${column.id}`}
        />
      ) : null}
    </>
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

function SessionBoardCardSkeleton() {
  return (
    <div
      aria-hidden="true"
      className="flex w-full shrink-0 flex-col gap-2.5 rounded-lg bg-card p-3 shadow-card"
      data-testid="sessions-board-card-skeleton"
    >
      <Skeleton className="h-2.5 w-[55%] rounded-xs" />
      <Skeleton className="h-3 w-[90%] rounded-xs" />
      <Skeleton className="h-2.5 w-2/5 rounded-xs" />
    </div>
  );
}

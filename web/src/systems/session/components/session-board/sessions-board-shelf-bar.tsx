import { Archive, ChevronRight, CircleSlash, Bookmark, type LucideIcon } from "@compozy/ui/icons";
import { use, useId, type ReactNode } from "react";

import { Button, SkeletonRegion, SkeletonText } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import {
  useSessionsBoardShelfSelection,
  type SessionsBoardShelfId,
} from "../../hooks/use-sessions-board-view-selection";
import type { SessionsBoardShelf } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionBoardCardSlot, slottedCard } from "./session-board-card-slot";
import { SessionsBoardLoadMore, SessionsBoardReadError } from "./sessions-board-column";
import { SESSIONS_BOARD_SHELF_BAR, SESSIONS_BOARD_SHELF_FRAME } from "./sessions-board-frame";

interface ShelfCopy {
  icon: LucideIcon;
  label: string;
  loading: string;
  failed: string;
}

const SHELF_COPY: Record<SessionsBoardShelfId, ShelfCopy> = {
  archive: {
    icon: Archive,
    label: "Archive",
    loading: "Loading archived sessions",
    failed: "Couldn't load archived sessions.",
  },
  later: {
    icon: Bookmark,
    label: "Later",
    loading: "Loading sessions in Later",
    failed: "Couldn't load sessions in Later.",
  },
};

export interface SessionsBoardShelfBarProps {
  /** Which bar: archived sessions, or the sessions parked in Later. */
  shelf: SessionsBoardShelfId;
  page: SessionsBoardShelf;
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
  onOpen: (session: SessionBoardPayload) => void;
  actions?: SessionLifecycleActionHandlers;
}

/**
 * Archived and parked sessions stay out of the columns but reachable (US-016):
 * a bottom disclosure bar each, collapsed by default and absent only once a
 * read confirms the bar is empty. A pending or failed first read keeps the bar,
 * so those sessions never silently disappear. Test ids read
 * `sessions-board-<shelf>-…`.
 */
export function SessionsBoardShelfBar({
  shelf,
  page,
  workspaceNameOf,
  onOpen,
  actions,
}: SessionsBoardShelfBarProps) {
  const BoardCard = slottedCard(use(SessionBoardCardSlot));
  const bodyId = useId();
  const copy = SHELF_COPY[shelf];
  const testId = `sessions-board-${shelf}`;
  const { open, toggle } = useSessionsBoardShelfSelection(
    shelf,
    page.sessions,
    page.fetching,
    page.failure !== null
  );
  if (page.loading) {
    return (
      <SessionsBoardShelfStatus copy={copy} testId={testId}>
        <SkeletonRegion label={copy.loading}>
          <SkeletonText className="w-4" />
        </SkeletonRegion>
      </SessionsBoardShelfStatus>
    );
  }
  if (page.failure === "initial") {
    return (
      <SessionsBoardShelfStatus alert copy={copy} testId={testId}>
        <CircleSlash aria-hidden="true" className="size-3.5 text-subtle" />
        <span>{copy.failed}</span>
        <Button
          className="ml-auto"
          data-testid={`${testId}-retry`}
          onClick={page.retry}
          size="sm"
          type="button"
          variant="quiet"
        >
          Retry
        </Button>
      </SessionsBoardShelfStatus>
    );
  }
  if (!page.count) return null;
  const ShelfIcon = copy.icon;
  return (
    <div className={SESSIONS_BOARD_SHELF_FRAME} data-testid={testId}>
      <button
        aria-controls={bodyId}
        aria-expanded={open}
        className={cn(
          SESSIONS_BOARD_SHELF_BAR,
          "outline-none hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-inset"
        )}
        data-testid={`${testId}-toggle`}
        onClick={toggle}
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-3.5 text-subtle transition-transform duration-fast ease-out motion-reduce:transition-none",
            open && "rotate-90"
          )}
        />
        <ShelfIcon aria-hidden="true" className="size-3.5 text-subtle" />
        <span>{copy.label}</span>
        <span className="text-subtle tabular-nums">{page.count}</span>
      </button>
      {open ? (
        <div
          className="flex items-start gap-2 overflow-x-auto px-4 pt-1 pb-4"
          data-testid={`${testId}-body`}
          id={bodyId}
        >
          {page.failure === "refresh" ? (
            <SessionsBoardReadError
              className="w-66"
              failure="refresh"
              onRetry={page.retry}
              retrying={page.retrying}
              testId={`sessions-board-read-error-${shelf}`}
            />
          ) : null}
          {page.sessions.map(session => (
            <div className="w-66 shrink-0" key={session.id}>
              <BoardCard
                actions={actions}
                archived={shelf === "archive"}
                onOpen={onOpen}
                session={session}
                workspaceName={workspaceNameOf(session)}
              />
            </div>
          ))}
          {page.failure === "more" ? (
            <SessionsBoardReadError
              className="w-66"
              failure="more"
              onRetry={page.retry}
              retrying={page.retrying}
              testId={`sessions-board-read-error-${shelf}`}
            />
          ) : page.hasMore ? (
            <SessionsBoardLoadMore
              count={page.count}
              loaded={page.sessions.length}
              loadingMore={page.loadingMore}
              onLoadMore={page.loadMore}
              testId={`sessions-board-load-more-${shelf}`}
            />
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

/**
 * The bar while its first read is pending or failed: no disclosure yet. A spacer holds the
 * chevron's place so the label does not move when the count arrives.
 */
function SessionsBoardShelfStatus({
  alert = false,
  copy,
  testId,
  children,
}: {
  alert?: boolean;
  copy: ShelfCopy;
  testId: string;
  children: ReactNode;
}) {
  const ShelfIcon = copy.icon;
  return (
    <div className={SESSIONS_BOARD_SHELF_FRAME} data-testid={testId}>
      <div
        className={SESSIONS_BOARD_SHELF_BAR}
        data-testid={`${testId}-status`}
        role={alert ? "alert" : undefined}
      >
        <span aria-hidden="true" className="size-3.5 shrink-0" />
        <ShelfIcon aria-hidden="true" className="size-3.5 text-subtle" />
        <span>{copy.label}</span>
        {children}
      </div>
    </div>
  );
}

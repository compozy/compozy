import { Archive, ChevronRight, CircleSlash } from "lucide-react";
import { use, useId, type ReactNode } from "react";

import { Button, SkeletonRegion, SkeletonText } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { useSessionsBoardArchiveSelection } from "../../hooks/use-sessions-board-selection";
import type { SessionsBoardView } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionBoardCardSlot, slottedCard } from "./session-board-card-slot";
import { SessionsBoardLoadMore, SessionsBoardReadError } from "./sessions-board-column";
import { SESSIONS_BOARD_ARCHIVE_BAR, SESSIONS_BOARD_ARCHIVE_FRAME } from "./sessions-board-frame";

export interface SessionsBoardArchiveBarProps {
  archive: SessionsBoardView["archive"];
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
  onOpen: (session: SessionBoardPayload) => void;
  actions?: SessionLifecycleActionHandlers;
}

/**
 * Archived sessions stay out of the columns but reachable (US-016): a bottom
 * disclosure bar, collapsed by default and absent only once a read confirms
 * nothing is archived. A pending or failed first read keeps the bar, so
 * archived sessions never silently disappear.
 */
export function SessionsBoardArchiveBar({
  archive,
  workspaceNameOf,
  onOpen,
  actions,
}: SessionsBoardArchiveBarProps) {
  const BoardCard = slottedCard(use(SessionBoardCardSlot));
  const bodyId = useId();
  const { open, toggle } = useSessionsBoardArchiveSelection(
    archive.sessions,
    archive.fetching,
    archive.failure !== null
  );
  if (archive.loading) {
    return (
      <SessionsBoardArchiveStatus>
        <SkeletonRegion label="Loading archived sessions">
          <SkeletonText className="w-4" />
        </SkeletonRegion>
      </SessionsBoardArchiveStatus>
    );
  }
  if (archive.failure === "initial") {
    return (
      <SessionsBoardArchiveStatus alert>
        <CircleSlash aria-hidden="true" className="size-3.5 text-subtle" />
        <span>Couldn't load archived sessions.</span>
        <Button
          className="ml-auto"
          data-testid="sessions-board-archive-retry"
          onClick={archive.retry}
          size="sm"
          type="button"
          variant="quiet"
        >
          Retry
        </Button>
      </SessionsBoardArchiveStatus>
    );
  }
  if (!archive.count) return null;
  return (
    <div className={SESSIONS_BOARD_ARCHIVE_FRAME} data-testid="sessions-board-archive">
      <button
        aria-controls={bodyId}
        aria-expanded={open}
        className={cn(
          SESSIONS_BOARD_ARCHIVE_BAR,
          "outline-none hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-inset"
        )}
        data-testid="sessions-board-archive-toggle"
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
        <Archive aria-hidden="true" className="size-3.5 text-subtle" />
        <span>Archive</span>
        <span className="text-subtle tabular-nums">{archive.count}</span>
      </button>
      {open ? (
        <div
          className="flex items-start gap-2 overflow-x-auto px-4 pb-4"
          data-testid="sessions-board-archive-body"
          id={bodyId}
        >
          {archive.failure === "refresh" ? (
            <SessionsBoardReadError
              className="w-66"
              failure="refresh"
              onRetry={archive.retry}
              retrying={archive.retrying}
              testId="sessions-board-read-error-archive"
            />
          ) : null}
          {archive.sessions.map(session => (
            <div className="w-66 shrink-0" key={session.id}>
              <BoardCard
                actions={actions}
                archived
                onOpen={onOpen}
                session={session}
                workspaceName={workspaceNameOf(session)}
              />
            </div>
          ))}
          {archive.failure === "more" ? (
            <SessionsBoardReadError
              className="w-66"
              failure="more"
              onRetry={archive.retry}
              retrying={archive.retrying}
              testId="sessions-board-read-error-archive"
            />
          ) : archive.hasMore ? (
            <SessionsBoardLoadMore
              count={archive.count}
              loaded={archive.sessions.length}
              loadingMore={archive.loadingMore}
              onLoadMore={archive.loadMore}
              testId="sessions-board-load-more-archive"
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
function SessionsBoardArchiveStatus({
  alert = false,
  children,
}: {
  alert?: boolean;
  children: ReactNode;
}) {
  return (
    <div className={SESSIONS_BOARD_ARCHIVE_FRAME} data-testid="sessions-board-archive">
      <div
        className={SESSIONS_BOARD_ARCHIVE_BAR}
        data-testid="sessions-board-archive-status"
        role={alert ? "alert" : undefined}
      >
        <span aria-hidden="true" className="size-3.5 shrink-0" />
        <Archive aria-hidden="true" className="size-3.5 text-subtle" />
        <span>Archive</span>
        {children}
      </div>
    </div>
  );
}

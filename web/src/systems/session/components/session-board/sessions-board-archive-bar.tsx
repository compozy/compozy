import { Archive, ChevronRight, CircleSlash } from "lucide-react";
import { useId, useState, type ReactNode } from "react";

import { Button, Spinner } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionsBoardView } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionBoardCard } from "./session-board-card";
import { SessionsBoardLoadMore, SessionsBoardReadError } from "./sessions-board-column";

export interface SessionsBoardArchiveBarProps {
  archive: SessionsBoardView["archive"];
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
  onOpen: (session: SessionBoardPayload) => void;
}

const BAR_CLASS = "flex h-tabs-list w-full items-center gap-2 px-4 text-small-body text-muted";

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
}: SessionsBoardArchiveBarProps) {
  const [open, setOpen] = useState(false);
  const bodyId = useId();
  if (archive.loading) {
    return (
      <SessionsBoardArchiveStatus>
        <Spinner aria-label="Loading archived sessions" className="size-3.5" />
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
    <div className="shrink-0 border-t border-line bg-canvas" data-testid="sessions-board-archive">
      <button
        aria-controls={bodyId}
        aria-expanded={open}
        className={cn(
          BAR_CLASS,
          "outline-none hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-inset"
        )}
        data-testid="sessions-board-archive-toggle"
        onClick={() => setOpen(value => !value)}
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
              <SessionBoardCard
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

/** The bar while its first read is pending or failed: no disclosure yet. */
function SessionsBoardArchiveStatus({
  alert = false,
  children,
}: {
  alert?: boolean;
  children: ReactNode;
}) {
  return (
    <div className="shrink-0 border-t border-line bg-canvas" data-testid="sessions-board-archive">
      <div
        className={BAR_CLASS}
        data-testid="sessions-board-archive-status"
        role={alert ? "alert" : undefined}
      >
        <Archive aria-hidden="true" className="size-3.5 text-subtle" />
        <span>Archive</span>
        {children}
      </div>
    </div>
  );
}

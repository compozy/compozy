import { Archive, ChevronRight } from "lucide-react";
import { useId, useState } from "react";

import { cn } from "@/lib/utils";

import type { SessionsBoardView } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionBoardCard } from "./session-board-card";
import { SessionsBoardLoadMore } from "./sessions-board-column";

export interface SessionsBoardArchiveBarProps {
  archive: SessionsBoardView["archive"];
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
  onOpen: (session: SessionBoardPayload) => void;
}

/**
 * Archived sessions stay out of the columns but reachable (US-016): a bottom
 * disclosure bar, collapsed by default and absent when nothing is archived.
 */
export function SessionsBoardArchiveBar({
  archive,
  workspaceNameOf,
  onOpen,
}: SessionsBoardArchiveBarProps) {
  const [open, setOpen] = useState(false);
  const bodyId = useId();
  if (archive.count <= 0) return null;
  return (
    <div className="shrink-0 border-t border-line bg-canvas" data-testid="sessions-board-archive">
      <button
        aria-controls={bodyId}
        aria-expanded={open}
        className="flex h-tabs-list w-full items-center gap-2 px-4 text-small-body text-muted outline-none hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-inset"
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
          {archive.hasMore ? (
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

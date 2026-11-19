import { Pin } from "@compozy/ui/icons";
import { useId } from "react";

import { Icon } from "@compozy/ui";

import type {
  SessionListPinnedRead,
  SessionListViewModel,
} from "../../hooks/use-session-list-view";
import {
  rowDragged,
  rowDropEdge,
  useSessionListDnd,
  type SessionDropLayout,
} from "../../hooks/use-session-list-dnd";
import { shiftId } from "../../lib/session-folders";
import { buildSessionListThreads, type SessionThreadModel } from "../../lib/session-list-threads";
import {
  SESSION_LIST_PINNED_FOLD_KEY,
  SESSION_LIST_PINNED_LIMIT,
  SessionListPinnedSectionContext,
} from "../../lib/session-list-pinned";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import type { SessionFoldSet, SessionSubagentFolds } from "../../lib/session-sidebar-scope";
import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import type { SessionListTarget, SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionListCatalogNavigation } from "./session-list-controls";
import { GroupLoadFailure } from "./session-list-group-body";
import { SessionListGroupToggle } from "./session-list-group-toggle";
import { SessionListThread, type SessionThreadRootLayout } from "./session-list-thread";

export interface SessionListPinnedProps {
  view: SessionListViewModel;
  filtering: boolean;
  collapsedThreads: SessionFoldSet;
  subagentThreads: SessionSubagentFolds;
  currentSessionId?: string;
  onThreadBranchChange: SessionThreadBranchChange;
  onSelectSession: (session: SessionListTarget) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
}

/**
 * The Pinned section above every group, folder and row of the active list: the sessions the
 * operator pinned, in the operator's order (a new pin on top), each carrying its subtree nested
 * under it. Its group and folder keep their place for the unpin, which returns the whole subtree.
 * Pins reorder by drag or by Move up and Move down where the list reads one pair; they carry no
 * folder verbs. No pins, or a search that matches none, shows no section.
 */
export function SessionListPinned(props: SessionListPinnedProps) {
  const { view, filtering, testIdPrefix } = props;
  const headerId = useId();
  const pinned = view.pinned;
  if (!pinned || pinned.loading) return null;
  const testId = `${testIdPrefix}-pinned`;
  // A failed refresh keeps the pins it already shows; the paging row below offers Retry.
  if (pinned.failed && pinned.sessions.length === 0) {
    return <GroupLoadFailure retryTestId={`${testId}-retry`} onRetry={pinned.retry} />;
  }
  // A page emptied by an unpin keeps the section while other pages hold pins, so Previous stays.
  if (pinned.sessions.length === 0 && !pinned.more) return null;
  const collapsed = view.collapsedGroupKeys.has(SESSION_LIST_PINNED_FOLD_KEY);
  return (
    <section
      role="group"
      aria-labelledby={headerId}
      className="pb-1"
      data-collapsed={collapsed ? "true" : undefined}
      data-testid={testId}
    >
      <div className="flex min-h-7 min-w-0 items-center gap-1 rounded-sm">
        <SessionListGroupToggle
          collapsed={collapsed}
          count={sessionListPinnedCount(pinned, filtering)}
          identity={
            <span id={headerId} className="flex min-w-0 items-center gap-1.5 font-medium text-fg">
              <Icon as={Pin} size="sm" className="shrink-0 text-faint" aria-hidden="true" />
              Pinned
            </span>
          }
          onToggle={() => view.toggleGroup(SESSION_LIST_PINNED_FOLD_KEY)}
          testId={`${testId}-toggle`}
        />
      </div>
      {collapsed ? null : <SessionListPinnedRows {...props} pinned={pinned} />}
    </section>
  );
}

/**
 * The header count: a search narrows the rows, so a count would name what the search hides; past
 * the page, "100+". It counts pins: the sessions a pin carries list under it, uncounted.
 */
function sessionListPinnedCount(
  pinned: SessionListPinnedRead,
  filtering: boolean
): number | string | null {
  if (filtering) return null;
  return pinned.more
    ? `${SESSION_LIST_PINNED_LIMIT}+`
    : buildSessionListThreads(pinned.sessions, "").length;
}

function SessionListPinnedRows({
  view,
  pinned,
  collapsedThreads,
  subagentThreads,
  currentSessionId,
  onThreadBranchChange,
  onSelectSession,
  sessionActions,
  testIdPrefix,
  filtering,
}: SessionListPinnedProps & { pinned: SessionListPinnedRead }) {
  const threads = buildSessionListThreads(pinned.sessions, "");
  const ownerOf = view.aggregate ? view.ownerOf : undefined;
  const paged = pinned.more || pinned.failed;
  // A search can list a carried row as a root, so pins reorder only over the unfiltered section.
  const reorder = filtering ? undefined : pinned.reorder;
  const dnd = useSessionListDnd(reorder ? pinnedDropLayout(reorder) : undefined, {
    manual: true,
    reorderable: reorder !== undefined,
  });
  const layoutFor = sessionListPinnedLayout(threads, reorder, dnd);
  return (
    <SessionListPinnedSectionContext value>
      <SessionListBadgeContext value={view.badgeOf}>
        <div className="flex flex-col gap-0.5">
          {threads.map(thread => (
            <SessionListThread
              key={thread.session.id}
              session={thread.session}
              childSessions={thread.childSessions}
              currentSessionId={currentSessionId}
              owner={ownerOf?.(thread.session)}
              ownerOf={ownerOf}
              collapsed={collapsedThreads.has(thread.session.id)}
              subagentThreads={subagentThreads}
              onThreadBranchChange={onThreadBranchChange}
              onSelectSession={onSelectSession}
              sessionActions={sessionActions}
              testIdPrefix={testIdPrefix}
              layoutFor={layoutFor}
            />
          ))}
        </div>
      </SessionListBadgeContext>
      {pinned.catalog && paged ? <SessionListCatalogNavigation catalog={pinned.catalog} /> : null}
    </SessionListPinnedSectionContext>
  );
}

/** The Pinned section's drop verbs: a row dropped between pins reorders them, nothing else. */
function pinnedDropLayout(reorder: (sessionIds: readonly string[]) => void): SessionDropLayout {
  return {
    folders: [],
    moveSession: () => undefined,
    reorderFolders: () => undefined,
    reorderSessions: (_folderId, sessionIds) => reorder(sessionIds),
  };
}

/**
 * Each pin's drag source, drop edge and Move up and Move down; the rows a pin carries keep their
 * order under it and take no order verbs. Absent while the section cannot reorder.
 */
function sessionListPinnedLayout(
  threads: readonly SessionThreadModel[],
  reorder: ((sessionIds: readonly string[]) => void) | undefined,
  dnd: ReturnType<typeof useSessionListDnd>
): ((session: SessionPayload) => SessionThreadRootLayout | undefined) | undefined {
  if (!reorder) return undefined;
  // A carried row whose pin is outside this read stands as a root here; only pins reorder.
  const ids = threads.filter(thread => thread.session.pinned_at).map(thread => thread.session.id);
  const container = { folderId: null, ids };
  const pins = new Set(ids);
  return session => {
    if (!pins.has(session.id)) return undefined;
    const move = (delta: -1 | 1) => {
      const next = shiftId(ids, session.id, delta);
      return next ? () => reorder(next) : undefined;
    };
    return {
      // A pin files nowhere from Pinned: its folder verbs stay with its place in the list.
      layoutMenu: {
        folders: [],
        folderId: null,
        onMove: () => undefined,
        onMoveUp: move(-1),
        onMoveDown: move(1),
      },
      drag: dnd.sessionRow(session.id, container),
      dropEdge: rowDropEdge(dnd.indicator, session.id),
      dragging: rowDragged(dnd.dragging, session.id),
    };
  };
}

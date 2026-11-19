import { Pin } from "@compozy/ui/icons";
import { useId } from "react";

import { Icon } from "@compozy/ui";

import type {
  SessionListPinnedRead,
  SessionListViewModel,
} from "../../hooks/use-session-list-view";
import { buildSessionListThreads } from "../../lib/session-list-threads";
import {
  SESSION_LIST_PINNED_FOLD_KEY,
  SESSION_LIST_PINNED_LIMIT,
  SessionListPinnedSectionContext,
} from "../../lib/session-list-pinned";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import type { SessionFoldSet, SessionSubagentFolds } from "../../lib/session-sidebar-scope";
import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import type { SessionListTarget } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionListCatalogNavigation } from "./session-list-controls";
import { GroupLoadFailure } from "./session-list-group-body";
import { SessionListGroupToggle } from "./session-list-group-toggle";
import { SessionListThread } from "./session-list-thread";

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
 * operator pinned, oldest pin first, each on its own (its group and folder keep their place for
 * the unpin). Its rows are not drag sources and carry no folder or order verbs. No pins, or a
 * search that matches none, shows no section.
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
 * the page, "100+". It counts pins, not threads: a pinned child nests under its pinned parent yet
 * is its own pin.
 */
function sessionListPinnedCount(
  pinned: SessionListPinnedRead,
  filtering: boolean
): number | string | null {
  if (filtering) return null;
  return pinned.more ? `${SESSION_LIST_PINNED_LIMIT}+` : pinned.sessions.length;
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
}: SessionListPinnedProps & { pinned: SessionListPinnedRead }) {
  const threads = buildSessionListThreads(pinned.sessions, "");
  const ownerOf = view.aggregate ? view.ownerOf : undefined;
  const paged = pinned.more || pinned.failed;
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
            />
          ))}
        </div>
      </SessionListBadgeContext>
      {pinned.catalog && paged ? <SessionListCatalogNavigation catalog={pinned.catalog} /> : null}
    </SessionListPinnedSectionContext>
  );
}

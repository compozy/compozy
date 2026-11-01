import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import { MessagesSquare } from "@compozy/ui/icons";

import { Empty } from "@compozy/ui";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionRowSelection } from "../../hooks/use-session-selection";
import type { SessionThreadModel } from "../../lib/session-list-threads";
import type { SessionListTarget, SessionPayload } from "../../types";
import type { ProfileOwner, ProfileOwnerLabel } from "@/systems/profiles";
import {
  useSessionListDnd,
  sessionRowNestTarget,
  rowDropEdge,
  rowDragged,
} from "../../hooks/use-session-list-dnd";
import { sessionHasOwnPlacement } from "../../lib/session-folders";
import { SessionListThread } from "./session-list-thread";

export interface SessionListThreadListProps {
  threads: readonly SessionThreadModel[];
  collapsedThreads: ReadonlySet<string>;
  subagentThreads: ReadonlySet<string>;
  currentSessionId?: string;
  /** Resolves each row's owner. Absent in a scoped list. */
  ownerOf?: (session: ProfileOwnerLabel) => ProfileOwner;
  onThreadBranchChange: SessionThreadBranchChange;
  onSelectSession: (session: SessionListTarget) => void;
  /** Bulk selection; absent where the list is read-only (grouped lists). */
  selection?: SessionRowSelection;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  /** Shown in place of the threads when none survive the filter. */
  emptyMessage: string;
}

/** The single-workspace breadth: provenance threads in served order, or the empty state. */
export function SessionListThreadList({
  threads,
  collapsedThreads,
  subagentThreads,
  currentSessionId,
  ownerOf,
  onThreadBranchChange,
  onSelectSession,
  selection,
  sessionActions,
  testIdPrefix,
  emptyMessage,
}: SessionListThreadListProps) {
  const dnd = useSessionListDnd(undefined, { manual: false, reorderable: false });
  const nestParents = threads.flatMap(thread => [thread.session, ...thread.childSessions]);
  const roots = new Map(threads.map(thread => [thread.session.id, thread.session]));
  const threadActions = { ...sessionActions, nestParents };
  const rowLayout = (session: SessionPayload) => ({
    drag: dnd.sessionRow(
      session.id,
      { folderId: null, ids: [] },
      undefined,
      sessionRowNestTarget(roots, session, sessionActions.onAdopt),
      !sessionHasOwnPlacement(session)
    ),
    dropEdge: rowDropEdge(dnd.indicator, session.id),
    dragging: rowDragged(dnd.dragging, session.id),
  });
  return (
    <>
      {threads.map(thread => (
        <SessionListThread
          key={thread.session.id}
          session={thread.session}
          layoutFor={rowLayout}
          childSessions={thread.childSessions}
          currentSessionId={currentSessionId}
          owner={ownerOf?.(thread.session)}
          ownerOf={ownerOf}
          collapsed={collapsedThreads.has(thread.session.id)}
          subagentThreads={subagentThreads}
          onThreadBranchChange={onThreadBranchChange}
          onSelectSession={onSelectSession}
          selection={selection}
          sessionActions={threadActions}
          testIdPrefix={testIdPrefix}
        />
      ))}
      {threads.length === 0 ? (
        <Empty
          className="px-3 py-8"
          fill={false}
          icon={MessagesSquare}
          size="compact"
          title={emptyMessage}
        />
      ) : null}
    </>
  );
}

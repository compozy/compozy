import { MessagesSquare } from "lucide-react";

import { Empty } from "@compozy/ui";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionRowSelection } from "../../hooks/use-session-selection";
import type { SessionThreadModel } from "../../lib/session-list-threads";
import type { SessionPayload } from "../../types";
import type { ProfileOwner, ProfileOwnerLabel } from "@/systems/profiles";
import { SessionListThread } from "./session-list-thread";

export interface SessionListThreadListProps {
  threads: readonly SessionThreadModel[];
  collapsedThreads: ReadonlySet<string>;
  currentSessionId?: string;
  /** Resolves each row's owner. Absent in a scoped list. */
  ownerOf?: (session: ProfileOwnerLabel) => ProfileOwner;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  selection: SessionRowSelection;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  /** Shown in place of the threads when none survive the filter. */
  emptyMessage: string;
}

/** The single-workspace breadth: provenance threads in served order, or the empty state. */
export function SessionListThreadList({
  threads,
  collapsedThreads,
  currentSessionId,
  ownerOf,
  onToggleThread,
  onSelectSession,
  selection,
  sessionActions,
  testIdPrefix,
  emptyMessage,
}: SessionListThreadListProps) {
  return (
    <>
      {threads.map(thread => (
        <SessionListThread
          key={thread.session.id}
          session={thread.session}
          childSessions={thread.childSessions}
          currentSessionId={currentSessionId}
          owner={ownerOf?.(thread.session)}
          ownerOf={ownerOf}
          collapsed={collapsedThreads.has(thread.session.id)}
          onToggleThread={onToggleThread}
          onSelectSession={onSelectSession}
          selection={selection}
          sessionActions={sessionActions}
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

import { useState } from "react";

import { SearchInput } from "@compozy/ui";

import { getSessionDisplayTitle } from "../../lib/session-display-title";
import { buildSessionTree, filterThreadSessions } from "../../lib/session-hierarchy";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { emptyForScope } from "@/systems/profiles";

import { SessionListThread } from "./session-list-thread";
import { SessionListToolbar } from "./session-list-toolbar";
import { SessionListWorkspaceGroups } from "./session-list-workspace-groups";

interface SessionThreadModel {
  session: SessionPayload;
  childSessions: SessionPayload[];
}

export interface SessionListProps {
  sessions: readonly SessionPayload[];
  disconnected: boolean;
  collapsedThreadIds: readonly string[];
  currentSessionId?: string;
  /** Breadth, order, and the widened per-workspace groups. */
  view: SessionListViewModel;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  onNewSession: () => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  /** Rendered above the controls (the modal's eyebrow + close chrome). */
  header?: (visibleCount: number) => React.ReactNode;
}

/**
 * Shared sessions catalog body: this workspace, or every workspace.
 *
 * The narrow breadth lists the active workspace's complete catalog as
 * provenance threads; `All workspaces` widens to every workspace, grouped and
 * labelled, so no workspace can stall unnoticed. Ordering is served by the
 * daemon for the operator's chosen sort — this component renders the order it
 * receives rather than re-deciding it.
 *
 * A matching descendant keeps its full ancestor path visible, so a thread never
 * renders detached from its root.
 */
export function SessionList({
  sessions,
  disconnected,
  collapsedThreadIds,
  currentSessionId,
  view,
  onToggleThread,
  onSelectSession,
  onNewSession,
  sessionActions,
  testIdPrefix,
  header,
}: SessionListProps) {
  const [filter, setFilter] = useState("");
  const normalizedFilter = filter.trim().toLocaleLowerCase();
  const matchesFilter = (session: SessionPayload) => {
    if (normalizedFilter === "") return true;
    return (
      getSessionDisplayTitle(session).toLocaleLowerCase().includes(normalizedFilter) ||
      session.agent_name.toLocaleLowerCase().includes(normalizedFilter)
    );
  };

  const tree = buildSessionTree(sessions);
  const threads: SessionThreadModel[] = [];
  for (const root of tree.roots) {
    const childSessions = filterThreadSessions(root, tree.childrenByParent, matchesFilter);
    if (childSessions === null) continue;
    threads.push({ session: root, childSessions });
  }
  const visibleCount = threads.reduce(
    (count, thread) => count + 1 + thread.childSessions.length,
    0
  );
  const collapsedThreads = new Set(collapsedThreadIds);
  const allWorkspaces = view.scope === "all-workspaces";
  // Say what is empty AND for whom: an operator in Marketing needs to know the
  // list is empty in Marketing, not on the machine (US-009.EC-3).
  const emptyMessage =
    normalizedFilter !== ""
      ? "No sessions match."
      : view.archived
        ? "No archived sessions."
        : `${emptyForScope("sessions", view.scopeLabel)}.`;

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid={`${testIdPrefix}-content`}>
      {header?.(visibleCount)}
      <SessionListToolbar
        allWorkspaces={allWorkspaces}
        archived={view.archived}
        sort={view.sort}
        disabled={view.saving}
        onAllWorkspacesChange={next => view.setScope(next ? "all-workspaces" : "workspace")}
        onArchivedChange={view.setArchived}
        onSortChange={view.setSort}
        onNewSession={onNewSession}
        testIdPrefix={testIdPrefix}
      />
      <div className="px-3 pb-1.5">
        <SearchInput
          value={filter}
          onChange={setFilter}
          placeholder="Filter sessions…"
          aria-label="Filter sessions"
          containerClassName="min-w-0"
        />
      </div>
      {disconnected ? (
        <p
          className="mx-3 my-1 rounded-md border border-warning/30 bg-warning-tint px-2.5 py-2 text-small-body text-warning"
          role="status"
        >
          Session updates are unavailable. Cached sessions remain visible.
        </p>
      ) : null}
      <div
        className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5"
        data-scope={view.scope}
      >
        {allWorkspaces ? (
          <SessionListWorkspaceGroups
            groups={view.workspaceGroups}
            collapsedWorkspaceIds={view.collapsedWorkspaceIds}
            currentSessionId={currentSessionId}
            ownerOf={view.aggregate ? view.ownerOf : undefined}
            onToggleWorkspace={view.toggleWorkspace}
            onSelectSession={onSelectSession}
            sessionActions={sessionActions}
            testIdPrefix={testIdPrefix}
          />
        ) : (
          <>
            {threads.map(thread => (
              <SessionListThread
                key={thread.session.id}
                session={thread.session}
                childSessions={thread.childSessions}
                currentSessionId={currentSessionId}
                owner={view.aggregate ? view.ownerOf(thread.session) : undefined}
                ownerOf={view.aggregate ? view.ownerOf : undefined}
                collapsed={collapsedThreads.has(thread.session.id)}
                onToggleThread={onToggleThread}
                onSelectSession={onSelectSession}
                sessionActions={sessionActions}
                testIdPrefix={testIdPrefix}
              />
            ))}
            {threads.length === 0 ? (
              <p className="px-3 py-8 text-center text-small-body text-muted">{emptyMessage}</p>
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}

export type { SessionThreadModel };

import { useState } from "react";

import { SearchInput } from "@compozy/ui";

import { getSessionDisplayTitle } from "../../lib/session-display-title";
import { buildSessionTree, filterThreadSessions } from "../../lib/session-hierarchy";
import type { SessionListViewModel } from "../../hooks/use-session-list-view";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionListArchived } from "./session-list-archived";
import { SessionListGroup } from "./session-list-group";
import { SessionListThread } from "./session-list-thread";
import { SessionListToolbar } from "./session-list-toolbar";
import { SessionListWorkspaceGroups } from "./session-list-workspace-groups";

interface SessionThreadModel {
  session: SessionPayload;
  childSessions: SessionPayload[];
}

const RECENT_THREAD_LIMIT = 6;

export interface SessionListProps {
  sessions: readonly SessionPayload[];
  archivedSessions?: readonly SessionPayload[];
  archivedTotal?: number;
  disconnected: boolean;
  collapsedAgentIds: readonly string[];
  collapsedThreadIds: readonly string[];
  currentSessionId?: string;
  /** Scope, order, and the widened per-workspace groups. */
  view: SessionListViewModel;
  onToggleGroup: (agentName: string) => void;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  /** Rendered above the controls (the modal's eyebrow + close chrome). */
  header?: (visibleCount: number) => React.ReactNode;
  /** Rendered under the list (the sidebar's new-session action). */
  footer?: React.ReactNode;
}

/**
 * Shared sessions catalog body: a tri-state scope over provenance threads.
 *
 * `Recent` and `All` stay inside the active workspace; `All workspaces` widens
 * to every workspace, grouped and labelled, so no workspace can stall
 * unnoticed. Ordering is served by the daemon for the operator's chosen sort —
 * this component renders the order it receives rather than re-deciding it.
 *
 * A matching descendant keeps its full ancestor path visible, so a thread never
 * renders detached from its root.
 */
export function SessionList({
  sessions,
  archivedSessions = [],
  archivedTotal,
  disconnected,
  collapsedAgentIds,
  collapsedThreadIds,
  currentSessionId,
  view,
  onToggleGroup,
  onToggleThread,
  onSelectSession,
  sessionActions,
  testIdPrefix,
  header,
  footer,
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
  const filteredArchived = archivedSessions.filter(matchesFilter);
  const filteredArchivedTotal = normalizedFilter === "" ? archivedTotal : undefined;
  const byAgent = new Map<string, SessionThreadModel[]>();
  for (const thread of threads) {
    const current = byAgent.get(thread.session.agent_name) ?? [];
    current.push(thread);
    byAgent.set(thread.session.agent_name, current);
  }
  const collapsedAgents = new Set(collapsedAgentIds);
  const collapsedThreads = new Set(collapsedThreadIds);

  const renderThread = (thread: SessionThreadModel) => (
    <SessionListThread
      key={thread.session.id}
      session={thread.session}
      childSessions={thread.childSessions}
      currentSessionId={currentSessionId}
      collapsed={collapsedThreads.has(thread.session.id)}
      onToggleThread={onToggleThread}
      onSelectSession={onSelectSession}
      sessionActions={sessionActions}
      testIdPrefix={testIdPrefix}
    />
  );

  const visibleThreads = view.scope === "recent" ? threads.slice(0, RECENT_THREAD_LIMIT) : threads;

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid={`${testIdPrefix}-content`}>
      {header?.(visibleCount)}
      <SessionListToolbar
        scope={view.scope}
        sort={view.sort}
        disabled={view.saving}
        onScopeChange={view.setScope}
        onSortChange={view.setSort}
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
        {view.scope === "all-workspaces" ? (
          <SessionListWorkspaceGroups
            groups={view.workspaceGroups}
            collapsedWorkspaceIds={view.collapsedWorkspaceIds}
            currentSessionId={currentSessionId}
            onToggleWorkspace={view.toggleWorkspace}
            onSelectSession={onSelectSession}
            sessionActions={sessionActions}
            testIdPrefix={testIdPrefix}
          />
        ) : view.scope === "all" ? (
          <>
            {[...byAgent.entries()].map(([agentName, agentThreads]) => (
              <SessionListGroup
                key={agentName}
                agentName={agentName}
                threads={agentThreads}
                collapsed={collapsedAgents.has(agentName)}
                collapsedThreadIds={collapsedThreads}
                currentSessionId={currentSessionId}
                onToggleGroup={onToggleGroup}
                onToggleThread={onToggleThread}
                onSelectSession={onSelectSession}
                sessionActions={sessionActions}
                testIdPrefix={testIdPrefix}
              />
            ))}
            {byAgent.size === 0 ? (
              <p className="px-3 py-8 text-center text-small-body text-muted">No sessions match.</p>
            ) : null}
          </>
        ) : (
          <>
            {visibleThreads.map(renderThread)}
            {threads.length === 0 ? (
              <p className="px-3 py-8 text-center text-small-body text-muted">No sessions match.</p>
            ) : null}
          </>
        )}
      </div>
      <div className="shrink-0">
        {view.scope === "recent" ? (
          <SessionListArchived
            sessions={filteredArchived}
            total={filteredArchivedTotal}
            onSelectSession={onSelectSession}
            sessionActions={sessionActions}
            testIdPrefix={testIdPrefix}
          />
        ) : null}
        {footer}
      </div>
    </div>
  );
}

export type { SessionThreadModel };

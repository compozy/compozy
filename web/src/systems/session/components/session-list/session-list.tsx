import { ChevronLeft, ChevronRight } from "lucide-react";
import { useState } from "react";

import { Icon, SearchInput } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { getSessionDisplayTitle } from "../../lib/session-display-title";
import { buildSessionTree, filterThreadSessions } from "../../lib/session-hierarchy";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionListArchived } from "./session-list-archived";
import { SessionListGroup } from "./session-list-group";
import { SessionListThread } from "./session-list-thread";

type SessionListView = "recent" | "all";

interface SessionThreadModel {
  session: SessionPayload;
  childSessions: SessionPayload[];
}

export interface SessionListProps {
  sessions: readonly SessionPayload[];
  archivedSessions?: readonly SessionPayload[];
  archivedTotal?: number;
  disconnected: boolean;
  collapsedAgentIds: readonly string[];
  collapsedThreadIds: readonly string[];
  currentSessionId?: string;
  onToggleGroup: (agentName: string) => void;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  /** Rendered above the filter (the modal's eyebrow + close chrome). */
  header?: (visibleCount: number) => React.ReactNode;
  /** Rendered under the recent pane (the sidebar's new-session action). */
  footer?: React.ReactNode;
}

/**
 * Shared sessions catalog body: filterable recent ⇄ all panes with provenance
 * threads. A matching descendant keeps its full ancestor path visible so a
 * thread never renders detached from its root.
 */
export function SessionList({
  sessions,
  archivedSessions = [],
  archivedTotal,
  disconnected,
  collapsedAgentIds,
  collapsedThreadIds,
  currentSessionId,
  onToggleGroup,
  onToggleThread,
  onSelectSession,
  sessionActions,
  testIdPrefix,
  header,
  footer,
}: SessionListProps) {
  const [view, setView] = useState<SessionListView>("recent");
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
    threads.push({
      session: root,
      childSessions,
    });
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

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid={`${testIdPrefix}-content`}>
      {header?.(visibleCount)}
      <div className="px-3 py-1.5">
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
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <div
          className={cn(
            "flex h-full min-h-0 w-[200%] transition-transform duration-shell-slow ease-spring",
            view === "all" && "-translate-x-1/2"
          )}
          data-view={view}
        >
          <div className="flex h-full w-1/2 shrink-0 flex-col">
            <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5">
              {threads.slice(0, 6).map(renderThread)}
              {threads.length === 0 ? (
                <p className="px-3 py-8 text-center text-small-body text-muted">
                  No sessions match.
                </p>
              ) : null}
            </div>
            <div className="shrink-0">
              <SessionListArchived
                sessions={filteredArchived}
                total={filteredArchivedTotal}
                onSelectSession={onSelectSession}
                sessionActions={sessionActions}
                testIdPrefix={testIdPrefix}
              />
              <button
                type="button"
                className="flex w-full items-center justify-center gap-1.5 border-t border-line-soft px-2 py-2.5 text-small-body font-medium text-subtle transition-colors hover:bg-row-hover hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
                onClick={() => setView("all")}
              >
                Show all sessions
                <Icon as={ChevronRight} size="sm" />
              </button>
              {footer}
            </div>
          </div>
          <div className="h-full w-1/2 shrink-0 overflow-y-auto px-2.5 pb-3">
            <button
              type="button"
              className="mt-0.5 mb-1 flex w-full items-center gap-1.5 rounded-sm px-2 py-1 text-small-body font-medium text-subtle transition-colors hover:bg-row-hover hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
              aria-label="Back to recent sessions"
              onClick={() => setView("recent")}
            >
              <Icon as={ChevronLeft} size="sm" />
              Recent
            </button>
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
          </div>
        </div>
      </div>
    </div>
  );
}

export type { SessionThreadModel };

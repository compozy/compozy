import { ChevronRight, TriangleAlert } from "lucide-react";

import { Button, Icon } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { WorkspaceSessionGroup } from "../../hooks/use-workspace-session-groups";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionPayload } from "../../types";
import type { ProfileOwner, ProfileOwnerLabel } from "@/systems/profiles";
import { emptyForScope } from "@/systems/profiles";
import { SessionListRow } from "./session-list-row";

function GroupNote({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-6 items-center gap-2 px-3 py-1 text-micro text-subtle">
      {children}
    </div>
  );
}

function GroupBody({
  group,
  currentSessionId,
  ownerOf,
  scopeLabel,
  archived,
  onSelectSession,
  sessionActions,
  testIdPrefix,
}: {
  group: WorkspaceSessionGroup;
  currentSessionId?: string;
  ownerOf?: (session: ProfileOwnerLabel) => ProfileOwner;
  scopeLabel: string | null;
  archived: boolean;
  onSelectSession: (session: SessionPayload) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
}) {
  if (group.failed) {
    // One unreachable workspace never blanks the list — it states what happened
    // inside its own group and offers the one useful action.
    return (
      <GroupNote>
        <Icon as={TriangleAlert} size="sm" className="text-warning" />
        Couldn&rsquo;t load sessions
        <Button
          variant="ghost"
          size="sm"
          className="ml-auto"
          data-testid={`${testIdPrefix}-workspace-${group.workspaceId}-retry`}
          onClick={group.retry}
        >
          Retry
        </Button>
      </GroupNote>
    );
  }
  if (group.loading) return <GroupNote>Loading sessions…</GroupNote>;
  if (group.sessions.length === 0) {
    return (
      <GroupNote>
        {emptyForScope(archived ? "archived sessions" : "sessions", scopeLabel)}.
      </GroupNote>
    );
  }
  return (
    <div className="flex flex-col gap-0.5 pl-2.5">
      {group.sessions.map(session => (
        <SessionListRow
          key={session.id}
          session={session}
          owner={ownerOf?.(session)}
          current={session.id === currentSessionId}
          onSelect={() => onSelectSession(session)}
          sessionActions={sessionActions}
          showActions={false}
          testIdPrefix={testIdPrefix}
        />
      ))}
    </div>
  );
}

export interface SessionListWorkspaceGroupsProps {
  groups: readonly WorkspaceSessionGroup[];
  collapsedWorkspaceIds: ReadonlySet<string>;
  currentSessionId?: string;
  scopeLabel: string | null;
  archived: boolean;
  /** Resolves each row's owner. Absent in a scoped list — no tags there. */
  ownerOf?: (session: ProfileOwnerLabel) => ProfileOwner;
  onToggleWorkspace: (workspaceId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
}

/**
 * The all-workspaces scope: every workspace's sessions, grouped and labelled,
 * with each group loading, failing, and retrying on its own. Counts come from
 * the daemon's total rather than the loaded page, so a collapsed group never
 * understates what it holds.
 */
export function SessionListWorkspaceGroups({
  groups,
  collapsedWorkspaceIds,
  currentSessionId,
  ownerOf,
  scopeLabel,
  archived,
  onToggleWorkspace,
  onSelectSession,
  sessionActions,
  testIdPrefix,
}: SessionListWorkspaceGroupsProps) {
  if (groups.length === 0) {
    return (
      <p className="px-3 py-8 text-center text-small-body text-muted">
        {emptyForScope(archived ? "archived sessions" : "sessions", scopeLabel)}.
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-1">
      {groups.map(group => {
        const collapsed = collapsedWorkspaceIds.has(group.workspaceId);
        return (
          <section key={group.workspaceId} data-collapsed={collapsed ? "true" : undefined}>
            <button
              type="button"
              aria-expanded={!collapsed}
              data-testid={`${testIdPrefix}-workspace-${group.workspaceId}`}
              className="flex min-h-7 w-full items-center gap-2 rounded-sm px-2 py-1 text-left transition-colors hover:bg-row-hover focus-visible:shadow-focus-ring focus-visible:outline-none"
              onClick={() => onToggleWorkspace(group.workspaceId)}
            >
              <Icon
                as={ChevronRight}
                size="sm"
                className={cn("text-faint transition-transform", !collapsed && "rotate-90")}
              />
              <span className="min-w-0 flex-1 truncate text-small-body font-medium text-fg">
                {group.workspaceName}
              </span>
              <span className="font-mono text-micro text-faint">{group.total}</span>
            </button>
            {collapsed ? null : (
              <GroupBody
                group={group}
                currentSessionId={currentSessionId}
                ownerOf={ownerOf}
                scopeLabel={scopeLabel}
                archived={archived}
                onSelectSession={onSelectSession}
                sessionActions={sessionActions}
                testIdPrefix={testIdPrefix}
              />
            )}
          </section>
        );
      })}
    </div>
  );
}

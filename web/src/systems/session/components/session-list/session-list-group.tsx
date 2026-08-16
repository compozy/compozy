import { ChevronRight } from "lucide-react";

import { Icon } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionListThread } from "./session-list-thread";

export interface SessionListGroupProps {
  agentName: string;
  /** Thread roots owned by this agent; child sessions stay with their provenance thread. */
  threads: ReadonlyArray<{ session: SessionPayload; childSessions: SessionPayload[] }>;
  collapsed: boolean;
  collapsedThreadIds: ReadonlySet<string>;
  currentSessionId?: string;
  onToggleGroup: (agentName: string) => void;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
}

export function SessionListGroup({
  agentName,
  threads,
  collapsed,
  collapsedThreadIds,
  currentSessionId,
  onToggleGroup,
  onToggleThread,
  onSelectSession,
  sessionActions,
  testIdPrefix,
}: SessionListGroupProps) {
  const sessionCount = threads.reduce(
    (count, thread) => count + 1 + thread.childSessions.length,
    0
  );
  return (
    <section className="border-b border-line-soft py-1 last:border-b-0">
      <button
        type="button"
        className="flex min-h-7 w-full items-center gap-2 rounded-sm px-2 py-1 text-left hover:bg-row-hover focus-visible:shadow-focus-ring focus-visible:outline-none"
        aria-expanded={!collapsed}
        onClick={() => onToggleGroup(agentName)}
      >
        <span className="grid size-5 place-items-center rounded-sm bg-elevated font-mono text-micro font-semibold text-muted">
          {agentName.slice(0, 2).toUpperCase()}
        </span>
        <span className="min-w-0 flex-1 truncate text-small-body font-medium text-fg-strong">
          {agentName}
        </span>
        <span className="font-mono text-micro text-faint">{sessionCount}</span>
        <Icon
          as={ChevronRight}
          size="sm"
          className={cn("text-faint transition-transform", !collapsed && "rotate-90")}
        />
      </button>
      <div
        inert={collapsed}
        className={cn(
          "grid transition-[grid-template-rows] duration-base",
          collapsed ? "grid-rows-[0fr]" : "grid-rows-[1fr]"
        )}
      >
        <div className="min-h-0 overflow-hidden pl-2">
          {threads.map(thread => (
            <SessionListThread
              key={thread.session.id}
              session={thread.session}
              childSessions={thread.childSessions}
              currentSessionId={currentSessionId}
              collapsed={collapsedThreadIds.has(thread.session.id)}
              onToggleThread={onToggleThread}
              onSelectSession={onSelectSession}
              sessionActions={sessionActions}
              testIdPrefix={testIdPrefix}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

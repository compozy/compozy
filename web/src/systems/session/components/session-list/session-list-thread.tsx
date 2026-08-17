import { ChevronRight } from "lucide-react";

import { Icon } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { childSessionSignalTone } from "../../lib/session-hierarchy";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionListRow } from "./session-list-row";

const CHILD_SIGNAL_CLASS: Record<string, string> = {
  danger: "bg-danger",
  warning: "bg-warning",
  accent: "bg-accent",
};

export interface SessionListThreadProps {
  session: SessionPayload;
  childSessions: readonly SessionPayload[];
  currentSessionId?: string;
  collapsed: boolean;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
}

/**
 * One provenance thread: the root session row plus its child sessions nested
 * behind a hairline connector. Collapsing keeps the most urgent child state
 * visible on the toggle so an escalation never hides behind the fold.
 */
export function SessionListThread({
  session,
  childSessions,
  currentSessionId,
  collapsed,
  onToggleThread,
  onSelectSession,
  sessionActions,
  testIdPrefix,
}: SessionListThreadProps) {
  const rootRow = (
    <SessionListRow
      session={session}
      current={session.id === currentSessionId}
      onSelect={() => onSelectSession(session)}
      sessionActions={sessionActions}
      testIdPrefix={testIdPrefix}
      trailing={
        childSessions.length > 0 ? (
          <ThreadToggle
            sessionId={session.id}
            childCount={childSessions.length}
            collapsed={collapsed}
            signalTone={collapsed ? childSessionSignalTone(childSessions) : null}
            onToggleThread={onToggleThread}
            testIdPrefix={testIdPrefix}
          />
        ) : undefined
      }
    />
  );
  if (childSessions.length === 0) return rootRow;

  return (
    <div data-testid={`${testIdPrefix}-thread-${session.id}`}>
      {rootRow}
      <div
        inert={collapsed}
        className={cn(
          "grid transition-[grid-template-rows] duration-base",
          collapsed ? "grid-rows-[0fr]" : "grid-rows-[1fr]"
        )}
      >
        <div className="relative min-h-0 overflow-hidden pl-5 before:absolute before:top-0.5 before:bottom-1 before:left-2.75 before:w-px before:bg-line-strong">
          {childSessions.map(child => (
            <SessionListRow
              key={child.id}
              session={child}
              current={child.id === currentSessionId}
              onSelect={() => onSelectSession(child)}
              sessionActions={sessionActions}
              testIdPrefix={testIdPrefix}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

function ThreadToggle({
  sessionId,
  childCount,
  collapsed,
  signalTone,
  onToggleThread,
  testIdPrefix,
}: {
  sessionId: string;
  childCount: number;
  collapsed: boolean;
  signalTone: string | null;
  onToggleThread: (sessionId: string) => void;
  testIdPrefix: string;
}) {
  return (
    <button
      type="button"
      className="flex items-center gap-1 rounded-full px-1.5 py-0.5 font-mono text-micro text-faint tabular-nums transition-colors hover:bg-elevated hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
      aria-expanded={!collapsed}
      aria-label={`Toggle ${childCount} child ${childCount === 1 ? "session" : "sessions"}`}
      data-testid={`${testIdPrefix}-thread-toggle-${sessionId}`}
      onClick={() => onToggleThread(sessionId)}
    >
      <Icon
        as={ChevronRight}
        size="sm"
        className={cn("transition-transform", !collapsed && "rotate-90")}
      />
      {childCount}
      {signalTone !== null ? (
        <span
          className={cn("size-[5px] rounded-full", CHILD_SIGNAL_CLASS[signalTone])}
          aria-hidden="true"
        />
      ) : null}
    </button>
  );
}

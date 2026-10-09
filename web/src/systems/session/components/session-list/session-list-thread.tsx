import type { SessionRowSelection } from "../../hooks/use-session-selection";
import { ChevronRight } from "lucide-react";

import { Icon, StateGlyph } from "@compozy/ui";

import { cn } from "@/lib/utils";

import {
  childSessionSignalState,
  isSubagentSession,
  type ChildSessionSignalState,
} from "../../lib/session-hierarchy";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { ProfileOwner, ProfileOwnerLabel } from "@/systems/profiles";
import { SessionListRow } from "./session-list-row";

export interface SessionListThreadProps {
  session: SessionPayload;
  /** The root row's owner tag, supplied only in aggregate mode. */
  owner?: ProfileOwner;
  /** Resolves each child row's owner. Absent in a scoped list. */
  ownerOf?: (session: ProfileOwnerLabel) => ProfileOwner;
  childSessions: readonly SessionPayload[];
  currentSessionId?: string;
  collapsed: boolean;
  onToggleThread: (sessionId: string) => void;
  onSelectSession: (session: SessionPayload) => void;
  selection?: SessionRowSelection;
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
  owner,
  ownerOf,
  childSessions,
  currentSessionId,
  collapsed,
  onToggleThread,
  onSelectSession,
  selection,
  sessionActions,
  testIdPrefix,
}: SessionListThreadProps) {
  // A revealed subagent session is not a plain child: it adds no toggle count and
  // never folds away while the operator is viewing it (nav VC-02).
  const foldable = childSessions.filter(child => !isSubagentSession(child));
  const folded = collapsed && foldable.length > 0;
  const rootRow = (
    <SessionListRow
      session={session}
      owner={owner}
      current={session.id === currentSessionId}
      onSelect={() => onSelectSession(session)}
      selection={selection}
      sessionActions={sessionActions}
      testIdPrefix={testIdPrefix}
      trailing={
        foldable.length > 0 ? (
          <ThreadToggle
            sessionId={session.id}
            childCount={foldable.length}
            collapsed={folded}
            childSignal={folded ? childSessionSignalState(foldable) : null}
            onToggleThread={onToggleThread}
            testIdPrefix={testIdPrefix}
          />
        ) : undefined
      }
    />
  );
  if (childSessions.length === 0) return rootRow;

  const childRow = (child: SessionPayload) => (
    <SessionListRow
      key={child.id}
      session={child}
      owner={ownerOf?.(child)}
      current={child.id === currentSessionId}
      onSelect={() => onSelectSession(child)}
      selection={selection}
      sessionActions={sessionActions}
      testIdPrefix={testIdPrefix}
    />
  );
  const revealed = childSessions.filter(isSubagentSession);

  return (
    <div data-testid={`${testIdPrefix}-thread-${session.id}`}>
      {rootRow}
      {foldable.length > 0 ? (
        <div
          inert={folded}
          className={cn(
            "grid transition-[grid-template-rows] duration-base",
            folded ? "grid-rows-[0fr]" : "grid-rows-[1fr]"
          )}
        >
          <div className={cn(CHILD_RAIL, "min-h-0 overflow-hidden")}>{foldable.map(childRow)}</div>
        </div>
      ) : null}
      {/* The viewed subagent sits outside the fold: collapsing plain children never hides it. */}
      {revealed.length > 0 ? <div className={CHILD_RAIL}>{revealed.map(childRow)}</div> : null}
    </div>
  );
}

/** Child rows indent behind the thread's hairline connector. */
const CHILD_RAIL =
  "relative pl-5 before:absolute before:top-0.5 before:bottom-1 before:left-2.75 before:w-px before:bg-line-strong";

function ThreadToggle({
  sessionId,
  childCount,
  collapsed,
  childSignal,
  onToggleThread,
  testIdPrefix,
}: {
  sessionId: string;
  childCount: number;
  collapsed: boolean;
  childSignal: ChildSessionSignalState | null;
  onToggleThread: (sessionId: string) => void;
  testIdPrefix: string;
}) {
  return (
    <button
      type="button"
      className="flex items-center gap-1 rounded-full px-1.5 py-0.5 font-mono text-micro text-faint tabular-nums transition-colors hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
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
      {childSignal !== null ? <StateGlyph size="sm" state={childSignal} /> : null}
    </button>
  );
}

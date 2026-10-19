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
import { SessionListRow, type SessionListRowProps } from "./session-list-row";

/** Folder, order, and drag wiring that only a thread's root row carries. */
export type SessionThreadRootLayout = Pick<
  SessionListRowProps,
  "layoutMenu" | "drag" | "dropEdge" | "dragging"
>;

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
  rootLayout?: SessionThreadRootLayout;
}

/**
 * One provenance thread: the root session row plus its child sessions, each
 * hanging off the root's rail by a tree elbow — the connector nested tool calls
 * use. Collapsing keeps the most urgent child state visible on the toggle so an
 * escalation never hides behind the fold.
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
  rootLayout,
}: SessionListThreadProps) {
  // A revealed subagent session is not a plain child: it adds no toggle count and
  // never folds away while the operator is viewing it (nav VC-02).
  const foldable = childSessions.filter(child => !isSubagentSession(child));
  const folded = collapsed && foldable.length > 0;
  const rootRow = (
    <SessionListRow
      {...rootLayout}
      session={session}
      // Context rules make a subagent a root only when its ancestry was deleted.
      orphanedSubagent={isSubagentSession(session)}
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

  const childRow = (child: SessionPayload) => {
    const childOwner = ownerOf?.(child);
    return (
      <div key={child.id} className={CHILD_NODE}>
        <SessionListRow
          session={child}
          // A child in its root's profile repeats nothing; only a different owner is tagged.
          owner={childOwner && childOwner.id !== owner?.id ? childOwner : undefined}
          current={child.id === currentSessionId}
          onSelect={() => onSelectSession(child)}
          selection={selection}
          sessionActions={sessionActions}
          testIdPrefix={testIdPrefix}
        />
      </div>
    );
  };
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

/** Child rows indent past the root's status mark, whose center the rail continues. */
const CHILD_RAIL = "relative pl-5";

/**
 * Each child's tree elbow ( |_ ): from the rail under the root's mark into the
 * child's own mark on its title line. The rail continues past a child only while
 * a sibling follows, so the last child closes the branch.
 */
const CHILD_NODE =
  "relative before:pointer-events-none before:absolute before:-top-0.5 before:-left-1.75 before:h-4.5 before:w-3 before:rounded-bl-md before:border-b before:border-l before:border-line-strong after:pointer-events-none after:absolute after:top-3.5 after:-bottom-0.5 after:-left-1.75 after:w-px after:bg-line-strong last:after:hidden";

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
      className="inline-flex h-5 items-center gap-1 rounded-pill px-1.5 font-mono text-micro text-muted tabular-nums transition-colors duration-fast hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
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

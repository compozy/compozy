import { memo } from "react";

import type { SessionRowSelection } from "../../hooks/use-session-selection";

import { childSessionSignalState, isSubagentSession } from "../../lib/session-hierarchy";
import { sessionSubagentCounts } from "../../lib/session-subagent-summary";
import {
  sessionThreadSegment,
  type SessionThreadBranchChange,
  type SessionThreadSegment,
} from "../../lib/session-thread-branch";
import type { SessionListTarget, SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { ProfileOwner, ProfileOwnerLabel } from "@/systems/profiles";
import { SessionListBranch } from "./session-list-branch";
import { BRANCH_NODE, BRANCH_RAIL } from "./session-list-branch-rail";
import { SessionListBranchToggle } from "./session-list-branch-toggle";
import { SessionListRow, type SessionListRowProps } from "./session-list-row";
import { SessionListSubagentBranch } from "./session-list-subagent-branch";
import type { SessionSubagentFolds } from "../../lib/session-sidebar-scope";

/** Folder, order, and drag wiring a row carries (folder and order verbs on roots only). */
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
  /** The root's child sessions are folded away. */
  collapsed: boolean;
  /** Sessions whose subagent list is open. */
  subagentThreads: SessionSubagentFolds;
  onThreadBranchChange: SessionThreadBranchChange;
  onSelectSession: (session: SessionListTarget) => void;
  selection?: SessionRowSelection;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  /** Each row's folder, order, and drag wiring, root and children alike. */
  layoutFor?: (session: SessionPayload) => SessionThreadRootLayout | undefined;
}

/**
 * One provenance thread: the root session row and its branch — the child
 * sessions or the subagents, one list at a time — each row hanging off the
 * root's rail by a tree elbow. A folded branch keeps the most urgent child
 * state on its toggle so an escalation never hides behind the fold.
 */
function SessionListThreadView({
  session,
  owner,
  ownerOf,
  childSessions,
  currentSessionId,
  collapsed,
  subagentThreads,
  onThreadBranchChange,
  onSelectSession,
  selection,
  sessionActions,
  testIdPrefix,
  layoutFor,
}: SessionListThreadProps) {
  // A revealed subagent session is not a plain child: it adds no toggle count and
  // never folds away while the operator is viewing it (nav VC-02).
  const foldable = childSessions.filter(child => !isSubagentSession(child));
  const revealed = childSessions.filter(isSubagentSession);
  const branch = (
    <SessionListBranchRow
      session={session}
      childSessions={foldable}
      collapsed={collapsed}
      subagentThreads={subagentThreads}
      onThreadBranchChange={onThreadBranchChange}
      currentSessionId={currentSessionId}
      onSelectSession={onSelectSession}
      testIdPrefix={testIdPrefix}
      row={trailing => (
        <SessionListRow
          {...layoutFor?.(session)}
          session={session}
          // Context rules make a subagent a root only when its ancestry was deleted.
          orphanedSubagent={isSubagentSession(session)}
          owner={owner}
          current={session.id === currentSessionId}
          onSelect={() => onSelectSession(session)}
          selection={selection}
          sessionActions={sessionActions}
          testIdPrefix={testIdPrefix}
          trailing={trailing}
        />
      )}
      childRow={child => {
        const childOwner = ownerOf?.(child);
        return (
          <div key={child.id} className={BRANCH_NODE}>
            <SessionListBranchRow
              session={child}
              childSessions={[]}
              collapsed
              subagentThreads={subagentThreads}
              onThreadBranchChange={onThreadBranchChange}
              currentSessionId={currentSessionId}
              onSelectSession={onSelectSession}
              testIdPrefix={testIdPrefix}
              row={trailing => (
                <SessionListRow
                  {...layoutFor?.(child)}
                  session={child}
                  // A child in its root's profile repeats nothing; only a different owner is tagged.
                  owner={childOwner && childOwner.id !== owner?.id ? childOwner : undefined}
                  inheritsProject={(child.workspace_id ?? "") === (session.workspace_id ?? "")}
                  current={child.id === currentSessionId}
                  onSelect={() => onSelectSession(child)}
                  selection={selection}
                  sessionActions={sessionActions}
                  testIdPrefix={testIdPrefix}
                  trailing={trailing}
                />
              )}
            />
          </div>
        );
      }}
    />
  );
  if (childSessions.length === 0 && sessionSubagentCounts(session) === null) return branch;

  return (
    <div data-testid={`${testIdPrefix}-thread-${session.id}`}>
      {branch}
      {/* The viewed subagent sits outside the fold: folding the branch never hides it. */}
      {revealed.length > 0 ? (
        <div className={BRANCH_RAIL}>
          {revealed.map(child => (
            <div key={child.id} className={BRANCH_NODE}>
              <SessionListBranchRow
                session={child}
                childSessions={[]}
                collapsed
                subagentThreads={subagentThreads}
                onThreadBranchChange={onThreadBranchChange}
                currentSessionId={currentSessionId}
                onSelectSession={onSelectSession}
                testIdPrefix={testIdPrefix}
                row={trailing => (
                  <SessionListRow
                    {...layoutFor?.(child)}
                    session={child}
                    inheritsProject={(child.workspace_id ?? "") === (session.workspace_id ?? "")}
                    current={child.id === currentSessionId}
                    onSelect={() => onSelectSession(child)}
                    selection={selection}
                    sessionActions={sessionActions}
                    testIdPrefix={testIdPrefix}
                    trailing={trailing}
                  />
                )}
              />
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

/**
 * Toggling one branch must not re-render its siblings: a thread renders again
 * only when its own rows, flags, or wiring change. The open-subagent set is
 * shared by the list, so only its members inside this thread count.
 */
export const SessionListThread = memo(SessionListThreadView, (prev, next) => {
  const keys = new Set([...Object.keys(prev), ...Object.keys(next)]);
  for (const key of keys as Set<keyof SessionListThreadProps>) {
    if (key === "subagentThreads" || key === "owner") continue;
    if (prev[key] !== next[key]) return false;
  }
  return (
    sameOwner(prev.owner, next.owner) &&
    [next.session, ...next.childSessions].every(
      row =>
        prev.subagentThreads.has(row.id) === next.subagentThreads.has(row.id) &&
        prev.subagentThreads.whenChildless?.has(row.id) ===
          next.subagentThreads.whenChildless?.has(row.id)
    )
  );
});

// Aggregate lists resolve a fresh owner object per render; equal fields are the same tag.
function sameOwner(a: ProfileOwner | undefined, b: ProfileOwner | undefined): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  return (
    a.id === b.id &&
    a.name === b.name &&
    a.color === b.color &&
    a.icon === b.icon &&
    a.emoji === b.emoji &&
    a.archived === b.archived
  );
}

/**
 * A session row with its branch toggle and the fold beneath it. A root row
 * may unfold child sessions or subagents; a child row only its subagents.
 */
function SessionListBranchRow({
  session,
  childSessions,
  collapsed,
  subagentThreads,
  onThreadBranchChange,
  currentSessionId,
  onSelectSession,
  testIdPrefix,
  row,
  childRow,
}: {
  session: SessionPayload;
  childSessions: readonly SessionPayload[];
  collapsed: boolean;
  subagentThreads: SessionSubagentFolds;
  onThreadBranchChange: SessionThreadBranchChange;
  currentSessionId: string | undefined;
  onSelectSession: (session: SessionListTarget) => void;
  testIdPrefix: string;
  row: (trailing: React.ReactNode) => React.ReactNode;
  childRow?: (child: SessionPayload) => React.ReactNode;
}) {
  const subagentCounts = sessionSubagentCounts(session);
  const segment = sessionThreadSegment({
    hasChildren: childSessions.length > 0,
    hasSubagents: subagentCounts !== null,
    collapsed,
    subagentsOpen:
      subagentThreads.has(session.id) ||
      (childSessions.length === 0 && subagentThreads.whenChildless?.has(session.id) === true),
  });
  if (childSessions.length === 0 && subagentCounts === null) return row(undefined);

  const branchId = `${testIdPrefix}-branch-${session.id}`;
  const list = (shown: SessionThreadSegment) =>
    shown === "children" ? (
      <div className={BRANCH_RAIL}>{childSessions.map(child => childRow?.(child))}</div>
    ) : (
      <div className={BRANCH_RAIL}>
        <SessionListSubagentBranch
          workspaceId={session.workspace_id ?? ""}
          sessionId={session.id}
          total={subagentCounts?.total ?? 0}
          currentSessionId={currentSessionId}
          onSelectSession={onSelectSession}
          testIdPrefix={testIdPrefix}
        />
      </div>
    );

  return (
    <>
      {row(
        <SessionListBranchToggle
          session={session}
          childCount={childSessions.length}
          childSignal={
            segment !== "children" && childSessions.length > 0
              ? childSessionSignalState(childSessions)
              : null
          }
          subagentCounts={subagentCounts}
          segment={segment}
          onChange={next => onThreadBranchChange(session.id, next)}
          branchId={branchId}
          testIdPrefix={testIdPrefix}
        />
      )}
      <div id={branchId}>
        <SessionListBranch segment={segment} render={list} />
      </div>
    </>
  );
}

import type { QueryClient } from "@tanstack/react-query";

import type { SessionPayload } from "../types";
import { sessionDescendantsOptions } from "./query-options";
import { isSubagentSession } from "./session-hierarchy";
import { sessionsRunningProcesses } from "./session-processes";

export type SessionTreeAction = "archive" | "delete";

/** Each target's child sessions, recursively, as the daemon reports them. */
export type SessionTrees = ReadonlyMap<string, readonly SessionPayload[]>;

/** The child sessions a confirmation offers to include, and the answer so far. */
export interface SessionChildrenChoice {
  count: number;
  included: boolean;
  /** Running processes the choice adds to the targets' own while included. */
  includedProcessCount: number;
  onIncludedChange: (included: boolean) => void;
}

export interface SessionChildrenScope {
  /** Children the choice decides on, without targets or repeats. */
  members: readonly SessionPayload[];
  /** A selected target another selected target's tree already includes, to its root target. */
  coveredBy: ReadonlyMap<string, string>;
}

/**
 * What an include-children answer changes. Subagents always follow an archived
 * parent and an archived child has nothing left to archive, so neither is a
 * choice there; delete leaves both behind unless asked.
 */
export function sessionChildrenScope(
  action: SessionTreeAction,
  targets: readonly Pick<SessionPayload, "id">[],
  trees: SessionTrees
): SessionChildrenScope {
  const targetIds = new Set(targets.map(target => target.id));
  const members = new Map<string, SessionPayload>();
  const covered = new Set<string>();
  for (const target of targets) {
    for (const child of trees.get(target.id) ?? []) {
      if (targetIds.has(child.id)) covered.add(child.id);
      else if (action === "delete" || !(isSubagentSession(child) || child.archived_at)) {
        members.set(child.id, child);
      }
    }
  }
  const coveredBy = new Map<string, string>();
  for (const target of targets) {
    if (covered.has(target.id)) continue;
    for (const child of trees.get(target.id) ?? []) {
      if (covered.has(child.id)) coveredBy.set(child.id, target.id);
    }
  }
  return { members: [...members.values()], coveredBy };
}

/** Targets whose tree another target includes run last, so their root can cover them. */
export function sessionTreeBatchOrder<T extends Pick<SessionPayload, "id">>(
  targets: readonly T[],
  coveredBy: ReadonlyMap<string, string>
): T[] {
  return [
    ...targets.filter(target => !coveredBy.has(target.id)),
    ...targets.filter(target => coveredBy.has(target.id)),
  ];
}

/** A failed read offers no children, so the verb keeps its single-session meaning. */
export async function readSessionTrees(
  queryClient: QueryClient,
  sessions: readonly SessionPayload[]
): Promise<SessionTrees> {
  const reads = await Promise.allSettled(
    sessions.map(session =>
      queryClient.fetchQuery({ ...sessionDescendantsOptions(session), staleTime: 0 })
    )
  );
  const trees = new Map<string, readonly SessionPayload[]>();
  reads.forEach((read, index) => {
    if (read.status === "fulfilled") trees.set(sessions[index]!.id, read.value);
  });
  return trees;
}

export function sessionChildrenChoice(
  scope: SessionChildrenScope,
  included: boolean,
  onIncludedChange: (included: boolean) => void
): SessionChildrenChoice | undefined {
  if (scope.members.length === 0) return undefined;
  return {
    count: scope.members.length,
    included,
    includedProcessCount: included ? sessionsRunningProcesses(scope.members) : 0,
    onIncludedChange,
  };
}

export function childSessionCountLabel(count: number): string {
  return `${count} child ${count === 1 ? "session" : "sessions"}`;
}

/** COPY.md: the include-children confirmation when nothing else needs an answer. */
export function sessionArchiveChildrenCopy(sessions: number, children: number) {
  const label = childSessionCountLabel(children);
  return sessions === 1
    ? {
        title: "Archive this session?",
        description: `This session has ${label}.`,
        confirmLabel: "Archive session",
      }
    : {
        title: `Archive ${sessions} sessions?`,
        description: `These sessions have ${label}.`,
        confirmLabel: `Archive ${sessions} sessions`,
      };
}

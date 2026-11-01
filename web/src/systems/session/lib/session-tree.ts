import { queryOptions, type QueryClient } from "@tanstack/react-query";

import { listSessionDescendants } from "../adapters/session-archive-api";
import type { SessionPayload } from "../types";
import { sessionKeys } from "./query-keys";
import { readCurrentSessions } from "./session-batch";
import { isSubagentSession } from "./session-hierarchy";
import { sessionsRunningProcesses } from "./session-processes";

/** The target's whole subtree from the daemon: a loaded catalog page cannot count it. */
export function sessionDescendantsOptions(
  session: Pick<SessionPayload, "id" | "workspace_id" | "profile_name">
) {
  return queryOptions({
    queryKey: sessionKeys.descendants(session.id, session.profile_name),
    queryFn: ({ signal }) =>
      listSessionDescendants(session.workspace_id ?? "", session.id, session.profile_name, signal),
    staleTime: 0,
  });
}

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
  /** Everything a yes reaches, subagents below a member included, for its process count. */
  reached: readonly SessionPayload[];
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
  const reached = new Map<string, SessionPayload>();
  const covered = new Set<string>();
  for (const target of targets) {
    for (const child of trees.get(target.id) ?? []) {
      if (targetIds.has(child.id)) {
        covered.add(child.id);
        continue;
      }
      if (action === "delete" || !child.archived_at) reached.set(child.id, child);
      if (action === "delete" || !(isSubagentSession(child) || child.archived_at)) {
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
  return { members: [...members.values()], reached: [...reached.values()], coveredBy };
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

/** A failed read rejects: without the count the verb cannot offer the operator's choice. */
export async function readSessionTrees(
  queryClient: QueryClient,
  sessions: readonly SessionPayload[]
): Promise<SessionTrees> {
  const reads = await Promise.all(
    sessions.map(session =>
      queryClient.fetchQuery({ ...sessionDescendantsOptions(session), staleTime: 0 })
    )
  );
  return new Map(reads.map((descendants, index) => [sessions[index]!.id, descendants]));
}

/** Fresh targets, then their trees: a stop has no children to offer. */
export async function readSessionChoiceTargets(
  queryClient: QueryClient,
  action: "stop" | "archive",
  sessions: readonly SessionPayload[]
) {
  const current = await readCurrentSessions(queryClient, sessions);
  const trees: SessionTrees =
    action === "archive" ? await readSessionTrees(queryClient, current.sessions) : new Map();
  return { current, trees };
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
    includedProcessCount: included ? sessionsRunningProcesses(scope.reached) : 0,
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

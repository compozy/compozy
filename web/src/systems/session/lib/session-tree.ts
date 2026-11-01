import type { QueryClient } from "@tanstack/react-query";

import type { SessionPayload } from "../types";
import { sessionDescendantsOptions } from "./query-options";
import { readCurrentSessions } from "./session-batch";
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
  /** The count may be stale: a yes waits for the read to settle. */
  pending: boolean;
  /** Present once that read failed, so the wait has a way out. */
  onRetryRead?: () => void;
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

/** Each root's tree or its read error: a partial map would read as "no children". */
export async function readSessionTrees(
  queryClient: QueryClient,
  sessions: readonly SessionPayload[]
) {
  const reads = await Promise.allSettled(
    sessions.map(async session => {
      const options = sessionDescendantsOptions(session);
      // An already-running read may predate the click; let it settle before refreshing.
      const existing = queryClient
        .getQueryCache()
        .find({ queryKey: options.queryKey, exact: true });
      if (existing?.state.fetchStatus === "fetching") await existing.promise?.catch(() => {});
      return queryClient.fetchQuery({ ...options, staleTime: 0 });
    })
  );
  const trees = new Map<string, readonly SessionPayload[]>();
  const errors = new Map<string, unknown>();
  reads.forEach((read, index) => {
    const id = sessions[index]!.id;
    if (read.status === "fulfilled") trees.set(id, read.value);
    else errors.set(id, read.reason);
  });
  return { trees, errors };
}

/**
 * Fresh targets, then their trees; a stop has no children to offer. A root whose tree cannot be
 * read joins the read errors, so it fails on its own without its children going unasked.
 */
export async function readSessionChoiceTargets(
  queryClient: QueryClient,
  action: "stop" | "archive",
  sessions: readonly SessionPayload[]
) {
  const current = await readCurrentSessions(queryClient, sessions);
  if (action === "stop") return { current, trees: new Map() as SessionTrees };
  const { trees, errors } = await readSessionTrees(queryClient, current.sessions);
  if (sessions.length === 1 && errors.size > 0) throw errors.values().next().value;
  return {
    current: {
      sessions: current.sessions.filter(session => !errors.has(session.id)),
      errors: new Map([...current.errors, ...errors]),
    },
    trees,
  };
}

export function sessionChildrenChoice(
  scope: SessionChildrenScope,
  included: boolean,
  onIncludedChange: (included: boolean) => void,
  read: { pending: boolean; onRetry?: () => void } = { pending: false }
): SessionChildrenChoice | undefined {
  if (scope.members.length === 0) return undefined;
  return {
    count: scope.members.length,
    included,
    includedProcessCount: included ? sessionsRunningProcesses(scope.reached) : 0,
    pending: read.pending,
    ...(read.onRetry ? { onRetryRead: read.onRetry } : {}),
    onIncludedChange,
  };
}

export function childSessionCountLabel(count: number): string {
  return `${count} child ${count === 1 ? "session" : "sessions"}`;
}

/** The batch position and skip rule a tree opt-in adds to archive and delete. */
export function sessionBatchCoverage(
  action: string,
  sessions: readonly SessionPayload[],
  trees?: SessionTrees
): ReadonlyMap<string, string> {
  return trees && (action === "archive" || action === "delete")
    ? sessionChildrenScope(action, sessions, trees).coveredBy
    : new Map<string, string>();
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

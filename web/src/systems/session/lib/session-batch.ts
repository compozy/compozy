import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import { settleBounded } from "@/lib/settle-bounded";
import type { QueryClient } from "@tanstack/react-query";
import type { SessionPayload } from "../types";
import { sessionScopedDetailOptions } from "./query-options";
import {
  sessionComputerOf,
  sessionRowKeyFor,
  withSessionComputer,
} from "./session-row-ref";

export interface SessionBatchResult {
  id: string;
  status: "pending" | "running" | "done" | "failed";
  error?: string;
  /** Present when the target runs on a remote computer, for per-computer failure reporting (R12). */
  computerId?: ComputerId;
}

export type SessionBatchAction =
  | "mark-seen"
  | "make-top-level"
  | "move-under"
  | "stop"
  | "archive"
  | "unarchive"
  | "delete"
  | "later"
  | "back-from-later";

/** The per-session failure line when the daemon's error carries no text. */
export function sessionBatchFailureMessage(action: SessionBatchAction): string {
  if (action === "mark-seen") return "Failed to mark session as seen.";
  if (action === "make-top-level") return "Failed to make session top-level.";
  if (action === "move-under") return "Failed to move session.";
  if (action === "later") return "Failed to move session to Later.";
  if (action === "back-from-later") return "Failed to bring session back from Later.";
  return `Failed to ${action} session.`;
}

interface SessionBatchOptions {
  action: SessionBatchAction;
  ids: readonly string[];
  previous: readonly SessionBatchResult[];
  /** Related sessions share a lane so recursive lifecycle mutations cannot race. */
  sessions?: readonly SessionPayload[];
  coveredBy?: ReadonlyMap<string, string>;
  execute: (id: string) => Promise<unknown>;
  onProgress: (results: readonly SessionBatchResult[]) => void;
}

/** Independent session trees run at bounded concurrency, preserving per-row receipts and retries. */
export async function runSessionBatch({
  action,
  ids,
  previous,
  sessions = [],
  coveredBy = new Map(),
  execute,
  onProgress,
}: SessionBatchOptions): Promise<SessionBatchResult[]> {
  const targets = new Set(ids);
  const results: SessionBatchResult[] =
    previous.length > 0
      ? previous.map(result =>
          targets.has(result.id) ? { id: result.id, status: "pending" } : result
        )
      : ids.map(id => ({ id, status: "pending" }));
  /** Publish fresh snapshots so observers never receive the mutable working array. */
  const update = (result: SessionBatchResult) => {
    const index = results.findIndex(current => current.id === result.id);
    results[index] = result;
    onProgress([...results]);
  };
  onProgress([...results]);
  const byId = new Map(sessions.map(session => [sessionRowKeyFor(session), session]));
  const computerOf = (id: string): ComputerId | undefined => {
    const session = byId.get(id);
    if (!session) return undefined;
    const computer = sessionComputerOf(session);
    return isRemoteComputerId(computer) ? computer : undefined;
  };
  const lanes = new Map<string, string[]>();
  for (const id of ids) {
    const session = byId.get(coveredBy.get(id) ?? id);
    const computer = session ? sessionComputerOf(session) : "local";
    const root = session?.lineage?.root_session_id || (coveredBy.get(id) ?? id);
    const key = JSON.stringify([computer, session?.workspace_id, session?.profile_id, root]);
    const lane = lanes.get(key) ?? [];
    lane.push(id);
    lanes.set(key, lane);
  }
  await settleBounded([...lanes.values()], 3, async lane => {
    for (const id of lane) {
      const computerId = computerOf(id);
      const computerField = computerId ? { computerId } : {};
      update({ id, status: "running", ...computerField });
      try {
        await execute(id);
        update({ id, status: "done", ...computerField });
      } catch (error) {
        update({
          id,
          status: "failed",
          ...computerField,
          error:
            error instanceof Error && error.message
              ? error.message
              : sessionBatchFailureMessage(action),
        });
      }
    }
  });
  return results;
}

export async function readCurrentSessions(
  queryClient: QueryClient,
  sessions: readonly SessionPayload[],
  allowPartial = false
) {
  const reads = await Promise.allSettled(
    sessions.map(async session => {
      const computerId = sessionComputerOf(session);
      const options = isRemoteComputerId(computerId)
        ? sessionScopedDetailOptions(session.id, { profile: session.profile_name }, { computerId })
        : sessionScopedDetailOptions(session.id, { profile: session.profile_name });
      // An already-running read may predate the click; let it settle before refreshing.
      const existing = queryClient
        .getQueryCache()
        .find({ queryKey: options.queryKey, exact: true });
      if (existing?.state.fetchStatus === "fetching") await existing.promise?.catch(() => {});
      const fresh = await queryClient.fetchQuery({ ...options, staleTime: 0 });
      return withSessionComputer(fresh, computerId);
    })
  );

  const current: SessionPayload[] = [];
  const errors = new Map<string, unknown>();
  reads.forEach((read, index) => {
    if (read.status === "fulfilled") current.push(read.value);
    else errors.set(sessionRowKeyFor(sessions[index]!), read.reason);
  });
  if (!allowPartial && sessions.length === 1 && errors.size > 0) throw errors.values().next().value;
  return { sessions: current, errors };
}

/** Every session a running batch, own mutation, or Later action is working on. */
export function lifecyclePendingSessionIds(state: {
  batching: boolean;
  batchResults: readonly SessionBatchResult[];
  ownPending: boolean;
  mutationSessionId: string | undefined;
  laterSessionIds: readonly string[];
}): readonly string[] {
  if (state.batching) return runningSessionIds(state.batchResults);
  if (state.ownPending) return state.mutationSessionId ? [state.mutationSessionId] : [];
  return state.laterSessionIds;
}

/** The targets a parallel batch is executing now; queued and settled ones are excluded. */
export function runningSessionIds(results: readonly SessionBatchResult[]): readonly string[] {
  return results.flatMap(result => (result.status === "running" ? [result.id] : []));
}

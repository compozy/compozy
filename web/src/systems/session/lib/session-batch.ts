import type { QueryClient } from "@tanstack/react-query";
import type { SessionPayload } from "../types";
import { sessionScopedDetailOptions } from "./query-options";

export interface SessionBatchResult {
  id: string;
  status: "pending" | "running" | "done" | "failed";
  error?: string;
}

interface SessionBatchOptions {
  action: "stop" | "archive" | "unarchive" | "delete";
  ids: readonly string[];
  previous: readonly SessionBatchResult[];
  execute: (id: string) => Promise<unknown>;
  onProgress: (results: readonly SessionBatchResult[]) => void;
}

/** Sequential mutations preserve daemon ordering and per-row progress, including retries. */
export async function runSessionBatch({
  action,
  ids,
  previous,
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
  for (const id of ids) {
    update({ id, status: "running" });
    try {
      await execute(id);
      update({ id, status: "done" });
    } catch (error) {
      update({
        id,
        status: "failed",
        error:
          error instanceof Error && error.message ? error.message : `Failed to ${action} session.`,
      });
    }
  }
  return results;
}

export async function readCurrentSessions(
  queryClient: QueryClient,
  sessions: readonly SessionPayload[]
) {
  const reads = await Promise.allSettled(
    sessions.map(async session => {
      const options = sessionScopedDetailOptions(session.id, { profile: session.profile_name });
      // An already-running read may predate the click; let it settle before refreshing.
      const existing = queryClient
        .getQueryCache()
        .find({ queryKey: options.queryKey, exact: true });
      if (existing?.state.fetchStatus === "fetching") await existing.promise?.catch(() => {});
      return queryClient.fetchQuery({ ...options, staleTime: 0 });
    })
  );

  const current: SessionPayload[] = [];
  const errors = new Map<string, unknown>();
  reads.forEach((read, index) => {
    if (read.status === "fulfilled") current.push(read.value);
    else errors.set(sessions[index]!.id, read.reason);
  });
  if (sessions.length === 1 && errors.size > 0) throw errors.values().next().value;
  return { sessions: current, errors };
}

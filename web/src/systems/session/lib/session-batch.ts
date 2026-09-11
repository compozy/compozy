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

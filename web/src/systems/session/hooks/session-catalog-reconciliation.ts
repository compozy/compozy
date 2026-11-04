import type { QueryClient, QueryKey } from "@tanstack/react-query";

import { notificationKeys } from "@/systems/notifications";

import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionByIdReads } from "../lib/session-query-invalidation";
import { queryKeyHasScope } from "../lib/session-query-policy";
import { isLiveSessionState } from "../lib/session-stream-liveness";
import type { SessionCatalogEventPayload, SessionPayload } from "../types";

/**
 * Re-reads the session detail, then wakes a stopped session's mounted transcript.
 *
 * A catalog upsert is the only signal for a transcript entry appended after the
 * session stopped (the post-stop discard marker): the transcript stream has already
 * ended on the terminal frame and a stopped session never polls. The decision is
 * taken from the authoritative detail read, never from the cached state — the wake
 * can arrive while the cache still says active/stopping although the daemon has
 * already stopped — so the transcript is re-read only after the detail invalidation
 * settles as non-live. Live sessions keep the live tail as the transcript's sole
 * owner, and an unknown state is left to the live tail's own detail read.
 */
async function refreshSessionDetail(
  queryClient: QueryClient,
  payload: SessionCatalogEventPayload,
  isOpen: () => boolean
): Promise<void> {
  const detailKey = sessionKeys.detail(payload.workspace_id, payload.session_id);
  await queryClient.invalidateQueries(
    {
      predicate: ({ queryKey }) =>
        queryKey.length === detailKey.length && queryKeyHasScope(queryKey, detailKey),
    },
    { cancelRefetch: false }
  );
  if (!isOpen() || payload.kind !== "upserted") return;
  const state = queryClient.getQueryData<SessionPayload>(detailKey)?.state;
  if (state === undefined || isLiveSessionState(state)) return;
  const transcriptKey = sessionKeys.transcript(payload.workspace_id, payload.session_id);
  await queryClient.invalidateQueries({
    predicate: ({ queryKey }) =>
      queryKey.length === transcriptKey.length && queryKeyHasScope(queryKey, transcriptKey),
  });
}

/** Let a session read finish, then reconcile any wake that arrived during it. */
export function createSessionReadReconciler(queryClient: QueryClient) {
  const pending = new Map<string, SessionCatalogEventPayload | undefined>();
  const running = new Set<string>();
  let closed = false;
  const flush = async (sessionId: string) => {
    try {
      while (!closed && pending.has(sessionId)) {
        const payload = pending.get(sessionId);
        pending.delete(sessionId);
        const byIdKey = sessionKeys.byIdRoot(sessionId);
        const detailKey = payload ? sessionKeys.detail(payload.workspace_id, sessionId) : undefined;
        const processKey = payload
          ? sessionKeys.processes(payload.workspace_id, sessionId)
          : undefined;
        const linkedWorkKey = payload
          ? sessionKeys.linkedWork(payload.workspace_id, sessionId)
          : undefined;
        // A read already in progress can predate this wake. Joining it is safe
        // only if one trailing read observes the change after that read settles.
        const cache = queryClient.getQueryCache();
        const reads = cache
          .findAll({
            predicate: ({ queryKey }) =>
              queryKeyHasScope(queryKey, byIdKey) ||
              (processKey !== undefined && queryKeyHasScope(queryKey, processKey)) ||
              (linkedWorkKey !== undefined && queryKeyHasScope(queryKey, linkedWorkKey)) ||
              (detailKey !== undefined &&
                queryKey.length === detailKey.length &&
                queryKeyHasScope(queryKey, detailKey)),
          })
          .flatMap(query =>
            query.state.fetchStatus === "fetching" && query.promise ? [query.promise] : []
          );
        if (reads.length > 0) {
          pending.set(sessionId, payload);
          await Promise.allSettled(reads);
          continue;
        }
        await Promise.all([
          ...(payload ? [refreshSessionDetail(queryClient, payload, () => !closed)] : []),
          ...(processKey
            ? [queryClient.invalidateQueries({ queryKey: processKey }, { cancelRefetch: false })]
            : []),
          ...(linkedWorkKey
            ? [queryClient.invalidateQueries({ queryKey: linkedWorkKey }, { cancelRefetch: false })]
            : []),
          invalidateSessionByIdReads(queryClient, sessionId, { cancelRefetch: false }),
        ]);
      }
    } finally {
      running.delete(sessionId);
    }
  };
  return {
    wake(sessionId: string, payload?: SessionCatalogEventPayload) {
      if (closed) return;
      // Location reads are independent of detail/transcript reconciliation.
      if (payload?.execution_location_changed === true) {
        const queryKey = sessionKeys.fileSearchRoot(payload.workspace_id, sessionId);
        void queryClient.cancelQueries({ queryKey }).then(() => {
          if (!closed) return queryClient.invalidateQueries({ queryKey });
        });
      }
      // Upserts carry a transcript obligation that later layout/attention wakes cannot erase.
      const queued = pending.get(sessionId);
      if (queued?.kind !== "upserted" && (payload || !pending.has(sessionId))) {
        pending.set(sessionId, payload);
      }
      if (running.has(sessionId)) return;
      running.add(sessionId);
      void flush(sessionId);
    },
    close() {
      closed = true;
      pending.clear();
    },
  };
}

interface CatalogChange {
  workspace_id: string;
  profile_name?: string;
  kind?: string;
}

function catalogReadAffected(key: QueryKey, change: CatalogChange | undefined): boolean {
  if (!change || !queryKeyHasScope(key, sessionKeys.lists())) return true;
  const workspace = change.workspace_id || "@user";
  const keyedWorkspace = typeof key[2] === "string" ? key[2] : "";
  if (keyedWorkspace && keyedWorkspace !== workspace) return false;
  const family = key[3];
  if (family === "folders")
    return !change.profile_name || !key[4] || key[4] === change.profile_name;
  const scope = (typeof family === "object" ? family : key[4]) as
    | { profile?: string; all_profiles?: boolean; workspace_ids?: string[] }
    | undefined;
  if (!scope || typeof scope !== "object") return true;
  if (
    change.profile_name &&
    scope.profile &&
    !scope.all_profiles &&
    scope.profile !== change.profile_name
  )
    return false;
  return !scope.workspace_ids?.length || scope.workspace_ids.includes(workspace);
}

// One wake window per stream, independent of the number of lifecycle/attention frames.
// An in-flight read is allowed to finish; cancelling it on every frame amplified
// one background transition into repeated full-catalog walks.
export function createCatalogReconciler(queryClient: QueryClient) {
  const workspaces = new Set<string>();
  const pending = new Set<string>();
  const changes = new Map<string, CatalogChange | undefined>();
  let lastRead = -Infinity;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let focused = true;
  const visible = () =>
    focused && (typeof document === "undefined" || document.visibilityState !== "hidden");
  const flush = () => {
    timer = undefined;
    if ((workspaces.size === 0 && pending.size === 0) || !visible()) return;
    lastRead = Date.now();
    const affected = [...changes.values()];
    changes.clear();
    const matches = (query: { queryKey: QueryKey }) =>
      affected.some(change => catalogReadAffected(query.queryKey, change));
    const hashes = new Set(pending);
    pending.clear();
    const prefixes = [
      ...Array.from(workspaces, workspace => sessionKeys.workspaceLists(workspace)),
      ...(workspaces.size > 0
        ? [
            sessionKeys.attentionSummary(),
            notificationKeys.attentionRoot(),
            sessionKeys.processesLeftRunning(),
          ]
        : []),
    ];
    workspaces.clear();
    for (const queryKey of prefixes) {
      for (const query of queryClient.getQueryCache().findAll({ queryKey })) {
        if (matches(query)) hashes.delete(query.queryHash);
      }
    }
    // A retry retains only blocked identities. Healthy siblings require a new wake.
    const predicate = (query: {
      queryHash: string;
      state: { status: string; errorUpdatedAt: number; fetchStatus: string };
      getObserversCount: () => number;
    }) => {
      const blocked =
        query.getObserversCount() > 0 &&
        (query.state.fetchStatus === "fetching" ||
          (query.state.status === "error" && Date.now() - query.state.errorUpdatedAt < 30_000));
      if (blocked) pending.add(query.queryHash);
      return !blocked;
    };
    for (const queryKey of prefixes) {
      void queryClient.invalidateQueries(
        { queryKey, predicate: query => matches(query) && predicate(query) },
        { cancelRefetch: false }
      );
    }
    if (hashes.size > 0) {
      void queryClient.invalidateQueries(
        { predicate: query => hashes.has(query.queryHash) && predicate(query) },
        { cancelRefetch: false }
      );
    }
    schedule();
  };
  const schedule = () => {
    if (timer !== undefined || !visible() || (workspaces.size === 0 && pending.size === 0)) return;
    const delay = Math.max(0, 5_000 - (Date.now() - lastRead));
    if (delay === 0) flush();
    else timer = setTimeout(flush, delay);
  };
  const onVisibility = () => schedule();
  const onFocus = () => {
    focused = true;
    schedule();
  };
  const onBlur = () => {
    focused = false;
  };
  if (typeof window !== "undefined") {
    window.addEventListener("focus", onFocus);
    window.addEventListener("blur", onBlur);
  }
  if (typeof document !== "undefined") document.addEventListener("visibilitychange", onVisibility);
  return {
    wake(workspace?: string, change?: CatalogChange) {
      changes.set(JSON.stringify([workspace, change?.profile_name, change?.kind]), change);
      workspaces.add("");
      if (workspace !== undefined) workspaces.add(workspace || "@user");
      else
        for (const query of queryClient
          .getQueryCache()
          .findAll({ queryKey: sessionKeys.lists() })) {
          if (typeof query.queryKey[2] === "string") workspaces.add(query.queryKey[2]);
        }
      schedule();
    },
    close() {
      if (typeof window !== "undefined") {
        window.removeEventListener("focus", onFocus);
        window.removeEventListener("blur", onBlur);
      }
      if (timer !== undefined) clearTimeout(timer);
      if (typeof document !== "undefined")
        document.removeEventListener("visibilitychange", onVisibility);
    },
  };
}

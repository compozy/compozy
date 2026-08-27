/**
 * View model for the inspector's Calls tab.
 *
 * Four reads, and the split between them is the point: two paged row lists —
 * `caller=` for what this session asked, `child_session_id=` for what it was
 * asked — and two `limit=1` count probes over the same filters. Counts key apart
 * from rows, so paging one direction never re-reads the other and a badge
 * refresh never evicts a page the operator is reading.
 *
 * Nothing here filters in the browser: both directions are server-side questions
 * with server-side answers, which is what lets a section honestly read "247"
 * while showing 25 rows.
 *
 * The fifth read answers a different question — *does the counterpart still
 * exist?* Retention prunes sessions while their call records survive, so a row
 * can name a session that can no longer be opened. Every call in one session's
 * panel shares one governed root, so one complete `root=` catalog read covers
 * every counterpart in both directions at once.
 */
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  CALLS_PANEL_PAGE_SIZE,
  callsListOptions,
  useAgentCommsScope,
  useCallCount,
  type CallDirectionSection,
  type CallPayload,
} from "@/systems/agent-comms";

import { sessionsCompleteListOptions } from "../lib/query-options";
import { useSession } from "./use-sessions";

export interface SessionCallsPanelModel {
  made: CallDirectionSection;
  received: CallDirectionSection;
  /** Counterparts retention has already removed. Empty until proven. */
  prunedSessionIds: ReadonlySet<string>;
}

const NO_PRUNED_SESSIONS: ReadonlySet<string> = new Set<string>();

/**
 * Which counterparts are gone.
 *
 * Fail-open by construction: a session is called pruned only once the catalog
 * for its root is loaded, complete, and not errored. A transport blip must read
 * as "not yet known", never as "deleted" — the same rule
 * `use-loop-node-session-availability` follows.
 */
function prunedFrom(
  made: readonly CallPayload[],
  received: readonly CallPayload[],
  known: ReadonlySet<string> | null
): ReadonlySet<string> {
  if (!known) return NO_PRUNED_SESSIONS;
  const pruned = new Set<string>();
  for (const call of made) {
    const child = call.child_session_id ?? "";
    if (child !== "" && !known.has(child)) pruned.add(child);
  }
  for (const call of received) {
    const caller = call.caller.id;
    if (caller !== "" && !known.has(caller)) pruned.add(caller);
  }
  return pruned;
}

export function useSessionCallsPanel(
  sessionId: string,
  liveDataEnabled = true
): SessionCallsPanelModel {
  const scope = useAgentCommsScope();
  const enabled = sessionId !== "" && liveDataEnabled;
  const session = useSession(sessionId);
  // Fail-open: a session that has not named its root yet is treated as its own
  // root rather than waiting on the first call row to arrive.
  const root = session.data?.lineage?.root_session_id ?? sessionId;

  const made = useInfiniteQuery(
    callsListOptions(
      scope,
      { caller: sessionId, limit: CALLS_PANEL_PAGE_SIZE },
      liveDataEnabled,
      enabled
    )
  );
  const received = useInfiniteQuery(
    callsListOptions(
      scope,
      { child_session_id: sessionId, limit: CALLS_PANEL_PAGE_SIZE },
      liveDataEnabled,
      enabled
    )
  );
  const madeTotal = useCallCount(scope, { caller: sessionId }, { live: liveDataEnabled, enabled });
  const receivedTotal = useCallCount(
    scope,
    { child_session_id: sessionId },
    { live: liveDataEnabled, enabled }
  );

  const madeCalls = (made.data?.pages ?? []).flatMap(page => page.items);
  const receivedCalls = (received.data?.pages ?? []).flatMap(page => page.items);

  const catalog = useQuery({
    ...sessionsCompleteListOptions({
      workspace_id: scope.workspaceId,
      root,
      ...scope.params,
    }),
    enabled: enabled && scope.workspaceId !== "" && root !== "",
  });
  const complete = catalog.isSuccess;
  const known = complete ? new Set(catalog.data.sessions.map(item => item.id)) : null;

  return {
    made: {
      calls: madeCalls,
      total: madeTotal,
      hasMore: made.hasNextPage,
      loadingMore: made.isFetchingNextPage,
      error:
        made.isError || made.isFetchNextPageError
          ? made.error instanceof Error
            ? made.error.message
            : "The calls request failed."
          : null,
      onRetry: () => {
        void made.refetch();
      },
      onLoadMore: () => {
        void made.fetchNextPage();
      },
    },
    received: {
      calls: receivedCalls,
      total: receivedTotal,
      hasMore: received.hasNextPage,
      loadingMore: received.isFetchingNextPage,
      error:
        received.isError || received.isFetchNextPageError
          ? received.error instanceof Error
            ? received.error.message
            : "The calls request failed."
          : null,
      onRetry: () => {
        void received.refetch();
      },
      onLoadMore: () => {
        void received.fetchNextPage();
      },
    },
    prunedSessionIds: prunedFrom(madeCalls, receivedCalls, known),
  };
}

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
 * panel shares one governed root, so one `root=` catalog read covers every
 * counterpart in both directions at once.
 */
import { useInfiniteQuery } from "@tanstack/react-query";

import {
  CALLS_PANEL_PAGE_SIZE,
  callsListOptions,
  useAgentCommsScope,
  useCallCount,
  type CallDirectionSection,
} from "@/systems/agent-comms";
import type { CallPayload } from "@/systems/agent-comms";

import { useSessions } from "./use-sessions";

export interface SessionCallsPanelModel {
  made: CallDirectionSection;
  received: CallDirectionSection;
  /** Counterparts retention has already removed. Empty until proven. */
  prunedSessionIds: ReadonlySet<string>;
}

const NO_PRUNED_SESSIONS: ReadonlySet<string> = new Set<string>();

/**
 * The one root every call on this panel hangs from.
 *
 * Read off the rows rather than assumed from `sessionId`: a session that was
 * itself delegated to is not its own root, and the daemon stamps the real one.
 */
function panelRoot(made: readonly CallPayload[], received: readonly CallPayload[]): string {
  return made[0]?.root_session_id ?? received[0]?.root_session_id ?? "";
}

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
  const root = panelRoot(madeCalls, receivedCalls);

  const catalog = useSessions(scope.workspaceId || null, {
    filters: { root },
    loadAll: true,
    enabled: enabled && root !== "",
  });
  const complete = root !== "" && !catalog.hasNextPage && !catalog.isError;
  const known = complete && catalog.data ? new Set(catalog.data.map(session => session.id)) : null;

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

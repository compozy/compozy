/**
 * Delegation rows and counts for the bell.
 *
 * Two server-filtered reads, never one broad page narrowed in the browser:
 *
 * - The unresolved delegation causes, read through the daemon's `attention`
 *   filter. Its `total` is the badge count, so the number stays right past the
 *   page bound. The filter — not a state list — is what makes the badge able to
 *   clear: `invalid-result` is a permanent terminal state, so a `state=` read
 *   would keep the bell lit forever. `attention=true` drops a cause the moment
 *   someone calls that child again or sends it a message, which is exactly what
 *   an operator means by "handled".
 * - The calls still in flight, used only to join against sessions the shell
 *   already classifies as blocked. That join is why "blocked on a decision" needs
 *   no tenth call state — being blocked is a fact about the child session, and
 *   the shell has exactly one definition of it.
 *
 * Rows survive a stale stream (the operator can still jump from a frozen row);
 * staleness travels with them so they contribute nothing to any count.
 */
import { useInfiniteQuery } from "@tanstack/react-query";

import {
  callsListOptions,
  deriveCallAttention,
  useAgentCommsScope,
  type CallAttentionRow,
} from "@/systems/agent-comms";
import type { SessionPayload } from "@/systems/session";

import { isNeedsYouSession } from "../lib/attention-model";

/** Calls that could be waiting on a decision. */
const OPEN_STATES = "queued,running";

const ROW_LIMIT = 100;

export interface OsAttentionCallsModel {
  rows: CallAttentionRow[];
  /** Daemon count of delegation causes needing a look. Zero while stale. */
  count: number;
  /** Child sessions whose call row already speaks for them. */
  coveredSessionIds: ReadonlySet<string>;
  stale: boolean;
  loading: boolean;
}

export function useAttentionCalls(
  sessions: readonly SessionPayload[],
  enabled: boolean,
  sessionsStale: boolean
): OsAttentionCallsModel {
  const scope = useAgentCommsScope();
  const active = enabled && scope.workspaceId !== "";

  const needsYou = useInfiniteQuery(
    callsListOptions(scope, { attention: true, limit: ROW_LIMIT }, active, active)
  );
  const open = useInfiniteQuery(
    callsListOptions(scope, { state: OPEN_STATES, limit: ROW_LIMIT }, active, active)
  );

  const blockedSessionIds = new Set<string>();
  for (const session of sessions) {
    if (isNeedsYouSession(session)) blockedSessionIds.add(session.id);
  }

  const stale =
    !active ||
    sessionsStale ||
    needsYou.isError ||
    open.isError ||
    needsYou.data === undefined ||
    open.data === undefined;

  const model = deriveCallAttention({
    needsYouCalls: (needsYou.data?.pages ?? []).flatMap(page => page.items),
    needsYouTotal: needsYou.data?.pages[0]?.total,
    openCalls: (open.data?.pages ?? []).flatMap(page => page.items),
    blockedSessionIds,
    stale,
  });

  return {
    rows: model.rows,
    count: model.count,
    coveredSessionIds: model.blockedChildSessionIds,
    stale,
    loading: active && (needsYou.isLoading || open.isLoading),
  };
}

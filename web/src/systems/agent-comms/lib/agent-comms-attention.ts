/**
 * Which calls need the operator, and how they reach the bell.
 *
 * Exactly three causes, per the spec — no fourth is invented here:
 *
 * 1. `invalid-result` — the answer never satisfied the contract.
 * 2. `completed-without-result` — the child finished and sent nothing.
 * 3. A child **blocked on a decision**.
 *
 * The third has no call-side state, and deliberately so: being blocked is a fact
 * about the child *session*, which the shell already models. Rather than mint a
 * tenth call state, this module joins two server-owned reads — the open calls in
 * this workspace, and the sessions the shell's own badge dictionary already
 * classifies as needs-you. One definition of "blocked", shared.
 *
 * That join is also why `blockedChildSessionIds` comes back out: a blocked child
 * would otherwise appear twice in the bell — once as a bare session row, once as
 * a call row carrying its delegation context. The call row is the better of the
 * two (it names the agent and the tree), so the OS layer suppresses the session
 * row for exactly these ids.
 *
 * There is no budget-exhausted cause. Completions are never admission-denied
 * (ADR-011), so no such state exists to surface.
 */
import { isNeedsYouCallState, toCallState, type CallState } from "./call-state";
import type { CallPayload } from "../types";

export type CallAttentionCause =
  | "invalid-result"
  | "completed-without-result"
  | "blocked-on-decision";

export interface CallAttentionRow {
  /** Stable across renders: the call, or the tree when coalesced. */
  id: string;
  cause: CallAttentionCause;
  agentName: string | null;
  rootSessionId: string;
  callId: string;
  childSessionId: string | null;
  /** ISO timestamp of the transition that raised this row. */
  changedAt: string;
  /**
   * How many causes this row stands for. `1` is an ordinary row; more means a
   * whole tree went wrong at once and the bell shows the real number instead of
   * N separate alarms.
   */
  count: number;
}

export interface CallAttentionInput {
  /**
   * The unresolved delegation causes, as the daemon's `attention` filter
   * returned them — never filtered down from a broader page, and never derived
   * from state here. Membership is the daemon's answer; `causeFor` below only
   * decides which of the two labels a row wears.
   */
  needsYouCalls: readonly CallPayload[];
  /** The daemon's count for that same filtered population. */
  needsYouTotal: number | undefined;
  /** Calls currently in flight, used only for the blocked-child join. */
  openCalls: readonly CallPayload[];
  /** Sessions the shell's badge dictionary already classifies as needs-you. */
  blockedSessionIds: ReadonlySet<string>;
  /**
   * Source disconnected. Stale rows stay listed and clickable — an old jump
   * target beats no target — but they contribute nothing to any count.
   */
  stale: boolean;
}

export interface CallAttentionModel {
  /** One row per cause, or one row per tree once coalesced. */
  rows: CallAttentionRow[];
  /** Badge count. Zero while stale, so a dead source never inflates the dock. */
  count: number;
  /** Child sessions already represented by a call row — suppress their session rows. */
  blockedChildSessionIds: ReadonlySet<string>;
}

/** Which label a row wears. Not a membership test — the daemon already ran one. */
function causeFor(state: CallState): CallAttentionCause | null {
  if (!isNeedsYouCallState(state)) return null;
  return state === "invalid-result" ? "invalid-result" : "completed-without-result";
}

function toRow(call: CallPayload, cause: CallAttentionCause): CallAttentionRow {
  return {
    id: call.call_id,
    cause,
    agentName: call.agent ?? null,
    rootSessionId: call.root_session_id,
    callId: call.call_id,
    childSessionId: call.child_session_id ?? null,
    changedAt: call.settled_at ?? call.updated_at,
    count: 1,
  };
}

/** Newest transition first — the same ordering the bell's other sections use. */
function byRecencyDesc(left: CallAttentionRow, right: CallAttentionRow): number {
  const recency = right.changedAt.localeCompare(left.changedAt);
  return recency === 0 ? left.id.localeCompare(right.id) : recency;
}

/**
 * Collapse a storm into one row per tree.
 *
 * A failing fan-out can put a dozen calls into `invalid-result` within seconds.
 * Twelve rows is not twelve pieces of information — it is one incident — so the
 * tree gets a single row carrying the real count, and opening it lands on that
 * tree unfolded. The representative row is the most recent, so the timestamp
 * still means something.
 */
function coalesceByTree(rows: readonly CallAttentionRow[]): CallAttentionRow[] {
  const byRoot = new Map<string, CallAttentionRow[]>();
  const order: string[] = [];
  for (const row of rows) {
    const existing = byRoot.get(row.rootSessionId);
    if (existing) {
      existing.push(row);
    } else {
      byRoot.set(row.rootSessionId, [row]);
      order.push(row.rootSessionId);
    }
  }

  return order.map(rootSessionId => {
    const group = byRoot.get(rootSessionId)!;
    if (group.length === 1) return group[0]!;
    const newest = [...group].sort(byRecencyDesc)[0]!;
    return { ...newest, id: `tree:${rootSessionId}`, count: group.length };
  });
}

export function deriveCallAttention(input: CallAttentionInput): CallAttentionModel {
  const stateRows: CallAttentionRow[] = [];
  for (const call of input.needsYouCalls) {
    const state = toCallState(call.state);
    if (state === null) continue;
    const cause = causeFor(state);
    if (cause === null) continue;
    stateRows.push(toRow(call, cause));
  }

  const blockedRows: CallAttentionRow[] = [];
  const blockedChildSessionIds = new Set<string>();
  for (const call of input.openCalls) {
    const child = call.child_session_id ?? "";
    if (child === "" || !input.blockedSessionIds.has(child)) continue;
    blockedRows.push(toRow(call, "blocked-on-decision"));
    blockedChildSessionIds.add(child);
  }

  const rows = coalesceByTree([...stateRows, ...blockedRows].sort(byRecencyDesc));

  // The badge is the daemon's `attention` total — never a loaded-page length
  // and never the blocked join, which is only as complete as the open-call page.
  const count = input.stale ? 0 : (input.needsYouTotal ?? 0);

  return { rows, count, blockedChildSessionIds };
}

/**
 * The delegation-tree projection. Components render what this returns and
 * compute nothing themselves.
 *
 * ## Why the hierarchy is walked over sessions, not over calls
 *
 * The obvious model — point each call at "the call whose `child_session_id` is
 * my `parent_session_id`" — is ambiguous, and quietly so. A follow-up call
 * reuses the same child session (`compozy call ses_… "one more thing"`), so
 * `child_session_id` is not unique across calls: a child that received three
 * calls offers three candidate parents for every call it then makes, and any
 * choice among them is invented structure.
 *
 * What the runtime actually models is a session lineage — caller session →
 * child session — with calls as the edges. So the walk descends sessions and
 * emits the calls on each edge, and the row's indent comes from the record's own
 * `depth`. That is the daemon's number, not a count of how many ancestors this
 * page happened to load, so indentation stays truthful across pagination.
 *
 * ## Nothing ever disappears
 *
 * A call whose caller session is unreachable — its parent's call fell outside
 * the page, or lineage is forged into a loop — is emitted as an orphan row at
 * the end of its group rather than dropped. Losing a row silently is worse than
 * showing one with less structure than usual.
 */
import type { SessionPayload } from "@/systems/session";

import { toCallState, type CallState } from "./call-state";
import type { CallPayload, ChildState } from "../types";

export interface CallTreeRow {
  call: CallPayload;
  /** Narrowed state, or null when the daemon sent a word the web does not know. */
  state: CallState | null;
  /** Daemon-recorded delegation depth. The row's indent, never a derived count. */
  depth: number;
  /**
   * True when the walk could not reach this call's caller and it was appended
   * rather than nested. The row still renders; it simply carries no ancestry.
   */
  orphaned: boolean;
  /**
   * Call ids delegated by this call's child session.
   *
   * Well-defined in this direction even though the reverse is not: "calls my
   * child made" is a fact, while "which of the calls to my child owns it" is a
   * guess whenever a child received more than one call. The walk assigns each
   * child session to the first call that reaches it, so a follow-up call never
   * duplicates the subtree its predecessor already owns.
   */
  childCallIds: string[];
}

export interface CallTreeGroup {
  /** The governed root every call in this group belongs to. */
  rootSessionId: string;
  /** Calls in lineage order, each carrying its own depth. */
  rows: CallTreeRow[];
  /** Call ids delegated directly by the root session — the group's first level. */
  topLevelCallIds: string[];
  /** The most urgent state present, for the collapsed header. */
  escalation: CallState | null;
}

export interface CallCommsTree {
  groups: CallTreeGroup[];
  /**
   * Sessions whose lineage loops back on itself. The walk stops at them; they
   * are exposed so a surface can say the structure is malformed instead of
   * pretending the missing branch does not exist.
   */
  cyclicSessionIds: ReadonlySet<string>;
  /** Every row by call id — the lookup a tree data-loader needs. */
  rowsByCallId: ReadonlyMap<string, CallTreeRow>;
}

/**
 * Urgency order for the collapsed-header escalation, most urgent first.
 *
 * The needs-you pair leads because those are the only two states that put a row
 * in the bell. `failed` and `expired` follow — same danger tone, but not
 * operator-actionable. Deliberate stops rank below faults. `running` outranks
 * `queued` so a live tree reads as live, and `completed` never escalates: a good
 * answer is not an alarm.
 */
const ESCALATION_ORDER: readonly CallState[] = [
  "invalid-result",
  "completed-without-result",
  "failed",
  "expired",
  "canceled",
  "timeout",
  "running",
  "queued",
];

const ESCALATION_RANK = new Map<CallState, number>(
  ESCALATION_ORDER.map((state, index) => [state, index])
);

/** The single most urgent state among a group's rows, or null when all are calm. */
export function escalateCallStates(rows: readonly CallTreeRow[]): CallState | null {
  let best: CallState | null = null;
  let bestRank = Number.POSITIVE_INFINITY;
  for (const row of rows) {
    if (row.state === null) continue;
    const rank = ESCALATION_RANK.get(row.state);
    if (rank === undefined || rank >= bestRank) continue;
    best = row.state;
    bestRank = rank;
    if (rank === 0) break;
  }
  return best;
}

/**
 * The same escalation over raw calls, for surfaces that hold a batch rather than
 * a projected tree — the transcript's fan-out card, chiefly. One rule, one
 * ordering, whichever shape the caller has.
 */
export function escalateCallPayloads(calls: readonly CallPayload[]): CallState | null {
  let best: CallState | null = null;
  let bestRank = Number.POSITIVE_INFINITY;
  for (const call of calls) {
    const state = toCallState(call.state);
    if (state === null) continue;
    const rank = ESCALATION_RANK.get(state);
    if (rank === undefined || rank >= bestRank) continue;
    best = state;
    bestRank = rank;
    if (rank === 0) break;
  }
  return best;
}

function toRow(call: CallPayload, orphaned: boolean): CallTreeRow {
  return {
    call,
    state: toCallState(call.state),
    depth: call.depth,
    orphaned,
    childCallIds: [],
  };
}

function indexByCallerSession(calls: readonly CallPayload[]): Map<string, CallPayload[]> {
  const byCaller = new Map<string, CallPayload[]>();
  for (const call of calls) {
    const caller = call.parent_session_id ?? "";
    const siblings = byCaller.get(caller);
    if (siblings) {
      siblings.push(call);
    } else {
      byCaller.set(caller, [call]);
    }
  }
  return byCaller;
}

interface WalkResult {
  rows: CallTreeRow[];
  emitted: Set<string>;
  cyclic: Set<string>;
  topLevelCallIds: string[];
}

/**
 * One step of the walk: either expand a session's outgoing calls, or emit a call
 * and descend into the child it created.
 *
 * Two frame kinds rather than a stack of session ids, because the order has to
 * be genuinely depth-first: a call's own subtree must land directly beneath it,
 * not after all of its siblings. Expanding sessions alone would read the tree
 * breadth-first and scatter one delegation across the group.
 */
type WalkFrame = { kind: "session"; sessionId: string } | { kind: "call"; call: CallPayload };

/**
 * Depth-first over the session lineage, emitting each session's outgoing calls
 * in the order the daemon returned them.
 *
 * A session is expanded at most once. Reaching an already-expanded session means
 * the lineage loops — a child has exactly one creating caller, so there is no
 * legitimate way to arrive twice — and the walk records it and stops descending
 * rather than recursing forever. An explicit stack keeps a 150-call tree off the
 * JS call stack.
 */
function walkLineage(
  rootSessionId: string,
  byCaller: ReadonlyMap<string, readonly CallPayload[]>
): WalkResult {
  const rows: CallTreeRow[] = [];
  const emitted = new Set<string>();
  const cyclic = new Set<string>();
  const expanded = new Set<string>();
  const topLevelCallIds: string[] = [];
  const rowByCallId = new Map<string, CallTreeRow>();
  /**
   * Which call owns each child session. The first call to reach a session claims
   * it, so a follow-up call to an already-claimed child does not re-parent the
   * work that child already did.
   */
  const ownerCallIdBySession = new Map<string, string>();
  const stack: WalkFrame[] = [{ kind: "session", sessionId: rootSessionId }];

  while (stack.length > 0) {
    const frame = stack.pop();
    if (frame === undefined) break;

    if (frame.kind === "call") {
      const call = frame.call;
      if (emitted.has(call.call_id)) continue;
      emitted.add(call.call_id);
      const row = toRow(call, false);
      rows.push(row);
      rowByCallId.set(call.call_id, row);

      const ownerCallId = ownerCallIdBySession.get(call.parent_session_id ?? "");
      const ownerRow = ownerCallId === undefined ? undefined : rowByCallId.get(ownerCallId);
      if (ownerRow) {
        ownerRow.childCallIds.push(call.call_id);
      } else {
        topLevelCallIds.push(call.call_id);
      }

      const child = call.child_session_id ?? "";
      if (child === "") continue;
      if (!ownerCallIdBySession.has(child)) ownerCallIdBySession.set(child, call.call_id);
      if (expanded.has(child)) {
        cyclic.add(child);
        continue;
      }
      stack.push({ kind: "session", sessionId: child });
      continue;
    }

    if (expanded.has(frame.sessionId)) {
      cyclic.add(frame.sessionId);
      continue;
    }
    expanded.add(frame.sessionId);
    const outgoing = byCaller.get(frame.sessionId) ?? [];
    for (let index = outgoing.length - 1; index >= 0; index -= 1) {
      stack.push({ kind: "call", call: outgoing[index]! });
    }
  }

  return { rows, emitted, cyclic, topLevelCallIds };
}

/**
 * Group a page of calls into delegation trees.
 *
 * Group order follows the incoming list order — the daemon returns calls newest
 * first, so the tree an operator just started sits at the top.
 */
export function buildCallTree(calls: readonly CallPayload[]): CallCommsTree {
  const groupOrder: string[] = [];
  const byRoot = new Map<string, CallPayload[]>();
  for (const call of calls) {
    const root = call.root_session_id;
    const existing = byRoot.get(root);
    if (existing) {
      existing.push(call);
    } else {
      byRoot.set(root, [call]);
      groupOrder.push(root);
    }
  }

  const cyclicSessionIds = new Set<string>();
  const rowsByCallId = new Map<string, CallTreeRow>();
  const groups: CallTreeGroup[] = [];
  for (const rootSessionId of groupOrder) {
    const members = byRoot.get(rootSessionId) ?? [];
    const walk = walkLineage(rootSessionId, indexByCallerSession(members));
    for (const sessionId of walk.cyclic) cyclicSessionIds.add(sessionId);

    const rows = walk.rows;
    const topLevelCallIds = [...walk.topLevelCallIds];
    // Unreachable calls join the group's first level rather than vanishing:
    // less ancestry than usual, but still on screen and still openable.
    for (const call of members) {
      if (walk.emitted.has(call.call_id)) continue;
      rows.push(toRow(call, true));
      topLevelCallIds.push(call.call_id);
    }
    for (const row of rows) rowsByCallId.set(row.call.call_id, row);
    groups.push({
      rootSessionId,
      rows,
      topLevelCallIds,
      escalation: escalateCallStates(rows),
    });
  }

  return { groups, cyclicSessionIds, rowsByCallId };
}

/**
 * Which of a tree's children are parked, still working, or gone.
 *
 * The expected children are an **input**, and that is the whole design. A child
 * the catalog omits has no session row to iterate, so absence only carries
 * meaning when measured against a list of who was supposed to be there — a
 * projection over sessions alone could never emit `gone`.
 *
 * `sessions` is `undefined` while a root's catalog is still loading or has
 * errored, and then nothing is claimed about that root at all. Reporting `gone`
 * on a transport blip would turn a slow network into a wall of dead children.
 */
export function childStatesForRoot(
  expectedChildIds: readonly string[],
  sessions: readonly SessionPayload[] | undefined
): ReadonlyMap<string, ChildState> {
  const states = new Map<string, ChildState>();
  if (sessions === undefined) return states;
  for (const childId of expectedChildIds) {
    if (childId !== "") states.set(childId, "gone");
  }
  for (const session of sessions) {
    if (!states.has(session.id)) continue;
    if (session.state === "stopped") {
      states.set(session.id, isParkedCallChild(session) ? "parked" : "gone");
      continue;
    }
    states.set(session.id, "running");
  }
  return states;
}

/**
 * A parked child is stopped *on purpose*, by settlement.
 *
 * The daemon has no `parked` session state — it parks by writing `parked_at`
 * and stopping the child with this exact reason (`parkSettledChild`). The stop
 * detail is the only part of that visible on the wire, which is why it is
 * matched literally rather than inferred from `stopped` alone: every other
 * stopped child is gone, whether it failed, was canceled, or ended normally.
 */
function isParkedCallChild(session: SessionPayload): boolean {
  return session.state === "stopped" && session.stop_detail === PARKED_STOP_DETAIL;
}

const PARKED_STOP_DETAIL = "call child parked";

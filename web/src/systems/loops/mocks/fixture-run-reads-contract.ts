import {
  LOOP_ROSTER_STATE_FILTERS,
  isLoopRosterStateFilter,
} from "../adapters/loop-roster-filters";
import { type LoopTimelineView, isLoopTimelineView } from "../adapters/loop-timeline-filters";

/**
 * The read layer's refusals, as the daemon actually writes them.
 *
 * A mock that accepts anything is worse than no mock: it lets a filter the
 * daemon rejects look supported in every test that touches it, and it gives the
 * adapters' error mapping nothing to map. These mirror
 * `respondLoopRunReadError` — the same statuses, the same `code`, the same
 * `details` — so a resolver refusal and a real one are the same object to
 * everything downstream.
 */
export interface LoopReadRefusalOf<S extends number> {
  status: S;
  body: { error: string; code: string; details?: Record<string, string> };
}

/** The statuses the read routes declare; the status is literal so MSW can match it. */
export type LoopReadStatus = 400 | 404 | 409;
export type LoopReadRefusal = LoopReadRefusalOf<LoopReadStatus>;

export type LoopReadResult<T, S extends LoopReadStatus = LoopReadStatus> =
  | { ok: true; page: T }
  | { ok: false; refusal: LoopReadRefusalOf<S> };

export function loopRunNotFound(runId: string): LoopReadRefusalOf<404> {
  return {
    status: 404,
    body: { error: "loop_run_not_found", code: "loop_run_not_found", details: { run_id: runId } },
  };
}

export function invalidNodeState(): LoopReadRefusalOf<400> {
  return {
    status: 400,
    body: {
      error: "invalid_node_state",
      code: "invalid_node_state",
      details: { allowed: LOOP_ROSTER_STATE_FILTERS.join(",") },
    },
  };
}

export function invalidCursor(): LoopReadRefusalOf<400> {
  return { status: 400, body: { error: "invalid_cursor", code: "invalid_cursor" } };
}

export function invalidRequest(message: string): LoopReadRefusalOf<400> {
  return { status: 400, body: { error: message, code: "invalid_request" } };
}

const ROSTER_LIMIT_MAX = 500;
const TIMELINE_LIMIT_MAX = 500;

/** The daemon's allowlist, read from the one module that declares it. */
const isRosterState = isLoopRosterStateFilter;

/**
 * An integer parameter, or a refusal. A malformed number is a bad request, not
 * an invitation to substitute the default — the daemon's own parser says so.
 */
function integerParam(
  params: URLSearchParams,
  name: string
): { ok: true; value: number | null } | { ok: false; refusal: LoopReadRefusalOf<400> } {
  const raw = params.get(name);
  if (raw === null || raw.trim() === "") return { ok: true, value: null };
  const value = Number(raw);
  if (!Number.isInteger(value)) {
    return { ok: false, refusal: invalidRequest(`${name} must be an integer`) };
  }
  return { ok: true, value };
}

export interface RosterQuery {
  state: string;
  generation: number | null;
  limit: number;
  cursor: RosterCursor | null;
}

export interface RosterCursor {
  runId: string;
  offset: number;
}

export interface TimelineQuery {
  view: LoopTimelineView;
  afterSequence: number;
  limit: number;
  cursor: TimelineCursor | null;
}

/**
 * The cursor binds the run, the view and the snapshot head it was minted
 * against, exactly like `timelineCursor` in the daemon. Replaying one against a
 * different run or view is a branch change, not a page — splicing two histories
 * together is the failure this token exists to make impossible.
 */
export interface TimelineCursor {
  runId: string;
  view: LoopTimelineView;
  fixedHeadSeq: number;
  beforeSeq: number;
}

function encode(value: unknown): string {
  return btoa(JSON.stringify(value));
}

function decode(raw: string): Record<string, unknown> | null {
  try {
    const parsed: unknown = JSON.parse(atob(raw));
    return typeof parsed === "object" && parsed !== null
      ? (parsed as Record<string, unknown>)
      : null;
  } catch {
    // A cursor the client did not get from us. Opaque means unparseable, which
    // is a 400 — never a silent restart from the head.
    return null;
  }
}

export function encodeRosterCursor(cursor: RosterCursor): string {
  return encode({ run_id: cursor.runId, offset: cursor.offset });
}

export function encodeTimelineCursor(cursor: TimelineCursor): string {
  return encode({
    run_id: cursor.runId,
    view: cursor.view,
    fixed_head_seq: cursor.fixedHeadSeq,
    before_seq: cursor.beforeSeq,
  });
}

export function normalizeRosterQuery(
  runId: string,
  params: URLSearchParams
): LoopReadResult<RosterQuery, 400> {
  const state = params.get("state") ?? "all";
  if (!isRosterState(state)) return { ok: false, refusal: invalidNodeState() };
  const generation = integerParam(params, "generation");
  if (!generation.ok) return generation;
  if (generation.value !== null && generation.value < 0) {
    return { ok: false, refusal: invalidRequest("generation must not be negative") };
  }
  const limit = integerParam(params, "limit");
  if (!limit.ok) return limit;
  if (limit.value !== null && (limit.value < 1 || limit.value > ROSTER_LIMIT_MAX)) {
    return { ok: false, refusal: invalidRequest("roster limit must be between 1 and 500") };
  }
  const rawCursor = params.get("cursor");
  let cursor: RosterCursor | null = null;
  if (rawCursor !== null && rawCursor.trim() !== "") {
    const decoded = decode(rawCursor);
    const offset = decoded?.offset;
    if (decoded === null || decoded.run_id !== runId || typeof offset !== "number") {
      return { ok: false, refusal: invalidCursor() };
    }
    cursor = { runId, offset };
  }
  return {
    ok: true,
    page: { state, generation: generation.value, limit: limit.value ?? ROSTER_LIMIT_MAX, cursor },
  };
}

export function normalizeTimelineQuery(
  runId: string,
  params: URLSearchParams,
  headSeq: number
): LoopReadResult<TimelineQuery, 400 | 409> {
  const rawView = params.get("view") ?? "notable";
  if (!isLoopTimelineView(rawView)) {
    return { ok: false, refusal: invalidRequest(`unknown timeline view: ${rawView}`) };
  }
  const view = rawView;
  const after = integerParam(params, "after_sequence");
  if (!after.ok) return after;
  if (after.value !== null && after.value < 0) {
    return { ok: false, refusal: invalidRequest("after_sequence must not be negative") };
  }
  const limit = integerParam(params, "limit");
  if (!limit.ok) return limit;
  if (limit.value !== null && (limit.value < 1 || limit.value > TIMELINE_LIMIT_MAX)) {
    return { ok: false, refusal: invalidRequest("timeline limit must be between 1 and 500") };
  }
  const rawCursor = params.get("cursor");
  let cursor: TimelineCursor | null = null;
  if (rawCursor !== null && rawCursor.trim() !== "") {
    const decoded = decode(rawCursor);
    if (decoded === null || typeof decoded.before_seq !== "number") {
      return { ok: false, refusal: invalidCursor() };
    }
    // A cursor minted for another run, another view, or another snapshot head
    // addresses a page set that no longer exists on this branch.
    if (decoded.run_id !== runId || decoded.view !== view || decoded.fixed_head_seq !== headSeq) {
      return {
        ok: false,
        refusal: {
          status: 409,
          body: { error: "timeline_branch_changed", code: "timeline_branch_changed" },
        },
      };
    }
    cursor = { runId, view, fixedHeadSeq: headSeq, beforeSeq: decoded.before_seq };
  }
  return {
    ok: true,
    page: {
      view,
      afterSequence: after.value ?? 0,
      limit: limit.value ?? TIMELINE_LIMIT_MAX,
      cursor,
    },
  };
}

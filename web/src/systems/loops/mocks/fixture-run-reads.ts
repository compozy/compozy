import { isTerminalLoopStatus } from "../lib/loop-formatters";
import type {
  LoopBriefing,
  LoopRunRosterPage,
  LoopTimelineEntry,
  LoopTimelinePage,
} from "../types";
import {
  DONE_RUN_ID,
  NEEDS_APPROVAL_RUN_ID,
  doneBriefing,
  doneRoster,
  doneTimeline,
  needsApprovalBriefing,
  needsApprovalRoster,
  needsApprovalTimeline,
} from "./fixture-run-reads-gated";
import { TIMELINE_NOISE_KINDS, rosterNode, runsById } from "./fixture-run-reads-nodes";

/**
 * The run read layer's mock world: briefing, roster and timeline for runs the
 * catalog fixtures already tell stories about (ADR-005).
 *
 * The projections are the daemon's, so the fixtures reproduce them rather than
 * inventing friendlier shapes: `progress.round` is the run's generation and
 * `steps_*` counts only that round's action nodes, `fanout_rollups` covers the
 * complete roster before any filter or page, timeline pages are newest-first
 * behind a fixed `head_seq`, and chatter arrives already coalesced so a folded
 * beat spans `first_seq..seq`. A story that renders a page the runtime cannot
 * produce is worse than no story at all.
 *
 * The runs that have stopped moving live in `fixture-run-reads-gated.ts`; this
 * file keeps the one that is still going, plus the resolvers every run answers
 * through.
 */

// Running run ---------------------------------------------------------------
// `looprun_running` is implement-tasks in round 1: the fan-out is half settled
// and one branch is on its second attempt after a transport failure.

const RUNNING_RUN_ID = "looprun_running";

const runningRoster: LoopRunRosterPage = {
  run_id: RUNNING_RUN_ID,
  loop_name: "implement-tasks",
  run_status: "running",
  nodes: [
    rosterNode(RUNNING_RUN_ID, 1, {
      node_id: "slug_input",
      state: "succeeded",
      attempt: 1,
      started_at: "2026-07-05T12:00:01Z",
      ended_at: "2026-07-05T12:00:01Z",
    }),
    rosterNode(RUNNING_RUN_ID, 1, {
      node_id: "load_tasks",
      state: "succeeded",
      attempt: 1,
      started_at: "2026-07-05T12:00:02Z",
      ended_at: "2026-07-05T12:00:41Z",
      usage: { tokens: 4_200 },
      attempts: [
        {
          attempt: 1,
          state: "succeeded",
          disposition: "succeeded",
          started_at: "2026-07-05T12:00:02Z",
          ended_at: "2026-07-05T12:00:41Z",
        },
      ],
    }),
    rosterNode(RUNNING_RUN_ID, 1, {
      node_id: "implement",
      state: "succeeded",
      attempt: 1,
      started_at: "2026-07-05T12:00:41Z",
      ended_at: "2026-07-05T12:00:42Z",
    }),
    rosterNode(RUNNING_RUN_ID, 1, {
      node_id: "execute_task",
      item_index: 0,
      state: "succeeded",
      attempt: 1,
      session_id: "sess_implement_tasks_0",
      started_at: "2026-07-05T12:00:42Z",
      ended_at: "2026-07-05T12:08:10Z",
      usage: { tokens: 180_400 },
      attempts: [
        {
          attempt: 1,
          state: "succeeded",
          disposition: "succeeded",
          started_at: "2026-07-05T12:00:42Z",
          ended_at: "2026-07-05T12:08:10Z",
        },
      ],
    }),
    rosterNode(RUNNING_RUN_ID, 1, {
      node_id: "execute_task",
      item_index: 1,
      state: "running",
      attempt: 2,
      session_id: "sess_implement_tasks_1",
      started_at: "2026-07-05T12:08:12Z",
      // The roster carries the last attempt that ended, so a node retrying in
      // place reports its failed attempt's end. Reproduced on purpose.
      ended_at: "2026-07-05T12:11:03Z",
      usage: { tokens: 121_800 },
      attempts: [
        {
          attempt: 1,
          state: "retrying",
          disposition: "retried",
          // `transport` is one of the two retry-eligible classes, which is what
          // an attempt that failed and came back has to be.
          failure_class: "transport",
          started_at: "2026-07-05T12:08:12Z",
          ended_at: "2026-07-05T12:11:03Z",
        },
        {
          attempt: 2,
          state: "running",
          disposition: "",
          started_at: "2026-07-05T12:11:34Z",
          ended_at: null,
        },
      ],
    }),
    rosterNode(RUNNING_RUN_ID, 1, { node_id: "execute_task", item_index: 2, state: "pending" }),
    rosterNode(RUNNING_RUN_ID, 1, { node_id: "collect", state: "pending" }),
  ],
  fanout_rollups: [{ generation: 1, node_id: "execute_task", total: 3, done: 1, failed: 0 }],
  next_cursor: "",
};

const runningBriefing: LoopBriefing = {
  run_id: RUNNING_RUN_ID,
  status: "running",
  tone: "ok",
  headline: "Running step execute_task in round 1.",
  blockers: [],
  artifacts: [],
  // Action nodes of round 1: load_tasks plus the three execute_task branches,
  // two of them settled.
  progress: { round: 1, steps_done: 2, steps_total: 4 },
  usage: { tokens: 412_000, cost_usd: 4.12, budget_used_pct: 20.6, duration_ms: 1_080_000 },
};

const runningTimeline: LoopTimelinePage = {
  run_id: RUNNING_RUN_ID,
  head_seq: 12,
  entries: [
    {
      seq: 12,
      first_seq: 8,
      kind: "token_tick",
      generation: 1,
      title: "token tick",
      at: "2026-07-05T12:18:00Z",
    },
    {
      seq: 7,
      kind: "node_running",
      generation: 1,
      node_id: "execute_task",
      attempt: 2,
      title: "step execute_task running",
      at: "2026-07-05T12:11:34Z",
    },
    {
      seq: 6,
      kind: "node_retry_scheduled",
      generation: 1,
      node_id: "execute_task",
      attempt: 1,
      title: "step execute_task retry scheduled",
      at: "2026-07-05T12:11:04Z",
    },
    {
      seq: 5,
      kind: "node_failed",
      generation: 1,
      node_id: "execute_task",
      attempt: 1,
      title: "step execute_task failed",
      at: "2026-07-05T12:11:03Z",
    },
    {
      seq: 4,
      kind: "node_succeeded",
      generation: 1,
      node_id: "execute_task",
      attempt: 1,
      title: "step execute_task succeeded",
      at: "2026-07-05T12:08:10Z",
    },
    {
      seq: 3,
      kind: "node_running",
      generation: 1,
      node_id: "execute_task",
      attempt: 1,
      title: "step execute_task running",
      at: "2026-07-05T12:00:42Z",
    },
    {
      seq: 2,
      kind: "node_succeeded",
      generation: 1,
      node_id: "load_tasks",
      attempt: 1,
      title: "step load_tasks succeeded",
      at: "2026-07-05T12:00:41Z",
    },
    {
      seq: 1,
      kind: "generation_started",
      generation: 1,
      title: "round 1 started",
      at: "2026-07-05T12:00:00Z",
    },
  ],
  next_cursor: "",
};

// Keyed fixtures ------------------------------------------------------------

export const loopRunBriefingByRunId: ReadonlyMap<string, LoopBriefing> = new Map([
  [RUNNING_RUN_ID, runningBriefing],
  [NEEDS_APPROVAL_RUN_ID, needsApprovalBriefing],
  [DONE_RUN_ID, doneBriefing],
]);

/** The complete roster per run: the resolvers filter and page over these. */
export const loopRunRosterByRunId: ReadonlyMap<string, LoopRunRosterPage> = new Map([
  [RUNNING_RUN_ID, runningRoster],
  [NEEDS_APPROVAL_RUN_ID, needsApprovalRoster],
  [DONE_RUN_ID, doneRoster],
]);

/** The complete newest-first story per run, behind that run's fixed head. */
export const loopRunTimelineByRunId: ReadonlyMap<string, LoopTimelinePage> = new Map([
  [RUNNING_RUN_ID, runningTimeline],
  [NEEDS_APPROVAL_RUN_ID, needsApprovalTimeline],
  [DONE_RUN_ID, doneTimeline],
]);

// Fallbacks -----------------------------------------------------------------
// Every other run in the mock world still answers, so a story never meets an
// unexpected 404. It answers with what the runs list already knows and an empty
// roster and story, which is the honest read for a run nobody authored history
// for.

function fallbackHeadline(status: string, round: number): string {
  if (isTerminalLoopStatus(status)) return `Run finished: ${status}.`;
  if (status === "queued") return "Waiting to start because the concurrency cap is full.";
  return `Run is active in round ${round}.`;
}

function fallbackBriefing(runId: string): LoopBriefing {
  const run = runsById.get(runId);
  const status = run?.status ?? "running";
  const round = run?.generation ?? 1;
  const terminal = isTerminalLoopStatus(status);
  const failed = status === "failed" || status === "exhausted" || status === "stalled";
  const at = run?.last_progress_at ?? run?.created_at ?? "2026-07-05T12:00:00Z";
  const budget = run?.budget_tokens ?? 0;
  const tokens = run?.tokens_used ?? 0;
  return {
    run_id: runId,
    status,
    tone: failed ? "failed" : "ok",
    headline: fallbackHeadline(status, round),
    blockers: [],
    artifacts: [],
    ...(terminal ? { outcome: { status, cause: status, at } } : {}),
    progress: { round, steps_done: 0, steps_total: 0 },
    usage: { tokens, ...(budget > 0 ? { budget_used_pct: (tokens / budget) * 100 } : {}) },
  };
}

function fallbackRoster(runId: string): LoopRunRosterPage {
  const run = runsById.get(runId);
  return {
    run_id: runId,
    loop_name: run?.loop_name ?? "",
    run_status: run?.status ?? "running",
    nodes: [],
    fanout_rollups: [],
    next_cursor: "",
  };
}

function fallbackTimeline(runId: string): LoopTimelinePage {
  return { run_id: runId, head_seq: 0, entries: [], next_cursor: "" };
}

// Resolvers -----------------------------------------------------------------

function numberParam(params: URLSearchParams, name: string): number | null {
  const raw = params.get(name);
  if (raw === null || raw.trim() === "") return null;
  const value = Number(raw);
  return Number.isFinite(value) ? value : null;
}

export function resolveLoopRunBriefing(runId: string): LoopBriefing {
  return loopRunBriefingByRunId.get(runId) ?? fallbackBriefing(runId);
}

export function resolveLoopRunRoster(runId: string, params: URLSearchParams): LoopRunRosterPage {
  const page = loopRunRosterByRunId.get(runId) ?? fallbackRoster(runId);
  const state = params.get("state");
  const generation = numberParam(params, "generation");
  const matched = page.nodes.filter(node => {
    if (state && state !== "all" && node.state !== state) return false;
    if (generation !== null && node.generation !== generation) return false;
    return true;
  });
  // The rollups are the daemon's whole-roster counts, so filtering a page never
  // shrinks them.
  const offset = numberParam(params, "cursor") ?? 0;
  const limit = numberParam(params, "limit") ?? matched.length;
  const end = Math.min(offset + limit, matched.length);
  return {
    ...page,
    nodes: matched.slice(offset, end),
    next_cursor: end < matched.length ? String(end) : "",
  };
}

export function resolveLoopRunTimeline(runId: string, params: URLSearchParams): LoopTimelinePage {
  const page = loopRunTimelineByRunId.get(runId) ?? fallbackTimeline(runId);
  const view = params.get("view") ?? "notable";
  const afterSequence = numberParam(params, "after_sequence") ?? 0;
  // The cursor is opaque to the client; the daemon fences it to the page's head,
  // and the mock keeps the same "everything older than this" meaning.
  const before = numberParam(params, "cursor") ?? Number.POSITIVE_INFINITY;
  const matched = page.entries.filter((entry: LoopTimelineEntry) => {
    if (view !== "all" && TIMELINE_NOISE_KINDS.has(entry.kind)) return false;
    return entry.seq > afterSequence && entry.seq < before;
  });
  const limit = numberParam(params, "limit") ?? matched.length;
  const entries = matched.slice(0, limit);
  const last = entries.at(-1);
  // Paging resumes below the beat's first sequence, so a coalesced run of
  // chatter is never served twice.
  const exhausted = entries.length === matched.length || last === undefined;
  return {
    ...page,
    entries,
    next_cursor: exhausted ? "" : String(last.first_seq ?? last.seq),
  };
}

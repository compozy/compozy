import type { LoopBriefing, LoopRunRosterPage, LoopTimelinePage } from "../types";
import { type RosterNodeSeed, rosterNode } from "./fixture-run-reads-nodes";

/**
 * The two runs that have stopped moving: one waiting on a person, one finished.
 *
 * They live together because they are the same question asked twice — what does
 * a run that is not going to change on its own look like — and apart from the
 * running fixture because that one is about motion.
 */

// Gated run -----------------------------------------------------------------
// `looprun_needs_approval` is quality-gate-demo in round 3: everything upstream
// settled and a person is the only way forward.

export const NEEDS_APPROVAL_RUN_ID = "looprun_needs_approval";

export const needsApprovalRoster: LoopRunRosterPage = {
  run_id: NEEDS_APPROVAL_RUN_ID,
  loop_name: "quality-gate-demo",
  run_status: "needs-approval",
  nodes: [
    rosterNode(NEEDS_APPROVAL_RUN_ID, 3, {
      node_id: "slug",
      state: "succeeded",
      attempt: 1,
      started_at: "2026-07-05T12:10:00Z",
      ended_at: "2026-07-05T12:10:01Z",
    }),
    rosterNode(NEEDS_APPROVAL_RUN_ID, 3, {
      node_id: "load_tasks",
      state: "succeeded",
      attempt: 1,
      started_at: "2026-07-05T12:10:01Z",
      ended_at: "2026-07-05T12:10:22Z",
    }),
    rosterNode(NEEDS_APPROVAL_RUN_ID, 3, {
      node_id: "implement",
      state: "succeeded",
      attempt: 1,
      started_at: "2026-07-05T12:10:22Z",
      ended_at: "2026-07-05T12:10:23Z",
    }),
    rosterNode(NEEDS_APPROVAL_RUN_ID, 3, {
      node_id: "execute_task",
      item_index: 0,
      state: "succeeded",
      attempt: 1,
      session_id: "sess_quality_gate_0",
      started_at: "2026-07-05T12:10:23Z",
      ended_at: "2026-07-05T12:12:20Z",
      usage: { tokens: 402_000 },
      attempts: [
        {
          attempt: 1,
          state: "succeeded",
          disposition: "succeeded",
          started_at: "2026-07-05T12:10:23Z",
          ended_at: "2026-07-05T12:12:20Z",
        },
      ],
    }),
    rosterNode(NEEDS_APPROVAL_RUN_ID, 3, {
      node_id: "execute_task",
      item_index: 1,
      state: "succeeded",
      attempt: 1,
      session_id: "sess_quality_gate_1",
      started_at: "2026-07-05T12:10:23Z",
      ended_at: "2026-07-05T12:13:40Z",
      usage: { tokens: 388_500 },
      attempts: [
        {
          attempt: 1,
          state: "succeeded",
          disposition: "succeeded",
          started_at: "2026-07-05T12:10:23Z",
          ended_at: "2026-07-05T12:13:40Z",
        },
      ],
    }),
    rosterNode(NEEDS_APPROVAL_RUN_ID, 3, {
      node_id: "collect",
      state: "succeeded",
      attempt: 1,
      started_at: "2026-07-05T12:13:40Z",
      ended_at: "2026-07-05T12:13:41Z",
    }),
    rosterNode(NEEDS_APPROVAL_RUN_ID, 3, {
      node_id: "review",
      state: "succeeded",
      attempt: 1,
      started_at: "2026-07-05T12:13:41Z",
      ended_at: "2026-07-05T12:15:02Z",
    }),
    rosterNode(NEEDS_APPROVAL_RUN_ID, 3, {
      node_id: "verify",
      state: "succeeded",
      attempt: 1,
      started_at: "2026-07-05T12:15:02Z",
      ended_at: "2026-07-05T12:16:30Z",
    }),
    rosterNode(NEEDS_APPROVAL_RUN_ID, 3, {
      node_id: "approve",
      state: "waiting",
      attempt: 1,
      started_at: "2026-07-05T12:18:00Z",
    }),
  ],
  fanout_rollups: [{ generation: 3, node_id: "execute_task", total: 2, done: 2, failed: 0 }],
  next_cursor: "",
};

export const needsApprovalBriefing: LoopBriefing = {
  run_id: NEEDS_APPROVAL_RUN_ID,
  status: "needs-approval",
  tone: "needs_you",
  headline: "This run needs attention: approval.",
  blockers: [
    {
      kind: "approval",
      gate_id: "approve",
      waiting_since: "2026-07-05T12:18:00Z",
      unblocker: `compozy loop approve ${NEEDS_APPROVAL_RUN_ID} --gate approve`,
    },
  ],
  artifacts: [],
  // quality-gate-demo authors every gate as a control node, so execute_task's
  // two branches are the round's whole action population.
  progress: { round: 3, steps_done: 2, steps_total: 2 },
  usage: { tokens: 1_100_000, cost_usd: 11.4, budget_used_pct: 55, duration_ms: 1_080_000 },
};

export const needsApprovalTimeline: LoopTimelinePage = {
  run_id: NEEDS_APPROVAL_RUN_ID,
  head_seq: 6,
  entries: [
    {
      seq: 6,
      kind: "needs_approval",
      generation: 3,
      node_id: "approve",
      title: 'approval "approve" opened',
      at: "2026-07-05T12:18:00Z",
    },
    {
      seq: 5,
      kind: "gate_verdict",
      generation: 3,
      node_id: "verify",
      title: 'gate "verify" approved',
      at: "2026-07-05T12:16:30Z",
    },
    {
      seq: 4,
      kind: "gate_verdict",
      generation: 3,
      node_id: "review",
      title: 'gate "review" approved',
      at: "2026-07-05T12:15:02Z",
    },
    {
      seq: 3,
      kind: "node_succeeded",
      generation: 3,
      node_id: "execute_task",
      attempt: 1,
      title: "step execute_task succeeded",
      at: "2026-07-05T12:13:40Z",
    },
    {
      seq: 2,
      kind: "node_succeeded",
      generation: 3,
      node_id: "execute_task",
      attempt: 1,
      title: "step execute_task succeeded",
      at: "2026-07-05T12:12:20Z",
    },
    {
      seq: 1,
      kind: "generation_started",
      generation: 3,
      title: "round 3 started",
      at: "2026-07-05T12:10:00Z",
    },
  ],
  next_cursor: "",
};

// Terminal run --------------------------------------------------------------
// `looprun_done_today` finished: it carries an outcome and two artifacts, one of
// which retention has already pruned.

export const DONE_RUN_ID = "looprun_done_today";

function doneNode(nodeId: string, seed: Omit<RosterNodeSeed, "node_id" | "state">) {
  return rosterNode(DONE_RUN_ID, 1, { node_id: nodeId, state: "succeeded", attempt: 1, ...seed });
}

export const doneRoster: LoopRunRosterPage = {
  run_id: DONE_RUN_ID,
  loop_name: "implement-tasks",
  run_status: "done",
  nodes: [
    doneNode("slug_input", {
      started_at: "2026-07-05T12:00:01Z",
      ended_at: "2026-07-05T12:00:01Z",
    }),
    doneNode("load_tasks", {
      started_at: "2026-07-05T12:00:02Z",
      ended_at: "2026-07-05T12:00:38Z",
      usage: { tokens: 5_100 },
    }),
    doneNode("implement", {
      started_at: "2026-07-05T12:00:38Z",
      ended_at: "2026-07-05T12:00:39Z",
    }),
    doneNode("execute_task", {
      item_index: 0,
      session_id: "sess_search_reindex_0",
      started_at: "2026-07-05T12:00:39Z",
      ended_at: "2026-07-05T13:20:12Z",
      usage: { tokens: 208_600 },
    }),
    doneNode("execute_task", {
      item_index: 1,
      session_id: "sess_search_reindex_1",
      started_at: "2026-07-05T13:20:12Z",
      ended_at: "2026-07-05T14:35:44Z",
      usage: { tokens: 174_900 },
    }),
    doneNode("execute_task", {
      item_index: 2,
      session_id: "sess_search_reindex_2",
      started_at: "2026-07-05T14:35:44Z",
      ended_at: "2026-07-05T15:39:50Z",
      usage: { tokens: 131_400 },
    }),
    doneNode("collect", {
      started_at: "2026-07-05T15:39:50Z",
      ended_at: "2026-07-05T15:41:00Z",
    }),
  ],
  fanout_rollups: [{ generation: 1, node_id: "execute_task", total: 3, done: 3, failed: 0 }],
  next_cursor: "",
};

export const doneBriefing: LoopBriefing = {
  run_id: DONE_RUN_ID,
  status: "done",
  tone: "ok",
  headline: "Run finished: done. Produced: execute_task[0], execute_task[1].",
  blockers: [],
  outcome: { status: "done", cause: "done", at: "2026-07-05T15:41:00Z" },
  artifacts: [
    {
      name: "execute_task[0]",
      output: "tr_search_reindex_0",
      ref: "loop-output:sha256:7c4e19a2",
      availability: "available",
    },
    {
      // Retention took the payload; the name and the reference outlive it, so
      // the run can still say what it produced.
      name: "execute_task[1]",
      output: "tr_search_reindex_1",
      ref: "loop-output:sha256:1f9ac304",
      availability: "pruned",
    },
  ],
  progress: { round: 1, steps_done: 4, steps_total: 4 },
  usage: { tokens: 520_000, cost_usd: 5.2, budget_used_pct: 26, duration_ms: 13_260_000 },
};

export const doneTimeline: LoopTimelinePage = {
  run_id: DONE_RUN_ID,
  head_seq: 6,
  entries: [
    {
      seq: 6,
      kind: "status_changed",
      generation: 1,
      title: "run status: done",
      at: "2026-07-05T15:41:00Z",
    },
    {
      seq: 5,
      kind: "node_succeeded",
      generation: 1,
      node_id: "execute_task",
      attempt: 1,
      title: "step execute_task succeeded",
      at: "2026-07-05T15:39:50Z",
    },
    {
      seq: 4,
      kind: "node_succeeded",
      generation: 1,
      node_id: "execute_task",
      attempt: 1,
      title: "step execute_task succeeded",
      at: "2026-07-05T14:35:44Z",
    },
    {
      seq: 3,
      kind: "node_succeeded",
      generation: 1,
      node_id: "execute_task",
      attempt: 1,
      title: "step execute_task succeeded",
      at: "2026-07-05T13:20:12Z",
    },
    {
      seq: 2,
      kind: "node_succeeded",
      generation: 1,
      node_id: "load_tasks",
      attempt: 1,
      title: "step load_tasks succeeded",
      at: "2026-07-05T12:00:38Z",
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

// Suite: OS attention projections
// Invariant: badges and bell rows expose only fresh runtime-owned attention;
// loop-node rows appear only when a presence probe is true, never as static chrome.
// Boundary IN: session catalog, task dashboard/list, loop-node probes, attention derivation, and dock adapter.
// Boundary OUT: query polling/stream transport and rendered popover/dock behavior.
import { describe, expect, it } from "vitest";

import type { SessionPayload } from "@/systems/session";
import type { TaskDashboardView, TaskListItem } from "@/systems/tasks";

import { dockBadgeFor } from "../../components/dock-badges";
import { OS_APPS } from "../app-registry";
import {
  attentionCount,
  deriveAttentionBadges,
  deriveAttentionRows,
  type OsLoopNodeAttentionRow,
} from "../attention-model";

const LOOP_NODE_ROWS: OsLoopNodeAttentionRow[] = [
  {
    kind: "loop-node",
    id: "waiting",
    title: "Loop nodes waiting on you",
    state: "waiting",
  },
  {
    kind: "loop-node",
    id: "attention",
    title: "Loop nodes needing attention",
    state: "attention",
  },
];

function session(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    id: "session-1",
    name: "Review runtime permissions",
    agent_name: "codex",
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "codex" },
      selection_revision: 0,
    },
    workspace_id: "workspace-1",
    workspace_path: "/workspace/compozy",
    state: "active",
    badge: "waiting-for-auth",
    attachable: true,
    available_commands: [],
    created_at: "2026-07-20T12:00:00Z",
    updated_at: "2026-07-20T12:01:00Z",
    ...overrides,
    archived_at: overrides.archived_at ?? null,
  };
}

function task(overrides: Partial<TaskListItem> = {}): TaskListItem {
  return {
    id: "task-1",
    title: "Approve deploy",
    identifier: "CompozyOS-42",
    status: "ready",
    scope: "workspace",
    origin: { kind: "web", ref: "operator" },
    created_at: "2026-07-20T12:00:00Z",
    updated_at: "2026-07-20T12:01:00Z",
    created_by: { kind: "human", ref: "operator" },
    approval_state: "pending",
    ...overrides,
  } as TaskListItem;
}

function dashboard(awaitingApprovalTasks: number): TaskDashboardView {
  return {
    totals: { awaiting_approval_tasks: awaitingApprovalTasks },
  } as TaskDashboardView;
}

describe("OS attention model", () => {
  it("Should derive the two dock badges from their exact named projections (UT-062, UT-082)", () => {
    const badges = deriveAttentionBadges({
      sessions: [
        session(),
        session({ id: "session-2", badge: "running" }),
        session({ id: "session-3" }),
      ],
      sessionsStale: false,
      dashboard: dashboard(4),
      tasksStale: false,
    });

    expect(badges).toEqual({ sessions: 2, tasks: 4 });
    expect(dockBadgeFor(OS_APPS.session, badges)).toBe(2);
    expect(dockBadgeFor(OS_APPS.tasks, badges)).toBe(4);
    expect(dockBadgeFor(OS_APPS.dashboard, badges)).toBeUndefined();
    expect(attentionCount(badges)).toBe(6);
  });

  it("Should hide zero and stale badge sources without substituting another count (UT-066, UT-082)", () => {
    expect(
      deriveAttentionBadges({
        sessions: [session()],
        sessionsStale: true,
        dashboard: dashboard(0),
        tasksStale: false,
      })
    ).toEqual({ sessions: undefined, tasks: undefined });

    expect(
      deriveAttentionBadges({
        sessions: [],
        sessionsStale: false,
        dashboard: dashboard(12),
        tasksStale: true,
      })
    ).toEqual({ sessions: undefined, tasks: undefined });
  });

  it("Should aggregate only unresolved fresh rows and reflect live source removal (UT-069)", () => {
    const waiting = session();
    const pending = task();
    const rows = deriveAttentionRows({
      sessions: [waiting, session({ id: "session-running", badge: "running" })],
      sessionRowsStale: false,
      tasks: [pending, task({ id: "task-done", approval_state: "approved" })],
      taskRowsStale: false,
      loopWaitingPresent: false,
      loopAttentionPresent: false,
    });

    expect(rows).toEqual([
      {
        kind: "session",
        id: waiting.id,
        title: waiting.name,
        agentName: waiting.agent_name,
      },
      {
        kind: "task",
        id: pending.id,
        title: pending.title,
        identifier: pending.identifier,
      },
    ]);

    expect(
      deriveAttentionRows({
        sessions: [session({ badge: "running" })],
        sessionRowsStale: false,
        tasks: [task({ approval_state: "approved" })],
        taskRowsStale: false,
        loopWaitingPresent: false,
        loopAttentionPresent: false,
      })
    ).toEqual([]);
  });

  it("Should suppress rows independently when either source is stale", () => {
    expect(
      deriveAttentionRows({
        sessions: [session()],
        sessionRowsStale: true,
        tasks: [task()],
        taskRowsStale: false,
        loopWaitingPresent: false,
        loopAttentionPresent: false,
      })
    ).toEqual([
      {
        kind: "task",
        id: "task-1",
        title: "Approve deploy",
        identifier: "CompozyOS-42",
      },
    ]);
  });

  it("Should omit loop-node rows when both presence probes are empty", () => {
    expect(
      deriveAttentionRows({
        sessions: [],
        sessionRowsStale: false,
        tasks: [],
        taskRowsStale: false,
        loopWaitingPresent: false,
        loopAttentionPresent: false,
      })
    ).toEqual([]);
  });

  it("Should append only the probed loop-node rows and keep their deep-link state", () => {
    expect(
      deriveAttentionRows({
        sessions: [],
        sessionRowsStale: false,
        tasks: [],
        taskRowsStale: false,
        loopWaitingPresent: true,
        loopAttentionPresent: false,
      })
    ).toEqual([LOOP_NODE_ROWS[0]]);

    expect(
      deriveAttentionRows({
        sessions: [],
        sessionRowsStale: false,
        tasks: [],
        taskRowsStale: false,
        loopWaitingPresent: false,
        loopAttentionPresent: true,
      })
    ).toEqual([LOOP_NODE_ROWS[1]]);

    expect(
      deriveAttentionRows({
        sessions: [],
        sessionRowsStale: false,
        tasks: [],
        taskRowsStale: false,
        loopWaitingPresent: true,
        loopAttentionPresent: true,
      })
    ).toEqual(LOOP_NODE_ROWS);
  });
});

// Suite: OS attention projections
// Invariant: session counts come from the daemon's cross-workspace summary and
// never from a row page; bell sections split needs-you from finished-unseen and
// order each by newest transition; a stale source counts for nothing while its
// rows stay jumpable; loop-node rows appear only when a presence probe is true.
// Boundary IN: attention summary, session rows, task dashboard/list, loop-node
// probes, attention derivation, ordering, and the dock adapter.
// Boundary OUT: query polling/stream transport and rendered popover/dock behavior.
import { describe, expect, it } from "vitest";

import type { SessionPayload } from "@/systems/session";
import type { TaskDashboardView, TaskListItem } from "@/systems/tasks";
import { pendingAskRequest } from "@/systems/loops/mocks/fixture-graph-eng-requests";

import { dockBadgeFor } from "../../components/dock-badges";
import { OS_APPS } from "../app-registry";
import {
  attentionCount,
  deriveAttentionBadges,
  deriveAttentionSections,
  type DeriveAttentionSectionsInput,
  type OsLoopNodeAttentionRow,
} from "../attention-model";
import { attentionBand, compareAttentionFirst, sortAttentionFirst } from "../attention-order";

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
    profile_name: "default",
    profile_id: "00000000000000000000000000",
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
    pending_interactions: overrides.pending_interactions ?? [],
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

function sectionsInput(
  overrides: Partial<DeriveAttentionSectionsInput> = {}
): DeriveAttentionSectionsInput {
  return {
    sessions: [],
    sessionRowsStale: false,
    workspaceLabels: new Map([["workspace-1", "compozy"]]),
    mutedWorkspaceIds: new Set(),
    tasks: [],
    taskRowsStale: false,
    loopWaitingPresent: false,
    loopAttentionPresent: false,
    ...overrides,
  };
}

describe("OS attention badges", () => {
  it("Should count sessions from the summary projection, never from row pages", () => {
    // The summary is exact past any page size; a row page is not.
    const badges = deriveAttentionBadges({
      summary: { needsYou: 137, finished: 5 },
      summaryStale: false,
      dashboard: dashboard(4),
      tasksStale: false,
      loopsPending: 7,
    });

    expect(badges).toEqual({ sessions: 137, tasks: 4, loops: 7 });
    expect(dockBadgeFor(OS_APPS.session, badges)).toBe(137);
    expect(dockBadgeFor(OS_APPS.tasks, badges)).toBe(4);
    expect(dockBadgeFor(OS_APPS.dashboard, badges)).toBeUndefined();
    expect(attentionCount(badges)).toBe(148);
  });

  it("Should keep finished-unseen work out of the badge (UT-052)", () => {
    const badges = deriveAttentionBadges({
      summary: { needsYou: 0, finished: 9 },
      summaryStale: false,
      dashboard: dashboard(0),
      tasksStale: false,
    });

    expect(badges).toEqual({ sessions: undefined, tasks: undefined });
    expect(attentionCount(badges)).toBe(0);
  });

  it("Should hide zero and stale badge sources without substituting another count (UT-053)", () => {
    expect(
      deriveAttentionBadges({
        summary: { needsYou: 3, finished: 1 },
        summaryStale: true,
        dashboard: dashboard(0),
        tasksStale: false,
      })
    ).toEqual({ sessions: undefined, tasks: undefined });

    expect(
      deriveAttentionBadges({
        summary: null,
        summaryStale: false,
        dashboard: dashboard(12),
        tasksStale: true,
      })
    ).toEqual({ sessions: undefined, tasks: undefined });
  });
});

describe("OS attention sections", () => {
  it("Should split needs-you from finished and order each by newest transition (UT-052)", () => {
    const auth = session({
      id: "s-auth",
      badge: "waiting-for-auth",
      attention_changed_at: "2026-07-20T12:05:00Z",
    });
    const failed = session({
      id: "s-failed",
      badge: "failed",
      attention_changed_at: "2026-07-20T12:09:00Z",
    });
    const doneOld = session({
      id: "s-done-old",
      badge: "done",
      attention_changed_at: "2026-07-20T11:00:00Z",
    });
    const doneNew = session({
      id: "s-done-new",
      badge: "done",
      attention_changed_at: "2026-07-20T12:30:00Z",
    });
    const running = session({ id: "s-running", badge: "running" });

    const sections = deriveAttentionSections(
      sectionsInput({ sessions: [auth, doneOld, running, failed, doneNew] })
    );

    expect(sections.needsYou.map(row => row.id)).toEqual(["s-failed", "s-auth"]);
    expect(sections.finished.map(row => row.id)).toEqual(["s-done-new", "s-done-old"]);
  });

  it("Should keep task approvals in the needs-you section beside sessions (UT-052)", () => {
    const sections = deriveAttentionSections(
      sectionsInput({
        sessions: [session({ id: "s-auth" })],
        tasks: [task(), task({ id: "task-done", approval_state: "approved" })],
        loopWaitingPresent: true,
      })
    );

    expect(sections.needsYou.map(row => `${row.kind}:${row.id}`)).toEqual([
      "session:s-auth",
      "task:task-1",
      "loop-node:waiting",
    ]);
    expect(sections.finished).toEqual([]);
  });

  it("Should quote the sanitized pending-interaction title as the reason", () => {
    const asking = session({
      id: "s-input",
      badge: "waiting-for-input",
      pending_interactions: [
        {
          interaction_id: "int-1",
          kind: "clarify",
          provider_request_id: "clr-1",
          status: "pending",
          created_at: "2026-07-20T12:00:00Z",
          title: "Should I drop the legacy column?",
        },
      ],
    });

    const sections = deriveAttentionSections(sectionsInput({ sessions: [asking] }));

    expect(sections.needsYou[0]).toMatchObject({
      reason: "Should I drop the legacy column?",
      badge: "waiting-for-input",
    });
  });

  it("Should fall back to the state word rather than invent a reason", () => {
    const sections = deriveAttentionSections(
      sectionsInput({ sessions: [session({ id: "s-failed", badge: "failed" })] })
    );

    expect(sections.needsYou[0]).toMatchObject({ reason: "failed" });
  });

  it("Should label rows by workspace and fall back to the id when the name is unknown", () => {
    const sections = deriveAttentionSections(
      sectionsInput({
        sessions: [
          session({ id: "s-a-known", attention_changed_at: "2026-07-20T12:10:00Z" }),
          session({
            id: "s-b-foreign",
            workspace_id: "workspace-2",
            attention_changed_at: "2026-07-20T12:05:00Z",
          }),
        ],
      })
    );

    expect(
      sections.needsYou.map(row => (row.kind === "session" ? row.workspaceLabel : null))
    ).toEqual(["compozy", "workspace-2"]);
  });

  it("Should mark muted workspaces without removing their rows (US-015.AC-1)", () => {
    const sections = deriveAttentionSections(
      sectionsInput({
        sessions: [session({ id: "s-muted", workspace_id: "workspace-2" })],
        mutedWorkspaceIds: new Set(["workspace-2"]),
      })
    );

    expect(sections.needsYou).toHaveLength(1);
    expect(sections.needsYou[0]).toMatchObject({ muted: true });
  });

  it("Should keep stale rows jumpable while marking them stale (UT-053, US-005.EC-2)", () => {
    // A stale source contributes zero to every count (asserted above), but the
    // operator can still reach the session from a frozen row.
    const sections = deriveAttentionSections(
      sectionsInput({ sessions: [session({ id: "s-auth" })], sessionRowsStale: true })
    );

    expect(sections.needsYou).toHaveLength(1);
    expect(sections.needsYou[0]).toMatchObject({ stale: true });
  });

  it("Should suppress task rows independently when the task source is stale (UT-053)", () => {
    const sections = deriveAttentionSections(
      sectionsInput({
        sessions: [session({ id: "s-auth" })],
        tasks: [task()],
        taskRowsStale: true,
      })
    );

    expect(sections.needsYou.map(row => row.kind)).toEqual(["session"]);
  });

  it("Should append only the probed loop-node rows and keep their deep-link state", () => {
    expect(deriveAttentionSections(sectionsInput()).needsYou).toEqual([]);
    expect(deriveAttentionSections(sectionsInput({ loopWaitingPresent: true })).needsYou).toEqual([
      LOOP_NODE_ROWS[0],
    ]);
    expect(deriveAttentionSections(sectionsInput({ loopAttentionPresent: true })).needsYou).toEqual(
      [LOOP_NODE_ROWS[1]]
    );
    expect(
      deriveAttentionSections(
        sectionsInput({ loopWaitingPresent: true, loopAttentionPresent: true })
      ).needsYou
    ).toEqual(LOOP_NODE_ROWS);
  });

  it("Should project pending loop requests with their exact workspace and jump target", () => {
    const sections = deriveAttentionSections(
      sectionsInput({
        loopRequests: [
          {
            request: pendingAskRequest,
            workspaceId: "workspace-2",
            workspaceLabel: "release",
            stale: false,
          },
        ],
      })
    );

    expect(sections.needsYou).toEqual([
      expect.objectContaining({
        kind: "loop-request",
        workspaceId: "workspace-2",
        workspaceLabel: "release",
        runId: pendingAskRequest.loop_run_id,
        nodeId: pendingAskRequest.node_id,
        requestKind: "ask",
        stale: false,
      }),
    ]);
  });
});

describe("attention-first ordering (UT-066)", () => {
  it("Should band needs-you above finished above working above the rest", () => {
    expect(attentionBand("waiting-for-input")).toBe("needs-you");
    expect(attentionBand("waiting-for-auth")).toBe("needs-you");
    expect(attentionBand("failed")).toBe("needs-you");
    expect(attentionBand("done")).toBe("finished");
    expect(attentionBand("running")).toBe("working");
    expect(attentionBand("idle")).toBe("rest");
    expect(attentionBand("stopped")).toBe("rest");
    expect(attentionBand("unknown")).toBe("rest");

    const ordered = sortAttentionFirst([
      session({ id: "s-idle", badge: "idle" }),
      session({ id: "s-running", badge: "running" }),
      session({ id: "s-done", badge: "done" }),
      session({ id: "s-needs", badge: "waiting-for-input" }),
    ]);

    expect(ordered.map(entry => entry.id)).toEqual(["s-needs", "s-done", "s-running", "s-idle"]);
  });

  it("Should break ties by newest transition, then by session id, stably across re-sorts", () => {
    const older = session({
      id: "s-older",
      badge: "failed",
      attention_changed_at: "2026-07-20T12:00:00Z",
    });
    const newer = session({
      id: "s-newer",
      badge: "failed",
      attention_changed_at: "2026-07-20T12:10:00Z",
    });
    const tieA = session({
      id: "s-aaa",
      badge: "failed",
      attention_changed_at: "2026-07-20T12:10:00Z",
    });

    const once = sortAttentionFirst([older, newer, tieA]).map(entry => entry.id);
    const twice = sortAttentionFirst(sortAttentionFirst([older, newer, tieA])).map(
      entry => entry.id
    );

    expect(once).toEqual(["s-aaa", "s-newer", "s-older"]);
    expect(twice).toEqual(once);
    expect(compareAttentionFirst(tieA, tieA)).toBe(0);
  });

  it("Should fall back to last update when a session carries no transition stamp", () => {
    const stamped = session({
      id: "s-stamped",
      badge: "failed",
      attention_changed_at: "2026-07-20T12:00:00Z",
      updated_at: "2026-07-20T09:00:00Z",
    });
    const unstamped = session({
      id: "s-unstamped",
      badge: "failed",
      updated_at: "2026-07-20T13:00:00Z",
    });

    expect(sortAttentionFirst([stamped, unstamped]).map(entry => entry.id)).toEqual([
      "s-unstamped",
      "s-stamped",
    ]);
  });
});

// Suite: OS attention data readiness
// Invariant: attention rows read cross-workspace and unscoped so no workspace or
// worktree can hide a blocked session, while the sessions modal follows the
// focused window's worktree scope; session counts come from the daemon summary
// and vanish when it is stale rather than reporting a page total.
// Owning layer: OS attention query adapter. Canonical suite: this hook test.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/systems/session/hooks/use-sessions", () => ({ useSessions: vi.fn() }));
// The list preference decides the order the modal query asks for; its own
// round-trip is covered where it lives.
vi.mock("@/systems/session/hooks/use-session-list-preferences", () => ({
  useSessionListPreferences: vi.fn(() => ({
    sort: "last_activity",
    scope: "workspace",
    setSort: vi.fn(),
    setScope: vi.fn(),
    loading: false,
  })),
}));
vi.mock("@/systems/tasks/lib/workspace-scope", () => ({
  taskScopeForActiveWorkspace: vi.fn(),
}));
vi.mock("@/systems/tasks/hooks/use-task-dashboard", () => ({
  useTaskDashboard: vi.fn(),
}));
vi.mock("@/systems/tasks/hooks/use-tasks", () => ({
  useTasks: vi.fn(),
}));
vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: vi.fn(),
}));
vi.mock("@/systems/workspace/hooks/use-active-worktree", () => ({
  useScopedWorktreeFilter: vi.fn(),
}));
vi.mock("../use-worktree-scope", () => ({ useFocusedWorktreeScopeId: vi.fn(() => "window:one") }));
vi.mock("@/systems/loops", () => ({
  useLoopNodeExists: vi.fn(() => false),
  useLoopRequestAttention: vi.fn(() => ({
    pendingCount: 0,
    items: [],
    disconnected: false,
    loading: false,
  })),
}));
// Delegation attention owns its own query suite; this one is about how the shell
// composes it. The default is a quiet source so existing cases are unaffected.
vi.mock("../use-attention-calls", () => ({
  useAttentionCalls: vi.fn(() => ({
    rows: [],
    count: 0,
    coveredSessionIds: new Set<string>(),
    stale: false,
    loading: false,
  })),
}));
// The summary and policy hooks own their own suites; this one is about how the
// shell composes them against the scoped modal queries.
vi.mock("../use-attention-summary", () => ({ useAttentionSummary: vi.fn() }));
vi.mock("../use-attention-policy", () => ({
  useAttentionPolicy: vi.fn(() => ({
    toasts: true,
    sound: true,
    system: false,
    mutedWorkspaceIds: new Set<string>(),
  })),
}));

import { useLoopNodeExists, useLoopRequestAttention } from "@/systems/loops";
import { pendingAskRequest } from "@/systems/loops/mocks/fixture-graph-eng-requests";
import { useAttentionCalls } from "../use-attention-calls";
import { useAttentionSummary } from "../use-attention-summary";
import { useOsAttention } from "../use-os-attention";
import { useSessions, type SessionPayload } from "@/systems/session";
import { taskScopeForActiveWorkspace, useTaskDashboard, useTasks } from "@/systems/tasks";
import {
  useActiveWorkspace,
  useScopedWorktreeFilter,
  type WorkspacePayload,
} from "@/systems/workspace";

const workspace: WorkspacePayload = {
  id: "ws_alpha",
  name: "alpha",
  root_dir: "/workspace/alpha",
  add_dirs: [],
  created_at: "2026-08-04T12:00:00Z",
  updated_at: "2026-08-04T12:00:00Z",
};

function sessionsQuery({
  data = [],
  isError = false,
  isLoading = false,
}: {
  data?: ReturnType<typeof useOsAttention>["sessions"] | undefined;
  isError?: boolean;
  isLoading?: boolean;
}) {
  return { data, isError, isLoading, total: data?.length ?? 0 } as ReturnType<typeof useSessions>;
}

function attentionCallsModel(
  overrides: Partial<ReturnType<typeof useAttentionCalls>> = {}
): ReturnType<typeof useAttentionCalls> {
  return {
    rows: [],
    count: 0,
    coveredSessionIds: new Set<string>(),
    stale: false,
    loading: false,
    ...overrides,
  };
}

/** `badge` is what puts a session in the needs-you class (see session-badge.ts). */
function waitingSession(id: string): SessionPayload {
  return {
    profile_id: "00000000000000000000000000",
    profile_name: "default",
    id,
    agent_name: "atlas",
    runtime: {
      status: "ready",
      transition: "initial_bind",
      effective: { provider: "claude" },
      selection_revision: 0,
    },
    badge: "waiting-for-auth",
    workspace_id: workspace.id,
    workspace_path: workspace.root_dir,
    state: "active",
    attachable: true,
    available_commands: [],
    pending_interactions: [],
    archived_at: null,
    created_at: "2026-08-04T12:00:00Z",
    updated_at: "2026-08-04T12:00:00Z",
  };
}

/** Call order in the hook: needs-you rows, finished rows, modal (scoped). */
const NEEDS_YOU_CALL = 0;
const FINISHED_CALL = 1;
const MODAL_CALL = 2;

function filtersForCall(call: number): Record<string, unknown> {
  const options = vi.mocked(useSessions).mock.calls[call]?.[1];
  return (options?.filters ?? {}) as Record<string, unknown>;
}

function workspaceForCall(call: number): string | null | undefined {
  return vi.mocked(useSessions).mock.calls[call]?.[0];
}

describe("useOsAttention", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useActiveWorkspace).mockReturnValue({
      scope: "workspace",
      activeWorkspaceId: "ws_alpha",
      workspaces: [workspace],
    } as never);
    vi.mocked(useScopedWorktreeFilter).mockReturnValue({ worktreeId: undefined, resolved: true });
    vi.mocked(taskScopeForActiveWorkspace).mockReturnValue({} as never);
    vi.mocked(useTaskDashboard).mockReturnValue({
      data: { freshness: { stale: false }, totals: { awaiting_approval_tasks: 0 } },
      isError: false,
      isLoading: false,
    } as never);
    vi.mocked(useTasks).mockReturnValue({ data: [], isError: false, isLoading: false } as never);
    vi.mocked(useAttentionSummary).mockReturnValue({
      summary: { needsYou: 0, finished: 0 },
      stale: false,
      loading: false,
    });
    vi.mocked(useLoopRequestAttention).mockReturnValue({
      pendingCount: 0,
      items: [],
      disconnected: false,
      loading: false,
    });
  });

  it("Should read the archive only through the modal catalog leg", () => {
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({}));

    renderHook(() => useOsAttention(workspace, "live", true));

    expect(filtersForCall(MODAL_CALL).archive).toBe("only");
    // The archive is a window's content, never an attention signal.
    expect(filtersForCall(NEEDS_YOU_CALL).archive).toBeUndefined();
    expect(filtersForCall(FINISHED_CALL).archive).toBeUndefined();
  });

  it("Should leave the modal catalog on active sessions when the archive is off", () => {
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({}));

    renderHook(() => useOsAttention(workspace, "live", false));

    expect(filtersForCall(MODAL_CALL).archive).toBeUndefined();
  });

  it("Should isolate attention-row failures from the sessions modal catalog", () => {
    vi.mocked(useSessions)
      .mockReturnValueOnce(sessionsQuery({ data: undefined, isError: true }))
      .mockReturnValueOnce(sessionsQuery({}))
      .mockReturnValueOnce(sessionsQuery({}))
      .mockReturnValueOnce(sessionsQuery({}));

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.attentionSessionsDisconnected).toBe(true);
    expect(result.current.sessionsDisconnected).toBe(false);
  });

  it("Should read attention rows cross-workspace and unscoped while the modal follows the worktree", () => {
    vi.mocked(useScopedWorktreeFilter).mockReturnValue({
      worktreeId: "wt_payments",
      resolved: true,
    });
    vi.mocked(useSessions)
      .mockReturnValueOnce(sessionsQuery({ data: [waitingSession("sess_other_worktree")] }))
      .mockReturnValueOnce(sessionsQuery({ data: [] }))
      .mockReturnValueOnce(sessionsQuery({ data: [] }))
      .mockReturnValueOnce(sessionsQuery({ data: [] }));

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    // Attention rows must carry neither a workspace nor a worktree, or a blocked
    // session elsewhere would stop raising a row.
    expect(workspaceForCall(NEEDS_YOU_CALL)).toBeNull();
    expect(workspaceForCall(FINISHED_CALL)).toBeNull();
    expect(filtersForCall(NEEDS_YOU_CALL).worktree).toBeUndefined();
    expect(filtersForCall(NEEDS_YOU_CALL).attention).toBe(true);
    expect(filtersForCall(FINISHED_CALL).badge).toBe("done");
    expect(workspaceForCall(MODAL_CALL)).toBe(workspace.id);
    expect(filtersForCall(MODAL_CALL).worktree).toBe("wt_payments");
    expect(result.current.sections.needsYou.map(row => row.id)).toEqual(["sess_other_worktree"]);
    expect(result.current.sessions).toEqual([]);
  });

  it("Should drop the worktree filter entirely when the scope falls back to the workspace", () => {
    vi.mocked(useScopedWorktreeFilter).mockReturnValue({ worktreeId: undefined, resolved: true });
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [] }));

    renderHook(() => useOsAttention(workspace, "live", false));

    // A missing or unavailable selection sends no filter at all rather than an
    // empty one, so the list matches the fallback notice.
    expect(filtersForCall(MODAL_CALL)).not.toHaveProperty("worktree", expect.anything());
    expect(filtersForCall(MODAL_CALL).worktree).toBeUndefined();
  });

  it("Should count sessions from the summary, not from the rows that happened to load", () => {
    vi.mocked(useAttentionSummary).mockReturnValue({
      summary: { needsYou: 137, finished: 4 },
      stale: false,
      loading: false,
    });
    vi.mocked(useSessions).mockReturnValue(
      sessionsQuery({ data: [waitingSession("sess_page_1")] })
    );

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.badges.sessions).toBe(137);
  });

  it("Should hide the count when the summary is stale rather than report a page total", () => {
    vi.mocked(useAttentionSummary).mockReturnValue({
      summary: { needsYou: 3, finished: 0 },
      stale: true,
      loading: false,
    });
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [waitingSession("sess_stale")] }));

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.badges.sessions).toBeUndefined();
  });

  it("Should surface loop-node rows only when the existence probes are true", () => {
    vi.mocked(useLoopNodeExists).mockImplementation((_workspaceId, state) => state === "waiting");
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [] }));

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.sections.needsYou).toEqual([
      {
        kind: "loop-node",
        id: "waiting",
        title: "Loop nodes waiting on you",
        state: "waiting",
      },
    ]);
    expect(result.current.notificationCount).toBe(0);
  });

  it("Should compose exact healthy loop totals and rows without changing session counts", () => {
    vi.mocked(useAttentionSummary).mockReturnValue({
      summary: { needsYou: 2, finished: 0 },
      stale: false,
      loading: false,
    });
    vi.mocked(useLoopRequestAttention).mockReturnValue({
      pendingCount: 4,
      items: [
        {
          request: pendingAskRequest,
          workspaceId: "ws_alpha",
          workspaceLabel: "alpha",
          stale: false,
        },
      ],
      disconnected: true,
      loading: false,
    });
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [] }));

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.badges).toMatchObject({ sessions: 2, loops: 4 });
    expect(result.current.notificationCount).toBe(6);
    expect(result.current.loopRequestsDisconnected).toBe(true);
    expect(result.current.sections.needsYou[0]).toMatchObject({ kind: "loop-request" });
  });

  it("Should light the Agents badge from the daemon's delegation count", () => {
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [] }));
    vi.mocked(useAttentionCalls).mockReturnValue(
      attentionCallsModel({
        rows: [
          {
            id: "call_bad",
            cause: "invalid-result",
            agentName: "compliance-review-agent",
            rootSessionId: "sess_release_control",
            callId: "call_bad",
            childSessionId: "sess_compliance_review",
            changedAt: "2026-08-20T18:10:00Z",
            count: 1,
          },
        ],
        count: 3,
      })
    );

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.badges.calls).toBe(3);
    expect(result.current.notificationCount).toBe(3);
    expect(result.current.sections.needsYou[0]).toMatchObject({
      kind: "call",
      cause: "invalid-result",
    });
  });

  it("Should render a coalesced tree as one row carrying its real count", () => {
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [] }));
    vi.mocked(useAttentionCalls).mockReturnValue(
      attentionCallsModel({
        rows: [
          {
            id: "tree:sess_migration_sweep",
            cause: "invalid-result",
            agentName: "platform-engineer-agent",
            rootSessionId: "sess_migration_sweep",
            callId: "call_0",
            childSessionId: null,
            changedAt: "2026-08-20T18:10:00Z",
            count: 12,
          },
        ],
        count: 12,
      })
    );

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    // Twelve things need a look, shown as one row — not twelve alarms.
    const callRows = result.current.sections.needsYou.filter(row => row.kind === "call");
    expect(callRows).toHaveLength(1);
    expect(callRows[0]).toMatchObject({
      count: 12,
      title: "12 calls need your look in this tree",
    });
    expect(result.current.badges.calls).toBe(12);
  });

  it("Should show a blocked child once, as the call row that names its tree", () => {
    vi.mocked(useSessions).mockReturnValue(
      sessionsQuery({ data: [waitingSession("sess_compliance_review")] })
    );
    vi.mocked(useAttentionCalls).mockReturnValue(
      attentionCallsModel({
        rows: [
          {
            id: "call_open",
            cause: "blocked-on-decision",
            agentName: "compliance-review-agent",
            rootSessionId: "sess_release_control",
            callId: "call_open",
            childSessionId: "sess_compliance_review",
            changedAt: "2026-08-20T18:10:00Z",
            count: 1,
          },
        ],
        count: 1,
        coveredSessionIds: new Set(["sess_compliance_review"]),
      })
    );

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    const kinds = result.current.sections.needsYou.map(row => row.kind);
    expect(kinds).toContain("call");
    // The bare session row is suppressed: the call row says the same thing with
    // the agent and the tree attached, so one blocked child is one row.
    expect(kinds).not.toContain("session");
  });

  it("Should drop the delegation badge while its source is stale, keeping the row", () => {
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [] }));
    vi.mocked(useAttentionCalls).mockReturnValue(
      attentionCallsModel({
        rows: [
          {
            id: "call_bad",
            cause: "completed-without-result",
            agentName: "copywriter-agent",
            rootSessionId: "sess_marketing_launch_copy",
            callId: "call_bad",
            childSessionId: null,
            changedAt: "2026-08-20T18:10:00Z",
            count: 1,
          },
        ],
        // Already zeroed by the agent-comms layer when its source went stale.
        count: 0,
        stale: true,
      })
    );

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.badges.calls).toBeUndefined();
    expect(result.current.callsDisconnected).toBe(true);
    // The row stays listed and clickable — an old jump target beats none.
    expect(result.current.sections.needsYou[0]).toMatchObject({ kind: "call", stale: true });
  });
});

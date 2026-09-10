// Suite: OS attention data readiness
// Invariant: attention rows read cross-workspace and unscoped so no workspace or
// worktree can hide a blocked session, while the sessions modal follows the
// focused window's worktree scope; session counts come from the daemon summary
// and vanish when it is stale rather than reporting a page total.
// Owning layer: OS attention query adapter. Canonical suite: this hook test.
import { renderHook as renderReactHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-query", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  const queryMock = vi.fn();
  return {
    ...actual,
    useQuery: Object.assign(
      (options: { queryKey: readonly unknown[] }) => {
        if (options.queryKey[0] === "notifications")
          return { data: notificationResponse, isError: notificationStale, isLoading: false };
        return queryMock(options);
      },
      { mockReturnValue: queryMock.mockReturnValue.bind(queryMock) }
    ),
  };
});
vi.mock("@/systems/profiles", () => ({ useProfileReadScope: vi.fn() }));
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
import { useQuery, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement, type ReactNode } from "react";
import type { AttentionNotifications } from "@/systems/notifications";

let notificationResponse: AttentionNotifications;
let notificationStale = false;
function renderHook<T>(callback: () => T) {
  const client = new QueryClient();
  return renderReactHook(callback, {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children),
  });
}

import { useProfileReadScope } from "@/systems/profiles";
import { pendingAskRequest } from "@/systems/loops/mocks/fixture-graph-eng-requests";
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

/** `badge` is what puts a session in the needs-you class (see session-badge.ts). */
function waitingSession(id: string): SessionPayload {
  return {
    supervision: null,
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
    notificationResponse = { snapshot: "snapshot", total: 0, needs_you: 0, finished: 0, items: [] };
    notificationStale = false;
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
    vi.mocked(useProfileReadScope).mockReturnValue({
      destination: "work",
      destinationOwner: { id: "profile-work" },
    } as never);
    vi.mocked(useQuery).mockReturnValue({
      data: { pending: [], resolved: [] },
      isError: false,
      isLoading: false,
    } as never);
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
    vi.mocked(useLoopNodeExists).mockImplementation(() => false);
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

  it("Should isolate notification failures from the sessions modal catalog", () => {
    notificationStale = true;
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
    expect(result.current.badges.sessions).toBeUndefined();
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

  it("Should use unread notification counts independently of source attention", () => {
    vi.mocked(useLoopNodeExists).mockImplementation(() => true);
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [] }));
    notificationResponse = {
      snapshot: "snapshot",
      total: 230,
      needs_you: 230,
      finished: 0,
      items: [
        {
          id: "occurrence",
          kind: "loop-node",
          source_id: "node",
          workspace_id: "ws-other",
          workspace_label: "other",
          title: "Waiting node",
          detail: "waiting",
          occurred_at: "2026-09-10T12:00:00Z",
          run_id: "run",
          item_index: 0,
          generation: 0,
          redacted: false,
          finished: false,
        },
      ],
    };
    const { result } = renderHook(() => useOsAttention(workspace, "live", false));
    expect(result.current.sections.needsYou).toEqual([
      expect.objectContaining({
        kind: "loop-node",
        notificationId: "occurrence",
        workspaceId: "ws-other",
      }),
    ]);
    expect(result.current.notificationCount).toBe(230);
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
    expect(result.current.notificationCount).toBe(0);
    expect(result.current.loopRequestsDisconnected).toBe(false);
    expect(result.current.sections.needsYou).toEqual([]);
  });

  it("Should count only terminal approvals owned by the current workspace and profile", () => {
    const current = waitingSession("sess-current");
    current.profile_id = "profile-work";
    current.pending_interactions = [
      {
        interaction_id: "interaction-terminal",
        kind: "permission",
        provider_request_id: "request-terminal",
        title: "Terminal Exec",
        tool_id: "compozy__terminal_exec",
        status: "pending",
        created_at: "2026-08-25T12:00:00Z",
      },
      {
        interaction_id: "interaction-other",
        kind: "permission",
        provider_request_id: "request-other",
        title: "Workspace Update",
        tool_id: "compozy__workspace_update",
        status: "pending",
        created_at: "2026-08-25T12:00:00Z",
      },
    ];
    const foreign = waitingSession("sess-foreign");
    foreign.workspace_id = "workspace-foreign";
    foreign.profile_id = "profile-personal";
    foreign.pending_interactions = [
      { ...current.pending_interactions[0]!, interaction_id: "foreign" },
    ];
    vi.mocked(useSessions)
      .mockReturnValueOnce(sessionsQuery({ data: [current, foreign] }))
      .mockReturnValueOnce(sessionsQuery({ data: [] }))
      .mockReturnValueOnce(sessionsQuery({ data: [] }));

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.badges.terminal).toBe(1);
  });

  it("Should preserve terminal source badges after all notifications are acknowledged", () => {
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [] }));
    vi.mocked(useQuery).mockReturnValue({
      data: {
        pending: [
          {
            id: "req-3f8a",
            terminal_id: "term-9cd7e14b2a66",
            profile_id: "profile-work",
            profile_name: "work",
            reason: "I need the staging database password",
            prompt_excerpt: "Password for user atlas:",
            redacted: true,
            requested_at: "2026-08-25T12:44:00Z",
            requester: { kind: "agent", id: "claude-code" },
          },
        ],
        resolved: [],
      },
      isError: false,
      isLoading: false,
    } as never);

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.badges.terminal).toBe(1);
    expect(result.current.notificationCount).toBe(0);
    expect(result.current.sections.needsYou).toEqual([]);
  });

  it("Should keep a healthy terminal count when the session stream is stale", () => {
    vi.mocked(useAttentionSummary).mockReturnValue({
      summary: { needsYou: 3, finished: 0 },
      stale: true,
      loading: false,
    });
    vi.mocked(useSessions).mockReturnValue(sessionsQuery({ data: [] }));
    vi.mocked(useQuery).mockReturnValue({
      data: {
        pending: [
          {
            id: "req-3f8a",
            terminal_id: "term-9cd7e14b2a66",
            profile_id: "profile-work",
            profile_name: "work",
            reason: "I need the staging database password",
            prompt_excerpt: "Password for user atlas:",
            redacted: true,
            requested_at: "2026-08-25T12:44:00Z",
            requester: { kind: "agent", id: "claude-code" },
          },
        ],
        resolved: [],
      },
      isError: false,
      isLoading: false,
    } as never);

    const { result } = renderHook(() => useOsAttention(workspace, "stale", false));

    expect(result.current.badges.sessions).toBeUndefined();
    expect(result.current.badges.terminal).toBe(1);
    expect(result.current.notificationCount).toBe(0);
  });
});

// Suite: OS attention data readiness
// Invariant: ledger notifications remain independent of the scoped modal page;
// exact session/terminal counters come from authoritative projections rather
// than hydrated history or page lengths, and keep their workspace/profile scope.
// Owning layer: OS attention query adapter. Canonical suite: this hook test.
import { act, renderHook as renderReactHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { DataScope } from "@/systems/workspace";

vi.mock("@tanstack/react-query", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  const queryMock = vi.fn();
  return {
    ...actual,
    useQuery: Object.assign(
      vi.fn((options: { queryKey: readonly unknown[] }) => {
        if (options.queryKey.includes("attention-summary")) {
          if (realSummaryQueries) return actual.useQuery(options);
          return { data: terminalMetadata, isError: terminalMetadataError, isLoading: false };
        }
        if (options.queryKey[0] === "notifications")
          return { data: notificationResponse, isError: notificationStale, isLoading: false };
        return queryMock(options);
      }),
      { mockReturnValue: queryMock.mockReturnValue.bind(queryMock) }
    ),
  };
});
vi.mock("@/systems/session/adapters/session-attention-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/session/adapters/session-attention-api")>()),
  fetchSessionAttentionSummary: vi.fn(),
}));

vi.mock("@/systems/profiles", () => ({ useProfileReadScope: vi.fn() }));
vi.mock("@/systems/session/hooks/use-session-catalog", () => ({ useSessionCatalog: vi.fn() }));
// The daemon seed decides the order the modal query asks for while the followed window has
// no choice of its own; the per-window state is covered where it lives.
vi.mock("@/systems/session/hooks/use-session-list-preferences", () => ({
  useSessionListPreferences: vi.fn(() => ({
    sort: "last_activity",
    groupBy: "none",
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
let terminalMetadata: { facets: { terminal_approvals: number } } | undefined;
let terminalMetadataError = false;
let realSummaryQueries = false;
function renderHook<T>(callback: () => T, client = new QueryClient()) {
  return renderReactHook(callback, {
    wrapper: ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children),
  });
}

import { useProfileReadScope } from "@/systems/profiles";
import { pendingAskRequest } from "@/systems/loops/mocks/fixture-graph-eng-requests";
import { useAttentionSummary } from "../use-attention-summary";
import { fetchSessionAttentionSummary } from "@/systems/session/adapters/session-attention-api";
import { useOsAttention } from "../use-os-attention";
import {
  sessionAttentionSummaryOptions,
  useSessionCatalog,
  type SessionCatalogStreamStatus,
  type SessionPayload,
} from "@/systems/session";
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
  return {
    facets: undefined,
    total: undefined,
    sessions: data ?? [],
    failed: isError || data === undefined,
    loading: isLoading,
    placeholder: false,
    next: false,
    previous: false,
    paging: false,
    nextPage: vi.fn(),
    previousPage: vi.fn(),
    retry: vi.fn(),
  } as ReturnType<typeof useSessionCatalog>;
}

/** `badge` is what puts a session in the needs-you class (see session-badge.ts). */
function waitingSession(id: string): SessionPayload {
  return {
    board_column: "done",
    pull_request: null,
    pull_request_lookup: null,
    supervision: null,
    profile_id: "00000000000000000000000000",
    profile_name: "default",
    scope: "workspace",
    effective_workspace_access: "",
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
const MODAL_CALL = 0;

function filtersForCall(call: number): Record<string, unknown> {
  const options = vi.mocked(useSessionCatalog).mock.calls[call]?.[1];
  return (options ?? {}) as Record<string, unknown>;
}

function workspaceForCall(call: number): DataScope | null | undefined {
  return vi.mocked(useSessionCatalog).mock.calls[call]?.[0];
}

describe("useOsAttention", () => {
  afterEach(() => {
    realSummaryQueries = false;
    vi.useRealTimers();
  });
  beforeEach(() => {
    vi.clearAllMocks();
    notificationResponse = { snapshot: "snapshot", total: 0, needs_you: 0, finished: 0, items: [] };
    notificationStale = false;
    terminalMetadata = { facets: { terminal_approvals: 0 } };
    terminalMetadataError = false;
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({}));
    vi.mocked(useActiveWorkspace).mockReturnValue({
      dataScope: { kind: "project", workspaceId: "ws_alpha" },
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
    vi.mocked(useAttentionSummary).mockImplementation(() => ({
      terminalApprovals:
        terminalMetadataError || !terminalMetadata
          ? undefined
          : [
              {
                workspace_id: workspace.id,
                profile_id: "profile-work",
                count: terminalMetadata.facets.terminal_approvals,
              },
            ],
      summary: { needsYou: 0, finished: 0 },
      stale: false,
      loading: false,
    }));
    vi.mocked(useLoopRequestAttention).mockReturnValue({
      pendingCount: 0,
      items: [],
      disconnected: false,
      loading: false,
    });
    vi.mocked(useLoopNodeExists).mockImplementation(() => false);
  });

  it("Should read the archive only through the modal catalog leg", () => {
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({}));

    renderHook(() => useOsAttention(workspace, "live", true));

    expect(filtersForCall(MODAL_CALL).archive).toBe("only");
    // The archive is a window's content, never an attention signal.
    expect(vi.mocked(useSessionCatalog)).toHaveBeenCalledTimes(1);
  });

  it("Should leave the modal catalog on active sessions when the archive is off", () => {
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({}));

    renderHook(() => useOsAttention(workspace, "live", false));

    expect(filtersForCall(MODAL_CALL).archive).toBeUndefined();
  });

  it("Should keep session navigation loading separate from loss of updates", () => {
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({}));
    let status: SessionCatalogStreamStatus = "live";
    const { result, rerender } = renderHook(() => useOsAttention(workspace, status, false));
    expect(result.current.sessionsDisconnected).toBe(false);

    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({ isLoading: true }));
    status = "live";
    rerender();
    expect(result.current.sessionsDisconnected).toBe(false);
    status = "disabled";
    rerender();
    expect(result.current.sessionsDisconnected).toBe(false);
    status = "connecting";
    rerender();
    expect(result.current.sessionsDisconnected).toBe(false);
    status = "stale";
    rerender();
    expect(result.current.sessionsDisconnected).toBe(true);
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({}));
    status = "live";
    rerender();
    expect(result.current.sessionsDisconnected).toBe(false);
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({ isError: true }));
    status = "live";
    rerender();
    expect(result.current.sessionsDisconnected).toBe(true);
  });

  it("Should isolate notification failures from the sessions modal catalog", () => {
    notificationStale = true;
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({}));

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.attentionSessionsDisconnected).toBe(true);
    expect(result.current.sessionsDisconnected).toBe(false);
  });

  it("Should keep ledger attention independent while modal follows the selected worktree", () => {
    vi.mocked(useScopedWorktreeFilter).mockReturnValue({
      worktreeId: "wt_payments",
      resolved: true,
    });
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({}));
    const { result } = renderHook(() => useOsAttention(workspace, "live", false));
    expect(workspaceForCall(MODAL_CALL)).toEqual({ kind: "project", workspaceId: workspace.id });
    expect(filtersForCall(MODAL_CALL).worktree).toBe("wt_payments");
    expect(vi.mocked(useSessionCatalog)).toHaveBeenCalledTimes(1);
    expect(result.current.sections.needsYou).toEqual([]);
  });

  it("Should enable All projects session and attention catalogs without a project workspace", () => {
    vi.mocked(useActiveWorkspace).mockReturnValue({
      dataScope: { kind: "all" },
      activeWorkspaceId: null,
      workspaces: [workspace],
    } as never);
    vi.mocked(useSessionCatalog).mockReturnValue(
      sessionsQuery({ data: [waitingSession("sess_global")] })
    );

    terminalMetadata = { facets: { terminal_approvals: 237 } };
    const { result } = renderHook(() => useOsAttention(null, "live", false));

    for (const call of [MODAL_CALL]) {
      expect(workspaceForCall(call)).toEqual({ kind: "all" });
      expect(vi.mocked(useSessionCatalog).mock.calls[call]?.[2]).toBe(true);
      expect(filtersForCall(call).worktree).toBeUndefined();
    }
    expect(result.current.sessions).toHaveLength(1);
    expect(result.current.sessionsDisconnected).toBe(false);
    expect(result.current.badges.terminal).toBeUndefined();
  });

  // Invariant (W-06): under User the modal and task attention read User only — never the
  // aggregate — and no worktree filter applies.
  it("Should read User sessions and tasks under User focus with no worktree filter", () => {
    vi.mocked(useActiveWorkspace).mockReturnValue({
      dataScope: { kind: "user" },
      activeWorkspaceId: null,
      workspaces: [workspace],
    } as never);
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({}));

    renderHook(() => useOsAttention(null, "live", false));

    expect(workspaceForCall(MODAL_CALL)).toEqual({ kind: "user" });
    expect(vi.mocked(useScopedWorktreeFilter).mock.calls.at(-1)?.[2]).toEqual({ enabled: false });
    expect(vi.mocked(taskScopeForActiveWorkspace)).toHaveBeenLastCalledWith({ kind: "user" });
  });

  it("Should drop the worktree filter entirely when the scope falls back to the workspace", () => {
    vi.mocked(useScopedWorktreeFilter).mockReturnValue({ worktreeId: undefined, resolved: true });
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({ data: [] }));

    renderHook(() => useOsAttention(workspace, "live", false));

    // A missing or unavailable selection sends no filter at all rather than an
    // empty one, so the list matches the fallback notice.
    expect(filtersForCall(MODAL_CALL)).not.toHaveProperty("worktree", expect.anything());
    expect(filtersForCall(MODAL_CALL).worktree).toBeUndefined();
  });

  it("Should count sessions from the summary, not from the rows that happened to load", () => {
    vi.mocked(useAttentionSummary).mockReturnValue({
      terminalApprovals: [],
      summary: { needsYou: 137, finished: 4 },
      stale: false,
      loading: false,
    });
    vi.mocked(useSessionCatalog).mockReturnValue(
      sessionsQuery({ data: [waitingSession("sess_page_1")] })
    );

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.badges.sessions).toBe(137);
  });

  it("Should hide the count when the summary is stale rather than report a page total", () => {
    vi.mocked(useAttentionSummary).mockReturnValue({
      terminalApprovals: [],
      summary: { needsYou: 3, finished: 0 },
      stale: true,
      loading: false,
    });
    vi.mocked(useSessionCatalog).mockReturnValue(
      sessionsQuery({ data: [waitingSession("sess_stale")] })
    );

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.badges.sessions).toBeUndefined();
  });

  it("Should use unread notification counts independently of source attention", () => {
    vi.mocked(useLoopNodeExists).mockImplementation(() => true);
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({ data: [] }));
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

  it("Should title an untitled session occurrence as its window does, never by its raw id", () => {
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({ data: [] }));
    notificationResponse = {
      snapshot: "snapshot",
      total: 1,
      needs_you: 1,
      finished: 0,
      items: [
        {
          id: "occurrence",
          kind: "session",
          source_id: "sess-fc79f940924ca862",
          workspace_id: "ws-other",
          workspace_label: "other",
          title: "sess-fc79f940924ca862",
          detail: "Edit rollout plan",
          badge: "waiting-for-permission",
          agent_name: "walk-agent",
          occurred_at: "2026-09-10T12:00:00Z",
          item_index: 0,
          generation: 0,
          redacted: false,
          finished: false,
        },
      ],
    };
    const { result } = renderHook(() => useOsAttention(workspace, "live", false));
    expect(result.current.sections.needsYou).toEqual([
      expect.objectContaining({ kind: "session", title: "New session" }),
    ]);
  });

  it("Should compose exact healthy loop totals and rows without changing session counts", () => {
    vi.mocked(useAttentionSummary).mockReturnValue({
      terminalApprovals: [],
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
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({ data: [] }));

    const { result } = renderHook(() => useOsAttention(workspace, "live", false));

    expect(result.current.badges).toMatchObject({ sessions: 2, loops: 4 });
    expect(result.current.notificationCount).toBe(0);
    expect(result.current.loopRequestsDisconnected).toBe(false);
    expect(result.current.sections.needsYou).toEqual([]);
  });

  it("Should preserve an exact terminal approval total beyond any rich session page", () => {
    terminalMetadata = { facets: { terminal_approvals: 237 } };
    const { result } = renderHook(() => useOsAttention(workspace, "live", false));
    expect(result.current.badges.terminal).toBe(237);
    expect(vi.mocked(useSessionCatalog)).toHaveBeenCalledTimes(1);
    expect(filtersForCall(MODAL_CALL).attention).toBeUndefined();
    expect(filtersForCall(MODAL_CALL).badge).toBeUndefined();
  });

  // Invariant: live catalog wakes are the sole shared attention-summary refresh owner;
  // disconnected polling still refreshes the exact projection and backs off errors.
  // Layer: OS attention/query integration. Canonical suite: this existing hook suite.
  it("Should refresh the shared summary on live wakes without a second poll and retain fallback", async () => {
    vi.useFakeTimers();
    realSummaryQueries = true;
    const actual = await vi.importActual<typeof import("../use-attention-summary")>(
      "../use-attention-summary"
    );
    vi.mocked(useAttentionSummary).mockImplementation(actual.useAttentionSummary);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(fetchSessionAttentionSummary).mockResolvedValue({
      needs_you: 0,
      finished: 0,
      by_workspace: [],
      terminal_approvals: [{ workspace_id: workspace.id, profile_id: "profile-work", count: 1 }],
    });
    let status: SessionCatalogStreamStatus = "live";
    const { result, rerender, unmount } = renderHook(
      () => useOsAttention(workspace, status, false),
      client
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(1);
    expect(result.current.badges.terminal).toBe(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(6_000);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(1);
    vi.mocked(fetchSessionAttentionSummary).mockResolvedValue({
      needs_you: 0,
      finished: 0,
      by_workspace: [],
      terminal_approvals: [{ workspace_id: workspace.id, profile_id: "profile-work", count: 2 }],
    });
    await act(async () => {
      await client.invalidateQueries({
        queryKey: sessionAttentionSummaryOptions().queryKey,
        exact: true,
      });
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(2);
    expect(result.current.badges.terminal).toBe(2);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(6_000);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(2);
    status = "stale";
    rerender();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_001);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(3);
    vi.mocked(fetchSessionAttentionSummary).mockRejectedValue(new Error("summary unavailable"));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_001);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(4);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(29_000);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(4);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_001);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(5);
    unmount();
    client.clear();
  });

  // Invariant: a failed summary read recovers in a quiet live workspace
  // without polling healthy siblings or requiring another lifecycle event.
  // Layer: OS attention/query integration; canonical suite: this hook suite.
  it("Should recover a failed summary read after quiet live-stream backoff", async () => {
    vi.useFakeTimers();
    realSummaryQueries = true;
    const actual = await vi.importActual<typeof import("../use-attention-summary")>(
      "../use-attention-summary"
    );
    vi.mocked(useAttentionSummary).mockImplementation(actual.useAttentionSummary);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(fetchSessionAttentionSummary).mockRejectedValue(new Error("summary unavailable"));
    const { result, unmount } = renderHook(() => useOsAttention(workspace, "live", false), client);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(1);
    expect(result.current.badges.terminal).toBeUndefined();
    vi.mocked(fetchSessionAttentionSummary).mockResolvedValue({
      needs_you: 0,
      finished: 0,
      by_workspace: [],
      terminal_approvals: [{ workspace_id: workspace.id, profile_id: "profile-work", count: 3 }],
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(29_000);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_001);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(2);
    expect(result.current.badges.terminal).toBe(3);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(6_000);
    });
    expect(fetchSessionAttentionSummary).toHaveBeenCalledTimes(2);
    unmount();
    client.clear();
  });

  it("Should scope terminal approval metadata to the destination profile even in aggregate view", () => {
    vi.mocked(useProfileReadScope).mockReturnValue({
      destination: "work",
      destinationOwner: { id: "profile-work" },
      aggregate: true,
      params: { all_profiles: true },
    } as never);
    vi.mocked(useAttentionSummary).mockReturnValue({
      summary: { needsYou: 300, finished: 0 },
      stale: false,
      loading: false,
      terminalApprovals: [
        { workspace_id: workspace.id, profile_id: "profile-work", count: 7 },
        { workspace_id: workspace.id, profile_id: "profile-other", count: 100 },
        { workspace_id: "ws-other", profile_id: "profile-work", count: 200 },
      ],
    });
    const { result } = renderHook(() => useOsAttention(workspace, "live", false));
    expect(result.current.badges.terminal).toBe(7);
    expect(
      vi.mocked(useQuery).mock.calls.some(([options]) => options.queryKey?.includes("facets"))
    ).toBe(false);
  });

  it("Should hide a stale terminal projection without borrowing a rich page count", () => {
    terminalMetadata = undefined;
    terminalMetadataError = true;
    vi.mocked(useSessionCatalog).mockReturnValue(
      sessionsQuery({ data: [waitingSession("unrelated")] })
    );
    const { result } = renderHook(() => useOsAttention(workspace, "live", false));
    expect(result.current.badges.terminal).toBeUndefined();
  });

  it("Should select User approvals without including project approvals", () => {
    vi.mocked(useActiveWorkspace).mockReturnValue({
      dataScope: { kind: "user" },
      activeWorkspaceId: null,
      workspaces: [workspace],
    } as never);
    vi.mocked(useAttentionSummary).mockReturnValue({
      summary: { needsYou: 104, finished: 0 },
      stale: false,
      loading: false,
      terminalApprovals: [
        { workspace_id: "", profile_id: "profile-work", count: 4 },
        { workspace_id: workspace.id, profile_id: "profile-work", count: 100 },
      ],
    });
    const { result } = renderHook(() => useOsAttention(null, "live", false));
    expect(result.current.badges.terminal).toBe(4);
  });

  it("Should preserve terminal source badges after all notifications are acknowledged", () => {
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({ data: [] }));
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
      terminalApprovals: [],
      summary: { needsYou: 3, finished: 0 },
      stale: true,
      loading: false,
    });
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({ data: [] }));
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

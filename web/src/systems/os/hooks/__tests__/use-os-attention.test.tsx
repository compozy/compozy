// Suite: OS attention data readiness
// Invariant: ledger notifications remain independent of the scoped modal page;
// exact session/terminal counters come from authoritative projections rather
// than hydrated history or page lengths, and keep their workspace/profile scope.
// Owning layer: OS attention query adapter. Canonical suite: this hook test.
import { renderHook as renderReactHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-query", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  const queryMock = vi.fn();
  return {
    ...actual,
    useQuery: Object.assign(
      vi.fn((options: { queryKey: readonly unknown[] }) => {
        if (options.queryKey.includes("facets"))
          return { data: terminalMetadata, isError: terminalMetadataError, isLoading: false };
        if (options.queryKey[0] === "notifications")
          return { data: notificationResponse, isError: notificationStale, isLoading: false };
        return queryMock(options);
      }),
      { mockReturnValue: queryMock.mockReturnValue.bind(queryMock) }
    ),
  };
});
vi.mock("@/systems/profiles", () => ({ useProfileReadScope: vi.fn() }));
vi.mock("@/systems/session/hooks/use-session-catalog", () => ({ useSessionCatalog: vi.fn() }));
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
let terminalMetadata: { facets: { terminal_approvals: number } } | undefined;
let terminalMetadataError = false;
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
import { useSessionCatalog, type SessionPayload } from "@/systems/session";
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
const MODAL_CALL = 0;

function filtersForCall(call: number): Record<string, unknown> {
  const options = vi.mocked(useSessionCatalog).mock.calls[call]?.[1];
  return (options ?? {}) as Record<string, unknown>;
}

function workspaceForCall(call: number): string | null | undefined {
  return vi.mocked(useSessionCatalog).mock.calls[call]?.[0];
}

describe("useOsAttention", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    notificationResponse = { snapshot: "snapshot", total: 0, needs_you: 0, finished: 0, items: [] };
    notificationStale = false;
    terminalMetadata = { facets: { terminal_approvals: 0 } };
    terminalMetadataError = false;
    vi.mocked(useSessionCatalog).mockReturnValue(sessionsQuery({}));
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
    expect(workspaceForCall(MODAL_CALL)).toBe(workspace.id);
    expect(filtersForCall(MODAL_CALL).worktree).toBe("wt_payments");
    expect(vi.mocked(useSessionCatalog)).toHaveBeenCalledTimes(1);
    expect(result.current.sections.needsYou).toEqual([]);
  });

  it("Should enable Global session and attention catalogs without a project workspace", () => {
    vi.mocked(useActiveWorkspace).mockReturnValue({
      scope: "global",
      activeWorkspaceId: null,
      workspaces: [workspace],
    } as never);
    vi.mocked(useSessionCatalog).mockReturnValue(
      sessionsQuery({ data: [waitingSession("sess_global")] })
    );

    terminalMetadata = { facets: { terminal_approvals: 237 } };
    const { result } = renderHook(() => useOsAttention(null, "live", false));

    for (const call of [MODAL_CALL]) {
      expect(workspaceForCall(call)).toBeNull();
      expect(vi.mocked(useSessionCatalog).mock.calls[call]?.[2]).toBe(true);
      expect(filtersForCall(call).worktree).toBeUndefined();
    }
    expect(result.current.sessions).toHaveLength(1);
    expect(result.current.sessionsDisconnected).toBe(false);
    expect(result.current.badges.terminal).toBeUndefined();
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

  it("Should scope terminal approval metadata to the destination profile even in aggregate view", () => {
    vi.mocked(useProfileReadScope).mockReturnValue({
      destination: "work",
      destinationOwner: { id: "profile-work" },
      aggregate: true,
      params: { all_profiles: true },
    } as never);
    renderHook(() => useOsAttention(workspace, "live", false));
    const facetsCall = vi
      .mocked(useQuery)
      .mock.calls.find(([options]) => options.queryKey?.includes("facets"));
    expect(facetsCall?.[0].queryKey?.at(-1)).toEqual(
      expect.objectContaining({ workspace_id: workspace.id, profile: "work" })
    );
    expect(facetsCall?.[0].queryKey?.at(-1)).not.toHaveProperty("all_profiles", true);
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

import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockNavigate,
  mockUseAgent,
  mockUseAgentSessions,
  mockUseAgentCatalogMetrics,
  mockUseSessionLifecycleActions,
} = vi.hoisted(() => ({
  mockNavigate: vi.fn<(input: unknown) => Promise<void>>(),
  mockUseAgent: vi.fn(),
  mockUseAgentSessions: vi.fn(),
  mockUseAgentCatalogMetrics: vi.fn(),
  mockUseSessionLifecycleActions: vi.fn(),
}));

let mockActiveWorkspaceId: string | null = "ws_alpha";
let mockRuntimeWorkspaceId: string | null = "ws_alpha";
let mockDataScope: { kind: "project"; workspaceId: string } | { kind: "user" } | { kind: "all" } = {
  kind: "project",
  workspaceId: "ws_alpha",
};

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock("@/systems/agent/hooks/use-agents", () => ({
  useAgent: (name: string, workspace?: string | null) => mockUseAgent(name, workspace),
}));

vi.mock("@/systems/agent/hooks/use-agent-sessions", () => ({
  useAgentSessions: (workspaceId: string | null, agentName: string) =>
    mockUseAgentSessions(workspaceId, agentName),
}));

vi.mock("@/systems/agent/hooks/use-agent-catalog-metrics", () => ({
  useAgentCatalogMetrics: (workspaceId: string | null, agentName: string) =>
    mockUseAgentCatalogMetrics(workspaceId, agentName),
}));

vi.mock("@/systems/agent/hooks/use-agent-create-host", () => ({
  useAgentCreateHost: () => ({
    openDialog: vi.fn(),
    openForDuplicate: vi.fn(),
  }),
}));

vi.mock("@/systems/agent/hooks/use-agent-delete-flow", () => ({
  useAgentDeleteFlow: () => ({
    open: false,
    openDialog: vi.fn(),
    closeDialog: vi.fn(),
    confirmDialog: null,
    isDeleting: false,
  }),
}));

vi.mock("@/systems/session/hooks/use-session-create", () => ({
  useSessionCreateActions: () => ({ openForAgent: vi.fn() }),
  useSessionCreateHasActiveWorkspace: () => true,
  useSessionCreateIsCreating: () => false,
  useSessionCreatePendingAgentName: () => null,
}));

vi.mock("../../../hooks/use-session-list-lifecycle-actions", () => ({
  useSessionListLifecycleActions: () => mockUseSessionLifecycleActions(),
}));

vi.mock("@/systems/session/hooks/use-session-lifecycle-actions", () => ({
  useSessionLifecycleActions: () => mockUseSessionLifecycleActions(),
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: mockActiveWorkspaceId,
    runtimeWorkspaceId: mockRuntimeWorkspaceId,
    registeredWorkspaces: [{ id: "ws_beta", name: "beta" }],
  }),
  useDataScope: () => mockDataScope,
}));

import { agentDetailTabItems, agentStatusSlot } from "@/systems/os/apps/agents/agent-detail-view";
import {
  useAgentDetailPage,
  type UseAgentDetailResult,
} from "@/systems/os/apps/agents/use-agent-detail";
import { WindowDataBindingContext } from "@/systems/workspace";

const emptySearch = {};

describe("useAgentDetailPage", () => {
  beforeEach(() => {
    mockActiveWorkspaceId = "ws_alpha";
    mockRuntimeWorkspaceId = "ws_alpha";
    mockDataScope = { kind: "project", workspaceId: "ws_alpha" };
    mockNavigate.mockReset();
    mockUseAgent.mockReset();
    mockUseAgentSessions.mockReset();
    mockUseAgentCatalogMetrics.mockReset();
    mockUseSessionLifecycleActions.mockReset();
    mockUseAgent.mockReturnValue({
      data: { name: "categorized-multi", provider: "codex", prompt: "prompt" },
      error: null,
      isLoading: false,
    });
    mockUseAgentSessions.mockReturnValue({
      sessions: [],
      hasMore: true,
      isLoadingMore: false,
      loadMore: vi.fn(),
      isError: false,
      isLoading: false,
    });
    mockUseAgentCatalogMetrics.mockReturnValue({
      total: 42,
      active: 3,
      failed: 1,
      runtimeSeconds: 3600,
      lastActivityAt: "2026-07-11T12:00:00Z",
      sessionsAvailable: true,
      isLoading: false,
      isError: false,
    });
    mockUseSessionLifecycleActions.mockReturnValue({
      actions: {
        pendingAction: null,
        pendingSessionId: null,
        onStop: vi.fn(),
        onArchive: vi.fn(),
        onUnarchive: vi.fn(),
        onDelete: vi.fn(),
      },
      deleteDialog: {
        open: false,
        session: null,
        isDeleting: false,
        onOpenChange: vi.fn(),
        onConfirm: vi.fn(),
      },
    });
  });

  it("Should load agent details in the active workspace context", () => {
    const { result } = renderHook(() => useAgentDetailPage("categorized-multi", emptySearch));

    expect(mockUseAgent).toHaveBeenCalledWith("categorized-multi", "ws_alpha");
    expect(mockUseAgentSessions).toHaveBeenCalledWith("ws_alpha", "categorized-multi");
    expect(mockUseAgentCatalogMetrics).toHaveBeenCalledWith("ws_alpha", "categorized-multi");
    expect(result.current.sessionActions).toBe(
      mockUseSessionLifecycleActions.mock.results[0]?.value.actions
    );
    expect(result.current.search).toEqual({
      tab: "overview",
      file: "agent",
      filter: "all",
    });
  });

  it("Should read User agent details through @user under User focus", () => {
    mockActiveWorkspaceId = null;
    mockRuntimeWorkspaceId = null;
    mockDataScope = { kind: "user" };

    renderHook(() => useAgentDetailPage("global-agent", emptySearch));

    expect(mockUseAgent).toHaveBeenCalledWith("global-agent", "@user");
    expect(mockUseAgentSessions).toHaveBeenCalledWith("@user", "global-agent");
  });

  // All projects: an agent opened in place reads its own project through the
  // window binding and keeps that binding across in-window navigation.
  it("Should read an agent opened in place through its window binding", () => {
    mockActiveWorkspaceId = null;
    mockRuntimeWorkspaceId = null;
    mockDataScope = { kind: "all" };
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(
        WindowDataBindingContext,
        { value: { partition: "global", binding: "ws_beta" } },
        children
      );

    const { result } = renderHook(() => useAgentDetailPage("reviewer", emptySearch), { wrapper });

    expect(mockUseAgent).toHaveBeenCalledWith("reviewer", "ws_beta");
    expect(mockUseAgentSessions).toHaveBeenCalledWith("ws_beta", "reviewer");
    act(() => result.current.setTab("sessions"));
    expect(mockNavigate).toHaveBeenLastCalledWith(
      expect.objectContaining({
        search: expect.objectContaining({ tab: "sessions", workspace: "ws_beta" }),
      })
    );
  });

  it("exposes exact catalog session metrics and session-row pagination separately", () => {
    const loadMore = vi.fn();
    mockUseAgentSessions.mockReturnValue({
      sessions: [{ id: "sess-page-1" }],
      hasMore: true,
      isLoadingMore: false,
      loadMore,
      isError: false,
      isLoading: false,
    });
    mockUseAgentCatalogMetrics.mockReturnValue({
      total: 205,
      active: 7,
      failed: null,
      runtimeSeconds: null,
      lastActivityAt: null,
      sessionsAvailable: false,
      isLoading: false,
      isError: false,
    });

    const { result } = renderHook(() => useAgentDetailPage("categorized-multi", emptySearch));

    expect(result.current.sessionsTotal).toBe(205);
    expect(result.current.activeSessionsTotal).toBe(7);
    expect(result.current.failedSessionsTotal).toBeNull();
    expect(result.current.runtimeSeconds).toBeNull();
    expect(result.current.metricsUnavailable).toBe(true);
    expect(result.current.metricsLoading).toBe(false);
    expect(result.current.metricsError).toBe(false);
    expect(result.current.sessionsLoading).toBe(false);
    expect(result.current.sessionsError).toBe(false);
    expect(result.current.lastSessionActivityAt).toBeNull();
    expect(result.current.hasMoreSessions).toBe(true);
    expect(result.current.onLoadMoreSessions).toBe(loadMore);
  });

  it("Should keep metrics and session-row query states independent under partial failures", () => {
    mockUseAgentSessions.mockReturnValue({
      sessions: [],
      hasMore: false,
      isLoadingMore: false,
      loadMore: vi.fn(),
      isError: true,
      isLoading: false,
    });
    mockUseAgentCatalogMetrics.mockReturnValue({
      total: 11,
      active: 2,
      failed: 1,
      runtimeSeconds: 90,
      lastActivityAt: "2026-07-11T12:00:00Z",
      sessionsAvailable: true,
      isLoading: false,
      isError: false,
    });

    const sessionsFailed = renderHook(() => useAgentDetailPage("categorized-multi", emptySearch));
    expect(sessionsFailed.result.current.sessionsError).toBe(true);
    expect(sessionsFailed.result.current.metricsUnavailable).toBe(false);
    expect(sessionsFailed.result.current.metricsError).toBe(false);
    expect(sessionsFailed.result.current.activeSessionsTotal).toBe(2);

    mockUseAgentSessions.mockReturnValue({
      sessions: [{ id: "sess-ok" }],
      hasMore: false,
      isLoadingMore: false,
      loadMore: vi.fn(),
      isError: false,
      isLoading: false,
    });
    mockUseAgentCatalogMetrics.mockReturnValue({
      total: 0,
      active: 0,
      failed: null,
      runtimeSeconds: null,
      lastActivityAt: null,
      sessionsAvailable: false,
      isLoading: false,
      isError: true,
    });

    const metricsFailed = renderHook(() => useAgentDetailPage("categorized-multi", emptySearch));
    expect(metricsFailed.result.current.sessionsError).toBe(false);
    expect(metricsFailed.result.current.sessionsLoading).toBe(false);
    expect(metricsFailed.result.current.metricsError).toBe(true);
    expect(metricsFailed.result.current.metricsUnavailable).toBe(true);
    expect(metricsFailed.result.current.sessions).toEqual([{ id: "sess-ok" }]);
  });
});

describe("agentStatusSlot", () => {
  const page = (overrides: Partial<UseAgentDetailResult>) =>
    ({
      agent: { name: "fraud-ops" },
      metricsLoading: false,
      metricsUnavailable: false,
      metricsError: false,
      ...overrides,
    }) as UseAgentDetailResult;

  it.each([
    { name: "metrics loaded", overrides: {}, expected: "pill" },
    { name: "metrics loading", overrides: { metricsLoading: true }, expected: "placeholder" },
    {
      name: "agent scope still resolving (metrics disabled)",
      overrides: { agent: undefined, metricsUnavailable: true },
      expected: "placeholder",
    },
    {
      name: "metrics failed",
      overrides: { metricsError: true, metricsUnavailable: true },
      expected: "none",
    },
    {
      name: "loaded agent without session metrics",
      overrides: { metricsUnavailable: true },
      expected: "none",
    },
  ] as const)("Should resolve the status slot when $name", ({ overrides, expected }) => {
    expect(agentStatusSlot(page(overrides as Partial<UseAgentDetailResult>))).toBe(expected);
  });
});

describe("agentDetailTabItems", () => {
  const page = (overrides: Partial<UseAgentDetailResult>) =>
    ({
      agent: { name: "fraud-ops" },
      metricsLoading: false,
      metricsUnavailable: false,
      metricsError: false,
      sessionsTotal: 12,
      activeSessionsTotal: 0,
      ...overrides,
    }) as UseAgentDetailResult;
  const sessionsCount = (overrides: Partial<UseAgentDetailResult>) =>
    agentDetailTabItems(page(overrides)).find(item => item.value === "sessions")?.count;

  // Invariant: the Sessions tab holds its count's slot for as long as the status slot holds its
  // placeholder, shows the exact total once the metrics are in, and never prints an inferred zero
  // when they never will be.
  it.each([
    { name: "metrics loaded", overrides: {}, expected: 12 },
    { name: "metrics loading", overrides: { metricsLoading: true }, expected: "pending" },
    {
      name: "agent scope still resolving (metrics disabled)",
      overrides: { agent: undefined, metricsUnavailable: true },
      expected: "pending",
    },
    {
      name: "metrics failed",
      overrides: { metricsError: true, metricsUnavailable: true },
      expected: undefined,
    },
    {
      name: "loaded agent without session metrics",
      overrides: { metricsUnavailable: true },
      expected: undefined,
    },
  ] as const)("Should resolve the Sessions tab count when $name", ({ overrides, expected }) => {
    expect(sessionsCount(overrides as Partial<UseAgentDetailResult>)).toBe(expected);
  });
});

import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockNavigate, mockUseAgent, mockUseAgentSessions, mockUseAgentCatalogMetrics } = vi.hoisted(
  () => ({
    mockNavigate: vi.fn<(input: unknown) => Promise<void>>(),
    mockUseAgent: vi.fn(),
    mockUseAgentSessions: vi.fn(),
    mockUseAgentCatalogMetrics: vi.fn(),
  })
);

let mockActiveWorkspaceId: string | null = "ws_alpha";

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

vi.mock("@/systems/session", () => ({
  useSessionCreate: () => ({
    hasActiveWorkspace: true,
    isCreating: false,
    openForAgent: vi.fn(),
    pendingAgentName: null,
  }),
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: mockActiveWorkspaceId,
  }),
}));

import { useAgentDetailPage } from "@/systems/os/apps/agents/use-agent-detail";

const emptySearch = {};

describe("useAgentDetailPage", () => {
  beforeEach(() => {
    mockActiveWorkspaceId = "ws_alpha";
    mockNavigate.mockReset();
    mockUseAgent.mockReset();
    mockUseAgentSessions.mockReset();
    mockUseAgentCatalogMetrics.mockReset();
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
  });

  it("Should load agent details in the active workspace context", () => {
    const { result } = renderHook(() => useAgentDetailPage("categorized-multi", emptySearch));

    expect(mockUseAgent).toHaveBeenCalledWith("categorized-multi", "ws_alpha");
    expect(mockUseAgentSessions).toHaveBeenCalledWith("ws_alpha", "categorized-multi");
    expect(mockUseAgentCatalogMetrics).toHaveBeenCalledWith("ws_alpha", "categorized-multi");
    expect(result.current.search).toEqual({
      tab: "overview",
      file: "agent",
      filter: "all",
    });
  });

  it("Should fall back to global agent details when no workspace is active", () => {
    mockActiveWorkspaceId = null;

    renderHook(() => useAgentDetailPage("global-agent", emptySearch));

    expect(mockUseAgent).toHaveBeenCalledWith("global-agent", null);
    expect(mockUseAgentSessions).toHaveBeenCalledWith(null, "global-agent");
    expect(mockUseAgentCatalogMetrics).toHaveBeenCalledWith(null, "global-agent");
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

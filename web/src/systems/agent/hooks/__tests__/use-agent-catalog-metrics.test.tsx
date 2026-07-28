import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/agent-api", async importOriginal => ({
  ...(await importOriginal()),
  fetchAgentCatalog: vi.fn(),
}));

import { fetchAgentCatalog } from "../../adapters/agent-api";
import { useAgentCatalogMetrics } from "../use-agent-catalog-metrics";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("useAgentCatalogMetrics", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("Should wait for workspace before querying the catalog by exact name", async () => {
    vi.mocked(fetchAgentCatalog).mockResolvedValue({
      agents: [],
      facets: { active: 0, categories: [], idle: 0, total: 0 },
      page: { has_more: false, limit: 1, total: 0 },
      sessions_available: true,
    });

    const initialProps: { workspaceId: string | null } = { workspaceId: null };
    const { rerender } = renderHook(
      ({ workspaceId }: { workspaceId: string | null }) =>
        useAgentCatalogMetrics(workspaceId, "fraud-ops-agent"),
      { initialProps, wrapper: createWrapper() }
    );

    expect(fetchAgentCatalog).not.toHaveBeenCalled();
    rerender({ workspaceId: "ws_alpha" });

    await waitFor(() => {
      expect(fetchAgentCatalog).toHaveBeenCalled();
    });
    const [request] = vi.mocked(fetchAgentCatalog).mock.calls[0]!;
    expect(request).toMatchObject({
      workspace: "ws_alpha",
      name: "fraud-ops-agent",
      limit: 1,
    });
    expect(request.q).toBeUndefined();
  });

  it("Should expose exact catalog session aggregates without page derivation", async () => {
    vi.mocked(fetchAgentCatalog).mockResolvedValue({
      agents: [
        {
          agent: {
            name: "fraud-ops-agent",
            provider: "claude",
            prompt: "p",
            origin: "global",
            definition_digest: "a".repeat(64),
          },
          sessions: {
            active: 2,
            failed: 3,
            runtime_seconds: 90,
            total: 11,
            last_activity_at: "2026-04-17T18:10:00Z",
          },
        },
      ],
      facets: { active: 1, categories: [], idle: 0, total: 1 },
      page: { has_more: false, limit: 1, total: 1 },
      sessions_available: true,
    });

    const { result } = renderHook(() => useAgentCatalogMetrics("ws_alpha", "fraud-ops-agent"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.sessionsAvailable).toBe(true);
    });
    expect(result.current).toMatchObject({
      total: 11,
      active: 2,
      failed: 3,
      runtimeSeconds: 90,
      lastActivityAt: "2026-04-17T18:10:00Z",
    });
  });

  it("Should report metrics unavailable when the exact agent name is absent from the page", async () => {
    vi.mocked(fetchAgentCatalog).mockResolvedValue({
      agents: [
        {
          agent: {
            name: "other-agent",
            provider: "claude",
            prompt: "p",
            origin: "global",
            definition_digest: "a".repeat(64),
          },
          sessions: {
            active: 9,
            failed: 4,
            runtime_seconds: 120,
            total: 40,
            last_activity_at: "2026-04-17T18:10:00Z",
          },
        },
      ],
      facets: { active: 1, categories: [], idle: 0, total: 1 },
      page: { has_more: false, limit: 1, total: 1 },
      sessions_available: true,
    });

    const { result } = renderHook(() => useAgentCatalogMetrics("ws_alpha", "fraud-ops-agent"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });
    expect(result.current.sessionsAvailable).toBe(false);
    expect(result.current).toMatchObject({
      total: 0,
      active: 0,
      failed: null,
      runtimeSeconds: null,
      lastActivityAt: null,
    });
  });
});

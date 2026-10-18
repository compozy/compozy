import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/systems/session/adapters/session-api", async importOriginal => ({
  ...(await importOriginal()),
  fetchSessions: vi.fn(),
}));

import { fetchSessions } from "@/systems/session/adapters/session-api";
import { useAgentSessions } from "../use-agent-sessions";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("useAgentSessions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(fetchSessions).mockImplementation(filters =>
      Promise.resolve({
        sessions: [],
        page: {
          has_more: filters?.archive === "only",
          limit: 50,
          total: filters?.archive === "only" ? 4 : 0,
          ...(filters?.archive === "only" ? { next_cursor: "archive-page-2" } : {}),
        },
      })
    );
  });

  it.each(["ws_alpha", null])(
    "Should query the agent's complete visible session population after scope %s resolves",
    async workspaceId => {
      const initialProps: { workspaceId: string | null; enabled: boolean } = {
        workspaceId: null,
        enabled: false,
      };
      const { result, rerender } = renderHook(
        ({ workspaceId, enabled }) => useAgentSessions(workspaceId, "claude-agent", { enabled }),
        {
          initialProps,
          wrapper: createWrapper(),
        }
      );

      expect(fetchSessions).not.toHaveBeenCalled();

      rerender({ workspaceId, enabled: true });

      await waitFor(() => {
        expect(fetchSessions).toHaveBeenCalledTimes(2);
      });
      await waitFor(() => {
        expect(result.current.archivedTotal).toBe(4);
      });
      for (const [filters] of vi.mocked(fetchSessions).mock.calls) {
        expect(filters?.workspace_id).toBe(workspaceId ?? undefined);
        expect(filters?.all_workspaces).toBe(workspaceId ? undefined : true);
        expect(filters?.agent).toBe("claude-agent");
        expect(filters?.sort).toBe("last_activity");
        expect(filters?.type).toBeUndefined();
      }
      expect(vi.mocked(fetchSessions).mock.calls.map(([filters]) => filters?.archive)).toEqual([
        undefined,
        "only",
      ]);
      expect(result.current.hasMoreArchived).toBe(true);
    }
  );

  it("Should surface an archived-catalog failure instead of presenting an empty archive", async () => {
    vi.mocked(fetchSessions).mockImplementation(filters => {
      if (filters?.archive === "only") {
        return Promise.reject(new Error("Archived sessions are unavailable"));
      }
      return Promise.resolve({
        sessions: [],
        page: { has_more: false, limit: 50, total: 0 },
      });
    });
    const { result } = renderHook(() => useAgentSessions("ws_alpha", "claude-agent"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
    expect(result.current.archivedSessions).toEqual([]);
  });
});

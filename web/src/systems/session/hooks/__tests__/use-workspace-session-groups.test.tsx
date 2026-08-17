// Suite: complete per-workspace session groups
// Invariant: Show all loads every cursor page inside each workspace group.
// Owning layer: grouped session query hook. No prior suite owned this projection.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/session-api", () => ({ fetchSessions: vi.fn() }));

import { fetchSessions } from "../../adapters/session-api";
import type { SessionPayload } from "../../types";
import { useWorkspaceSessionGroups } from "../use-workspace-session-groups";

function session(id: string): SessionPayload {
  return {
    id,
    agent_name: "claude",
    runtime: { status: "ready", transition: "initial_bind", selection_revision: 0 },
    workspace_id: "ws-alpha",
    workspace_path: "/workspace/alpha",
    state: "active",
    badge: "idle",
    attachable: true,
    available_commands: [],
    created_at: "2026-08-16T10:00:00Z",
    updated_at: "2026-08-16T10:00:00Z",
    archived_at: null,
    pending_interactions: [],
  };
}

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return createElement(QueryClientProvider, { client }, children);
}

describe("useWorkspaceSessionGroups", () => {
  beforeEach(() => vi.clearAllMocks());

  it("Should load the complete cursor chain for a workspace", async () => {
    vi.mocked(fetchSessions)
      .mockResolvedValueOnce({
        sessions: [session("sess-2")],
        page: { has_more: true, limit: 1, next_cursor: "cursor-1", total: 2 },
      })
      .mockResolvedValueOnce({
        sessions: [session("sess-1")],
        page: { has_more: false, limit: 1, total: 2 },
      });

    const { result } = renderHook(
      () =>
        useWorkspaceSessionGroups({
          workspaces: [{ id: "ws-alpha", name: "Alpha" }],
          sort: "attention",
          archived: false,
          enabled: true,
        }),
      { wrapper }
    );

    await waitFor(() => expect(result.current[0]?.sessions).toHaveLength(2));
    expect(result.current[0]?.total).toBe(2);
    expect(fetchSessions).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ cursor: "cursor-1", workspace: "ws-alpha" }),
      expect.any(AbortSignal)
    );
    expect(vi.mocked(fetchSessions).mock.calls[0]?.[0]).not.toHaveProperty("archive");
  });

  it("Should read the archive per workspace when the archived breadth is on", async () => {
    vi.mocked(fetchSessions).mockResolvedValue({
      sessions: [session("sess-archived")],
      page: { has_more: false, limit: 100, total: 1 },
    });

    const { result } = renderHook(
      () =>
        useWorkspaceSessionGroups({
          workspaces: [{ id: "ws-alpha", name: "Alpha" }],
          sort: "attention",
          archived: true,
          enabled: true,
        }),
      { wrapper }
    );

    await waitFor(() => expect(result.current[0]?.sessions).toHaveLength(1));
    expect(fetchSessions).toHaveBeenCalledWith(
      expect.objectContaining({ archive: "only", workspace: "ws-alpha" }),
      expect.any(AbortSignal)
    );
  });
});

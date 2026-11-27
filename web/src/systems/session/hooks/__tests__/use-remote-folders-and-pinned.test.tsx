// Suite: Remote folder error re-read / read-only cache (T-F11) & multi-computer Pinned (T-F12)
// Invariant: Remote folder creation re-reads layout on conflict/limit error; offline/read-only
//            folders render from cache with no network requests or verbs; multi-computer Pinned
//            merges pins across answered computers, labels remote pins, and omits reorder (O6).
// Owning layer: hooks (systems/session/hooks)
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/session-folders-api", () => ({
  fetchSessionFolders: vi.fn(),
  createSessionFolder: vi.fn(),
  renameSessionFolder: vi.fn(),
  deleteSessionFolder: vi.fn(),
  moveSessionToFolder: vi.fn(),
  reorderSessionFolders: vi.fn(),
  reorderSessions: vi.fn(),
}));
vi.mock("../../adapters/session-catalog-api", () => ({
  fetchSessionCatalogPage: vi.fn(),
  fetchSessionFacets: vi.fn(),
}));

import { LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import {
  LOCAL_COMPUTER_FIXTURE,
  makeComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";

import { fetchSessionCatalogPage } from "../../adapters/session-catalog-api";
import { createSessionFolder, fetchSessionFolders } from "../../adapters/session-folders-api";
import { sessionKeys } from "../../lib/query-keys";
import type { ComputerSessionsSource } from "../use-computer-sessions-sources";
import { useMultiComputerPinned } from "../use-multi-computer-pinned";
import { useSessionFolders } from "../use-session-folders";

function wrap(client: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
}

describe("Remote folders (T-F11) and multi-computer Pinned (T-F12)", () => {
  beforeEach(() => vi.clearAllMocks());

  it("Should return cached folders without issuing network requests when readOnly is true (F10/F11)", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const folder = { id: "fld_1", name: "Backend", position: 0 };
    client.setQueryData(
      sessionKeys.forComputer("cmp_devbox").folders("ws_r1", "default"),
      [folder]
    );

    const { result } = renderHook(
      () =>
        useSessionFolders({
          workspaceId: "ws_r1",
          profile: "default",
          enabled: true,
          computerId: "cmp_devbox",
          readOnly: true,
        }),
      { wrapper: wrap(client) }
    );

    expect(result.current.layout).toBeUndefined();
    expect(result.current.cachedFolders).toEqual([folder]);
    expect(fetchSessionFolders).not.toHaveBeenCalled();
  });

  it("Should invalidate remote folders query when createFolder fails with conflict or limit (F12)", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidateSpy = vi.spyOn(client, "invalidateQueries");
    vi.mocked(fetchSessionFolders).mockResolvedValue([]);
    vi.mocked(createSessionFolder).mockRejectedValue(new Error("Folder limit reached (200)."));

    const { result } = renderHook(
      () =>
        useSessionFolders({
          workspaceId: "ws_r1",
          profile: "default",
          enabled: true,
          computerId: "cmp_devbox",
        }),
      { wrapper: wrap(client) }
    );

    await waitFor(() => expect(result.current.layout).toBeDefined());
    await expect(result.current.layout!.createFolder("Overflow")).rejects.toThrow(
      "Folder limit reached (200)."
    );
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: sessionKeys.forComputer("cmp_devbox").folders("ws_r1", "default"),
    });
  });

  it("Should combine pins across computers, label remote pins and disable reorder (T-F12 / Q8 / O6)", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(fetchSessionCatalogPage).mockImplementation(async (_query, _signal, computerId) => ({
      sessions: [
        computerId === "cmp_devbox"
          ? ({
              id: "pin_shared",
              name: "Remote pin",
              agent_name: "claude",
              workspace_id: "ws_r1",
              pinned_at: "2026-04-10T10:00:00.000Z",
              state: "active",
              badge: "idle",
              updated_at: "2026-04-10T10:00:00.000Z",
              created_at: "2026-04-10T10:00:00.000Z",
              pending_interactions: [],
              runtime: {},
            } as never)
          : ({
              id: "pin_shared",
              name: "Local pin",
              agent_name: "claude",
              workspace_id: "ws_l1",
              pinned_at: "2026-04-10T11:00:00.000Z",
              state: "active",
              badge: "idle",
              updated_at: "2026-04-10T11:00:00.000Z",
              created_at: "2026-04-10T11:00:00.000Z",
              pending_interactions: [],
              runtime: {},
            } as never),
      ],
      page: { has_more: false, next_cursor: "" },
    }));

    const localSource: ComputerSessionsSource = {
      key: LOCAL_COMPUTER_ID,
      isLocal: true,
      name: "MacBook Pro",
      computer: LOCAL_COMPUTER_FIXTURE,
      read: "ok",
      detail: "This computer",
      canOperate: true,
      canCreate: true,
      facets: undefined,
      count: 1,
      needsYou: 0,
      working: 0,
      laterCount: 0,
      workspaces: [{ id: "ws_l1", name: "compozy" } as never],
      workspacesLoading: false,
      workspacesError: false,
      matchedWorkspaceIds: null,
      retry: vi.fn(),
    };
    const devboxSource: ComputerSessionsSource = {
      ...localSource,
      key: "cmp_devbox",
      isLocal: false,
      name: "devbox",
      computer: makeComputerFixture({ id: "cmp_devbox", name: "devbox", status: "connected" }),
      workspaces: [{ id: "ws_r1", name: "compozy" } as never],
    };

    const { result } = renderHook(
      () =>
        useMultiComputerPinned({
          enabled: true,
          sources: [localSource, devboxSource],
          scope: { kind: "all" },
          localLens: { profile: "default" },
          groupFilters: { include_health: true, subagents: "exclude" },
          allProjects: true,
        }),
      { wrapper: wrap(client) }
    );

    await waitFor(() => expect(result.current?.sessions).toHaveLength(2));
    expect(result.current?.reorder).toBeUndefined();
    expect(result.current?.computerBySessionId?.get("cmp_devbox::pin_shared")?.name).toBe("devbox");
    expect(result.current?.computerBySessionId?.has("local::pin_shared")).toBe(false);
  });
});

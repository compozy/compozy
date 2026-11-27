// Suite: Remote folder error re-read / read-only cache (T-F11), multi-computer Pinned (T-F12),
//        structural-sharing computer ownership survival (Opus 1 / Astra 2), and same-ID cross-computer
//        batch execution/retry (Astra 6).
// Invariant: Remote folder creation re-reads layout on conflict/limit error; offline/read-only
//            folders render from cache with no network requests or verbs; multi-computer Pinned
//            merges pins across answered computers, labels remote pins, and omits reorder (O6);
//            TanStack structural sharing preserves `session.computerId` across refetches; batches
//            over identical session IDs on two computers execute and retry independently.
// Owning layer: hooks (systems/session/hooks)
import { QueryClient, QueryClientProvider, useInfiniteQuery } from "@tanstack/react-query";
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
  LOCAL_COMPUTER,
  makeComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";

import { fetchSessionCatalogPage } from "../../adapters/session-catalog-api";
import { createSessionFolder, fetchSessionFolders } from "../../adapters/session-folders-api";
import { sessionKeys } from "../../lib/query-keys";
import { runSessionBatch } from "../../lib/session-batch";
import { sessionCatalogOptions } from "../../lib/session-catalog-options";
import {
  parseSessionRowKey,
  sessionComputerOf,
  sessionRowKey,
  withSessionComputer,
} from "../../lib/session-row-ref";
import { primarySessionFixture } from "../../mocks/fixtures";
import type { ComputerSessionsSource } from "../use-computer-sessions-sources";
import { useMultiComputerPinned } from "../use-multi-computer-pinned";
import { useSessionFolders } from "../use-session-folders";

function wrap(client: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
}

describe("Remote folders (T-F11), multi-computer Pinned (T-F12), ownership and cross-computer batches", () => {
  beforeEach(() => vi.clearAllMocks());

  it("Should return cached folders without issuing network requests when readOnly is true (F10/F11)", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const folder = { id: "fld_1", name: "Backend", position: 0 };
    client.setQueryData(sessionKeys.forComputer("cmp_devbox").folders("ws_r1", "default"), [
      folder,
    ]);

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
      page: { has_more: false, next_cursor: "", limit: 50 },
    }));

    const localSource: ComputerSessionsSource = {
      key: LOCAL_COMPUTER_ID,
      isLocal: true,
      name: "MacBook Pro",
      computer: LOCAL_COMPUTER,
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
    expect(
      result.current?.computerBySessionId?.get(
        sessionRowKey({ computerId: "cmp_devbox", sessionId: "pin_shared" })
      )?.name
    ).toBe("devbox");
    expect(
      result.current?.computerBySessionId?.has(
        sessionRowKey({ computerId: LOCAL_COMPUTER_ID, sessionId: "pin_shared" })
      )
    ).toBe(false);
  });

  it("Should preserve remote computerId on fetched rows across TanStack Query structural-sharing refetches (Opus 1 / Astra 2)", async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: 0 } },
    });
    let call = 0;
    vi.mocked(fetchSessionCatalogPage).mockImplementation(async () => {
      call += 1;
      return {
        sessions: [
          {
            ...primarySessionFixture,
            id: "sess_remote_1",
            name: call === 1 ? "Before refetch" : "After refetch",
            updated_at: call === 1 ? "2026-04-10T10:00:00.000Z" : "2026-04-10T10:05:00.000Z",
          },
        ],
        page: { has_more: false, next_cursor: "", limit: 50 },
      };
    });

    const { result } = renderHook(
      () => useInfiniteQuery(sessionCatalogOptions({ all_workspaces: true }, "cmp_devbox")),
      { wrapper: wrap(client) }
    );

    await waitFor(() =>
      expect(result.current.data?.pages[0]?.sessions[0]?.name).toBe("Before refetch")
    );
    const firstRow = result.current.data!.pages[0]!.sessions[0]!;
    expect(firstRow.computerId).toBe("cmp_devbox");
    expect(sessionComputerOf(firstRow)).toBe("cmp_devbox");

    await result.current.refetch();
    await waitFor(() =>
      expect(result.current.data?.pages[0]?.sessions[0]?.name).toBe("After refetch")
    );
    const refetchedRow = result.current.data!.pages[0]!.sessions[0]!;
    expect(refetchedRow.computerId).toBe("cmp_devbox");
    expect(sessionComputerOf(refetchedRow)).toBe("cmp_devbox");
  });

  it("Should execute, track progress, and retry two same-ID sessions on different computers independently (Astra 6)", async () => {
    const localSession = { ...primarySessionFixture, id: "same_id", name: "Local copy" };
    const remoteSession = withSessionComputer(
      { ...primarySessionFixture, id: "same_id", name: "Remote copy" },
      "cmp_devbox"
    );
    const localKey = sessionRowKey({ computerId: LOCAL_COMPUTER_ID, sessionId: "same_id" });
    const remoteKey = sessionRowKey({ computerId: "cmp_devbox", sessionId: "same_id" });
    expect(localKey).not.toBe(remoteKey);

    const attempts: Array<{ computerId: string; sessionId: string }> = [];
    let failRemote = true;
    const execute = async (rowKey: string) => {
      const ref = parseSessionRowKey(rowKey);
      attempts.push(ref);
      if (ref.computerId === "cmp_devbox" && failRemote) {
        throw new Error("Remote daemon busy");
      }
    };

    const first = await runSessionBatch({
      action: "archive",
      ids: [localKey, remoteKey],
      previous: [],
      sessions: [localSession, remoteSession],
      execute,
      onProgress: () => undefined,
    });

    expect(first).toEqual([
      { id: localKey, status: "done" },
      {
        id: remoteKey,
        status: "failed",
        error: "Remote daemon busy",
        computerId: "cmp_devbox",
      },
    ]);
    expect(attempts).toEqual([
      { computerId: LOCAL_COMPUTER_ID, sessionId: "same_id" },
      { computerId: "cmp_devbox", sessionId: "same_id" },
    ]);

    failRemote = false;
    const retried = await runSessionBatch({
      action: "archive",
      ids: [remoteKey],
      previous: first,
      sessions: [localSession, remoteSession],
      execute,
      onProgress: () => undefined,
    });
    expect(retried).toEqual([
      { id: localKey, status: "done" },
      { id: remoteKey, status: "done", computerId: "cmp_devbox" },
    ]);
    expect(attempts).toHaveLength(3);
  });
});

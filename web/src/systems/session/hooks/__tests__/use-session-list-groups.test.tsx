// Suite: session-list grouping, its view model, and workspace working marks
// Invariant: groups, counts, and collapsed signals come from one complete facets read — never a
// loaded page — so an off-page group still shows; each group carries its own scope, lens, and
// (only for one workspace × profile pair) the pair that owns folders and manual order. The view
// model renders an inapplicable stored grouping as none and serves a stored `manual` sort as
// `last_activity` outside one pair, without rewriting either preference. A collapsed group stays
// collapsed when the list remounts (selecting a session remounts the window's content) and opens
// only on the operator's toggle. Workspace working marks
// read the same all-workspaces aggregate instead of one catalog read per workspace.
// Owning layer: session-list query hooks; session catalog I/O is the mocked boundary.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/session-api", () => ({ fetchSessions: vi.fn() }));
vi.mock("../../adapters/session-catalog-api", () => ({
  fetchSessionCatalogPage: vi.fn(),
  fetchSessionFacets: vi.fn(),
}));

import { WindowScopeContext } from "@/hooks/use-window-scope";
import {
  profilesListOptions,
  resetProfileViews,
  setProfileView,
  useProfileReadScope,
} from "@/systems/profiles";
import { defaultProfileFixture, marketingProfileFixture } from "@/systems/profiles/mocks";
import {
  USER_SCOPE_SELECTOR,
  enableAllProjects,
  setActiveWorkspaceId,
  type DataScope,
} from "@/systems/workspace";

import { fetchSessions } from "../../adapters/session-api";
import { fetchSessionCatalogPage, fetchSessionFacets } from "../../adapters/session-catalog-api";
import { sessionCatalogOptions, sessionFacetsOptions } from "../../lib/session-catalog-options";
import { useSessionListGroups, type SessionListGroupsInput } from "../use-session-list-groups";
import { SESSION_LIST_MANUAL_SORT_NOTE, useSessionListView } from "../use-session-list-view";
import { useWorkspaceWorkingSessions } from "../use-workspace-working-sessions";

function counts(all: number, working = 0, needsYou = 0) {
  return {
    terminal_approvals: 0,
    all,
    needs_you: needsYou,
    working,
    finished: 0,
    idle: all - working - needsYou,
    board: { needs_you: 0, working: 0, waiting: 0, in_review: 0, done: 0 },
  };
}

function facetsResponse(
  byWorkspace: Array<[string, ReturnType<typeof counts>]> = [],
  byProfile: Array<[string, ReturnType<typeof counts>]> = []
) {
  return {
    facets: counts(0),
    by_workspace: byWorkspace.map(([workspace_id, facets]) => ({ workspace_id, facets })),
    by_profile: byProfile.map(([profile_id, facets]) => ({ profile_id, facets })),
    later: 0,
  };
}

function clientWrapper(
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
}

let wrapper = clientWrapper();
beforeEach(() => {
  wrapper = clientWrapper();
});

const ALL: DataScope = { kind: "all" };
const ALPHA: DataScope = { kind: "project", workspaceId: "ws-alpha" };

function groupsInput(overrides: Partial<SessionListGroupsInput> = {}): SessionListGroupsInput {
  return {
    groupBy: "project",
    scope: ALL,
    lens: { profile: "default" },
    workspaces: [
      { id: "ws-beta", name: "Beta" },
      { id: "ws-alpha", name: "Alpha" },
    ],
    shelf: "active",
    enabled: true,
    ...overrides,
  };
}

describe("useSessionListGroups", () => {
  beforeEach(() => vi.clearAllMocks());

  it("W-09: Should list every project group from facets, in switcher order with User last", async () => {
    // ws-alpha's sessions sit on no loaded page; its group still exists because
    // membership comes from facets, not from whatever page the list has read.
    vi.mocked(fetchSessionFacets).mockResolvedValue(
      facetsResponse([
        ["", counts(2)],
        ["ws-alpha", counts(201, 1, 3)],
        ["ws-gone", counts(1)],
        ["ws-empty", counts(0)],
        ["ws-beta", counts(4, 2)],
      ])
    );
    const { result } = renderHook(() => useSessionListGroups(groupsInput()), { wrapper });

    await waitFor(() => expect(result.current.groups).toHaveLength(4));
    expect(result.current.groups.map(group => group.key)).toEqual([
      "project:ws-beta",
      "project:ws-alpha",
      "project:ws-gone",
      "user",
    ]);
    expect(result.current.groups[1]).toMatchObject({
      kind: "project",
      badge: { kind: "project", name: "Alpha" },
      total: 201,
      working: 1,
      needsYou: 3,
      scope: ALPHA,
      lens: { profile: "default" },
      pair: { workspaceId: "ws-alpha", profile: "default" },
    });
    expect(fetchSessionFacets).toHaveBeenCalledWith(
      expect.objectContaining({ all_workspaces: true, profile: "default", subagents: "exclude" }),
      expect.any(AbortSignal)
    );
    expect(fetchSessionCatalogPage).not.toHaveBeenCalled();
    expect(fetchSessions).not.toHaveBeenCalled();
  });

  it("Should count only a worktree's sessions when the list is restricted to it", async () => {
    vi.mocked(fetchSessionFacets).mockResolvedValue(facetsResponse([["ws-alpha", counts(1)]]));
    const { result } = renderHook(
      () => useSessionListGroups({ ...groupsInput(), worktreeId: "wt-1" }),
      { wrapper }
    );

    await waitFor(() => expect(result.current.groups).toHaveLength(1));
    expect(fetchSessionFacets).toHaveBeenCalledWith(
      expect.objectContaining({ all_workspaces: true, worktree: "wt-1" }),
      expect.any(AbortSignal)
    );
  });

  it("W-27: Should group sessions outside every project into one User group", async () => {
    vi.mocked(fetchSessionFacets).mockResolvedValue(facetsResponse([["", counts(3, 0, 1)]]));
    const { result } = renderHook(() => useSessionListGroups(groupsInput()), { wrapper });

    await waitFor(() => expect(result.current.groups).toHaveLength(1));
    expect(result.current.groups[0]).toMatchObject({
      key: "user",
      badge: { kind: "user" },
      total: 3,
      needsYou: 1,
      scope: { kind: "user" },
      pair: { workspaceId: USER_SCOPE_SELECTOR, profile: "default" },
    });
  });

  it.each([
    ["a focused project", ALPHA, { workspaceId: "ws-alpha", profile: "marketing" }],
    ["All projects", ALL, null],
  ] as const)(
    "W-10: Should list profile groups from by_profile in %s, pairing only one project",
    async (_case, scope, pair) => {
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      client.setQueryData(profilesListOptions().queryKey, [
        defaultProfileFixture,
        marketingProfileFixture,
      ]);
      vi.mocked(fetchSessionFacets).mockResolvedValue(
        facetsResponse(
          [],
          [
            [marketingProfileFixture.id, counts(2, 1)],
            [defaultProfileFixture.id, counts(0)],
          ]
        )
      );
      const { result } = renderHook(
        () =>
          useSessionListGroups(
            groupsInput({ groupBy: "profile", scope, lens: { all_profiles: true } })
          ),
        { wrapper: clientWrapper(client) }
      );

      await waitFor(() => expect(result.current.groups).toHaveLength(1));
      expect(result.current.groups[0]).toMatchObject({
        key: `profile:${marketingProfileFixture.id}`,
        kind: "profile",
        owner: { id: marketingProfileFixture.id, name: "marketing" },
        total: 2,
        working: 1,
        scope,
        lens: { profile: "marketing" },
        pair,
      });
      expect(fetchSessionFacets).toHaveBeenCalledWith(
        expect.objectContaining({ all_profiles: true }),
        expect.any(AbortSignal)
      );
    }
  );

  it("Should read nothing while the list renders flat", () => {
    const { result } = renderHook(() => useSessionListGroups(groupsInput({ groupBy: "none" })), {
      wrapper,
    });
    expect(result.current).toEqual({ groups: [], loading: false, error: undefined });
    expect(fetchSessionFacets).not.toHaveBeenCalled();
  });

  it("Should scope archived group counts before requesting pages", async () => {
    vi.mocked(fetchSessionFacets).mockResolvedValue(facetsResponse());
    const { result } = renderHook(() => useSessionListGroups(groupsInput({ shelf: "archive" })), {
      wrapper,
    });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(fetchSessionFacets).toHaveBeenCalledWith(
      expect.objectContaining({ archive: "only", profile: "default", all_workspaces: true }),
      expect.any(AbortSignal)
    );
  });

  // UT-W18: counts use the same subagent visibility as the rows they head.
  it("Should exclude subagent sessions from group counts until a search widens them", async () => {
    vi.mocked(fetchSessionFacets).mockResolvedValue(facetsResponse());
    const { rerender } = renderHook(
      ({ search }: { search?: string }) => useSessionListGroups(groupsInput({ search })),
      { wrapper, initialProps: {} }
    );
    rerender({ search: "  " });
    rerender({ search: "review" });
    await waitFor(() =>
      expect(fetchSessionFacets).toHaveBeenCalledWith(
        expect.objectContaining({ subagents: "include" }),
        expect.any(AbortSignal)
      )
    );
    expect(fetchSessionFacets).toHaveBeenCalledWith(
      expect.objectContaining({ subagents: "exclude", all_workspaces: true }),
      expect.any(AbortSignal)
    );
  });

  it("Should report a failed facets read with one retry that reads it again", async () => {
    vi.mocked(fetchSessionFacets).mockRejectedValueOnce(new Error("facets down"));
    const { result } = renderHook(() => useSessionListGroups(groupsInput()), { wrapper });

    await waitFor(() => expect(result.current.error).toBeDefined());
    expect(result.current.groups).toEqual([]);
    vi.mocked(fetchSessionFacets).mockResolvedValue(facetsResponse([["ws-alpha", counts(1)]]));
    result.current.error!.retry();
    await waitFor(() => expect(result.current.groups).toHaveLength(1));
    expect(result.current.error).toBeUndefined();
  });

  // UT-W18: visibility survives normalization into the wire request and the cache key, for both
  // the page and its facet counts, so `exclude` and `include` never share a cache entry.
  it("Should carry subagent visibility into page and facet requests and keys", async () => {
    const page = (subagents: "include" | "exclude") =>
      sessionCatalogOptions({ workspace_id: "ws-alpha", subagents });
    const facets = (subagents: "include" | "exclude") =>
      sessionFacetsOptions({ workspace_id: "ws-alpha", subagents });
    expect(page("exclude").queryKey[4]).toMatchObject({ subagents: "exclude" });
    expect(page("exclude").queryKey).not.toEqual(page("include").queryKey);
    expect(facets("exclude").queryKey).not.toEqual(facets("include").queryKey);

    vi.mocked(fetchSessionCatalogPage).mockResolvedValue({
      sessions: [],
      page: { has_more: false, limit: 100, next_cursor: null },
    } as never);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    await client.fetchInfiniteQuery(page("exclude"));
    expect(fetchSessionCatalogPage).toHaveBeenCalledWith(
      expect.objectContaining({ subagents: "exclude" }),
      expect.any(AbortSignal)
    );
  });
});

describe("useSessionListView", () => {
  const shell = { sort: "created", group_by: "none" };
  let fetcher: ReturnType<typeof vi.spyOn<typeof globalThis, "fetch">>;
  let registered: Array<[string, string]> = [];
  let layout: { folders: unknown[]; placements: unknown[] } = { folders: [], placements: [] };
  let workspaceOrders: unknown[] = [];

  beforeEach(() => {
    registered = [["ws-alpha", "Alpha"]];
    layout = { folders: [], placements: [] };
    workspaceOrders = [];
    vi.clearAllMocks();
    vi.mocked(fetchSessionCatalogPage).mockResolvedValue({
      sessions: [],
      page: { has_more: false, limit: 100 },
    } as never);
    vi.mocked(fetchSessionFacets).mockResolvedValue(facetsResponse([["ws-alpha", counts(1)]]));
    // The daemon's settings and workspace reads are the I/O boundary here.
    fetcher = vi.spyOn(globalThis, "fetch").mockImplementation(async input => {
      const request = input instanceof Request ? input : new Request(String(input));
      const path = new URL(request.url).pathname;
      const json = (body: unknown) =>
        new Response(JSON.stringify(body), { headers: { "content-type": "application/json" } });
      if (path === "/api/settings/shell" && request.method === "GET") {
        return json({ config: { sessions: { ...shell } } });
      }
      if (path === "/api/workspaces") {
        return json({
          workspaces: registered.map(([id, name]) => ({
            id,
            name,
            root_dir: `/workspace/${id}`,
            add_dirs: [],
            created_at: "2026-10-01T00:00:00Z",
            updated_at: "2026-10-01T00:00:00Z",
          })),
        });
      }
      if (path === "/api/workspace-folders") return json(layout);
      if (path === "/api/workspace-order/workspaces" && request.method === "PUT") {
        const body = (await request.json()) as { workspace_ids: string[] };
        workspaceOrders.push(body);
        // The daemon pins the listed order at the head of the root.
        layout = {
          ...layout,
          placements: body.workspace_ids.map((workspace_id, position) => ({
            workspace_id,
            position,
          })),
        };
        return json(layout);
      }
      return new Response(JSON.stringify({ error: "not found" }), { status: 404 });
    });
    setActiveWorkspaceId("ws-alpha");
  });

  afterEach(() => {
    fetcher.mockRestore();
    shell.sort = "created";
    shell.group_by = "none";
  });

  function settingsWrites() {
    return fetcher.mock.calls.filter(([input]) => (input as Request).method !== "GET");
  }

  // Invariant: the Later toggle's count rides the active facets read the footer totals make,
  // so the list costs one facets request per catalog wake, never a second `later=only` read.
  it("Should count Later from the active facets read without a Later facets request", async () => {
    vi.mocked(fetchSessionFacets).mockResolvedValue({
      ...facetsResponse([["ws-alpha", counts(1)]]),
      later: 4,
    });
    const { result } = renderHook(() => useSessionListView(), { wrapper });

    await waitFor(() => expect(result.current.laterCount).toBe(4));
    const facetReads = vi.mocked(fetchSessionFacets).mock.calls.map(([query]) => query);
    expect(facetReads.length).toBeGreaterThan(0);
    expect(facetReads.every(query => query?.later === undefined)).toBe(true);
  });

  it("W-11: Should render a stored project grouping flat inside one project without rewriting it", async () => {
    shell.group_by = "project";
    const { result } = renderHook(() => useSessionListView(), { wrapper });

    await waitFor(() => expect(result.current.storedGroupBy).toBe("project"));
    await waitFor(() => expect(fetchSessionCatalogPage).toHaveBeenCalled());
    expect(result.current.groupBy).toBe("none");
    expect(result.current.groupByOptions).toEqual(["none"]);
    expect(result.current.catalog).toBeDefined();
    // Flat, the list reads no group membership; its one facets read is the totals footer
    // over the focused project alone (#1044).
    expect(result.current.groups).toEqual([]);
    await waitFor(() => expect(vi.mocked(fetchSessionFacets).mock.calls.length).toBeGreaterThan(0));
    for (const [filters] of vi.mocked(fetchSessionFacets).mock.calls) {
      expect(filters).toMatchObject({ workspace_id: "ws-alpha" });
      expect(filters).not.toHaveProperty("all_workspaces");
    }

    // The same stored value applies as soon as the breadth allows it.
    act(() => enableAllProjects());
    await waitFor(() => expect(result.current.groupBy).toBe("project"));
    expect(result.current.groupByOptions).toEqual(["none", "project"]);
    await waitFor(() => expect(result.current.groups).toHaveLength(1));
    expect(result.current.catalog).toBeUndefined();
    expect(settingsWrites()).toEqual([]);
  });

  it("W-12: Should serve a stored manual sort as last activity outside one pair, with a note", async () => {
    shell.sort = "manual";
    act(() => enableAllProjects());
    const { result } = renderHook(() => useSessionListView(), { wrapper });

    await waitFor(() => expect(result.current.sortNote).toBe(SESSION_LIST_MANUAL_SORT_NOTE));
    expect(result.current.sort).toBe("manual");
    await waitFor(() =>
      expect(fetchSessionCatalogPage).toHaveBeenCalledWith(
        expect.objectContaining({ all_workspaces: true, sort: "last_activity" }),
        expect.any(AbortSignal)
      )
    );
    expect(result.current.layout).toBeUndefined();
    expect(result.current.newFolderUnavailable).toBe(
      "Folders live inside one project and profile. Group by project to add one."
    );

    // One project under one profile is a pair: manual order applies and the note leaves.
    act(() => setActiveWorkspaceId("ws-alpha"));
    await waitFor(() => expect(result.current.sortNote).toBeNull());
    await waitFor(() =>
      expect(fetchSessionCatalogPage).toHaveBeenCalledWith(
        expect.objectContaining({ workspace_id: "ws-alpha", sort: "manual" }),
        expect.any(AbortSignal)
      )
    );
    expect(settingsWrites()).toEqual([]);
  });
  // #1085: with empty projects hidden, a reorder of the projects on screen writes the hidden
  // ones back into their slots instead of letting the daemon move them after the listed ones.
  it("Should keep a hidden empty project in its slot when the shown projects reorder", async () => {
    shell.group_by = "project";
    registered = [
      ["ws-alpha", "Alpha"],
      ["ws-beta", "Beta"],
      ["ws-gamma", "Gamma"],
    ];
    layout = {
      folders: [],
      placements: [
        { workspace_id: "ws-gamma", position: 0 },
        { workspace_id: "ws-alpha", position: 1 },
        { workspace_id: "ws-beta", position: 2 },
      ],
    };
    vi.mocked(fetchSessionFacets).mockResolvedValue(
      facetsResponse([
        ["ws-alpha", counts(1)],
        ["ws-beta", counts(2)],
      ])
    );
    act(() => enableAllProjects());
    const { result } = renderHook(() => useSessionListView(), { wrapper });

    await waitFor(() => expect(result.current.projectFolders).toBeDefined());
    await waitFor(() =>
      expect(result.current.groups.map(group => group.key)).toEqual([
        "project:ws-alpha",
        "project:ws-beta",
      ])
    );
    act(() =>
      result.current.projectFolders!.layout.reorderWorkspaces(null, ["ws-beta", "ws-alpha"])
    );
    await waitFor(() => expect(workspaceOrders).toHaveLength(1));
    expect(workspaceOrders[0]).toEqual({ workspace_ids: ["ws-gamma", "ws-beta", "ws-alpha"] });
    // Once the layout rereads, the list shows the new order with the hidden project still out.
    await waitFor(() =>
      expect(result.current.groups.map(group => group.key)).toEqual([
        "project:ws-beta",
        "project:ws-alpha",
      ])
    );
  });

  // Issue #903: selecting a session retargets the window, which remounts its content and the
  // list view with it (`SessionPromptRuntimeProvider key={sessionId}`); the collapse is the
  // window's rail state, so the remounted list keeps it while another window keeps its own.
  it("W-13: Should keep a window's collapsed group across a selection remount", async () => {
    shell.group_by = "project";
    act(() => enableAllProjects());
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inWindow = (scopeId: string) => {
      const withClient = clientWrapper(client);
      return ({ children }: { children: ReactNode }) =>
        createElement(
          WindowScopeContext,
          { value: scopeId },
          createElement(withClient, null, children)
        );
    };
    const other = renderHook(() => useSessionListView(), { wrapper: inWindow("w-903-other") });
    const first = renderHook(() => useSessionListView(), { wrapper: inWindow("w-903-main") });
    await waitFor(() => expect(first.result.current.groups).toHaveLength(1));
    const key = first.result.current.groups[0]!.key;

    act(() => first.result.current.toggleGroup(key));
    expect(first.result.current.collapsedGroupKeys.has(key)).toBe(true);
    first.unmount();

    const remounted = renderHook(() => useSessionListView(), { wrapper: inWindow("w-903-main") });
    await waitFor(() => expect(remounted.result.current.groups).toHaveLength(1));
    expect(remounted.result.current.collapsedGroupKeys.has(key)).toBe(true);
    // The window that was already open keeps its own rail (#843).
    await waitFor(() => expect(other.result.current.groups).toHaveLength(1));
    expect(other.result.current.collapsedGroupKeys.has(key)).toBe(false);

    act(() => remounted.result.current.toggleGroup(key));
    expect(remounted.result.current.collapsedGroupKeys.has(key)).toBe(false);
    expect(settingsWrites()).toEqual([]);
  });

  it("W-15: Should preserve each window's folds when a provisional profile resolves", async () => {
    shell.group_by = "project";
    const profile = { ...marketingProfileFixture, id: "profile-resolving", name: "resolving" };
    act(() => {
      enableAllProjects();
      setProfileView({ kind: "profile", profile: profile.name });
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(profilesListOptions().queryKey, []);
    const inWindow = (scopeId: string) => {
      const withClient = clientWrapper(client);
      return ({ children }: { children: ReactNode }) =>
        createElement(
          WindowScopeContext,
          { value: scopeId },
          createElement(withClient, null, children)
        );
    };
    const other = renderHook(() => useSessionListView(), { wrapper: inWindow("w-resolve-other") });
    const first = renderHook(() => useSessionListView(), { wrapper: inWindow("w-resolve-main") });
    await waitFor(() => expect(first.result.current.groups).toHaveLength(1));
    const key = first.result.current.groups[0]!.key;
    act(() => first.result.current.toggleGroup(key));

    const identity = renderHook(() => useProfileReadScope(), {
      wrapper: inWindow("w-resolve-main"),
    });
    act(() => client.setQueryData(profilesListOptions().queryKey, [profile]));
    await waitFor(() => expect(identity.result.current.destinationOwner?.id).toBe(profile.id));
    await waitFor(() => expect(first.result.current.collapsedGroupKeys.has(key)).toBe(true));
    expect(other.result.current.collapsedGroupKeys.has(key)).toBe(false);
    const inherited = renderHook(() => useSessionListView(), {
      wrapper: inWindow("w-resolve-new"),
    });
    await waitFor(() => expect(inherited.result.current.collapsedGroupKeys.has(key)).toBe(true));

    act(() => {
      client.setQueryData(profilesListOptions().queryKey, [
        { ...profile, name: "resolved-renamed" },
      ]);
      setProfileView({ kind: "profile", profile: "resolved-renamed" });
    });
    await waitFor(() =>
      expect(identity.result.current.destinationOwner?.name).toBe("resolved-renamed")
    );
    await waitFor(() => expect(first.result.current.collapsedGroupKeys.has(key)).toBe(true));
    expect(other.result.current.collapsedGroupKeys.has(key)).toBe(false);
    act(() => resetProfileViews());
  });

  // A persisted fold names its profile by id, so renaming the profile keeps it.
  it("W-14: Should keep a collapsed group when its profile is renamed", async () => {
    shell.group_by = "project";
    act(() => enableAllProjects());
    act(() => setProfileView({ kind: "profile", profile: marketingProfileFixture.name }));
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(profilesListOptions().queryKey, [marketingProfileFixture]);
    const { result } = renderHook(() => useSessionListView(), {
      wrapper: clientWrapper(client),
    });
    await waitFor(() => expect(result.current.groups).toHaveLength(1));
    const key = result.current.groups[0]!.key;
    act(() => result.current.toggleGroup(key));
    expect(result.current.collapsedGroupKeys.has(key)).toBe(true);

    act(() => {
      client.setQueryData(profilesListOptions().queryKey, [
        { ...marketingProfileFixture, name: "growth" },
      ]);
      setProfileView({ kind: "profile", profile: "growth" });
    });
    await waitFor(() => expect(result.current.groups).toHaveLength(1));
    expect(result.current.collapsedGroupKeys.has(key)).toBe(true);

    act(() => result.current.toggleGroup(key));
    resetProfileViews();
  });
});

describe("useWorkspaceWorkingSessions", () => {
  beforeEach(() => vi.clearAllMocks());

  const facets = (working: number) => ({
    terminal_approvals: 0,
    all: working + 1,
    needs_you: 0,
    working,
    finished: 0,
    idle: 1,
    board: { needs_you: 0, working: 0, waiting: 0, in_review: 0, done: 0 },
  });

  it("Should key working counts by workspace from one cross-workspace facets read", async () => {
    vi.mocked(fetchSessionFacets).mockResolvedValue({
      facets: facets(3),
      by_profile: [],
      later: 0,
      by_workspace: [
        { workspace_id: "ws-alpha", facets: facets(2) },
        { workspace_id: "ws-beta", facets: facets(0) },
        { workspace_id: "ws-gamma", facets: facets(1) },
      ],
    });
    const { result } = renderHook(() => useWorkspaceWorkingSessions(true), { wrapper });

    await waitFor(() => expect(result.current).toEqual({ "ws-alpha": 2, "ws-gamma": 1 }));
    expect(fetchSessionFacets).toHaveBeenCalledTimes(1);
    expect(fetchSessionFacets).toHaveBeenCalledWith(
      expect.objectContaining({
        all_workspaces: true,
        subagents: "exclude",
        profile: "default",
      }),
      expect.any(AbortSignal)
    );
    expect(fetchSessions).not.toHaveBeenCalled();
  });

  it("Should not read the catalog while no switcher is open", () => {
    const { result } = renderHook(() => useWorkspaceWorkingSessions(false), { wrapper });

    expect(result.current).toEqual({});
    expect(fetchSessionFacets).not.toHaveBeenCalled();
  });
});

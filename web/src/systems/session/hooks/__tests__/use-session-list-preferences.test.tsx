// Suite: daemon-backed session-list preferences
// Invariant: full-section preference writes never overlap, so an older response
// cannot overwrite a newer sort or grouping choice, and writes never carry the
// deprecated `scope`. An old client's `legacy_scope` applies breadth once per newer
// explicit revision (ADR-004 / W-29): the first observation applies only
// `all-workspaces`, and defaults never apply.
// Owning layer: session-list preference hook; the daemon's shell settings route is the
// mocked I/O boundary.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { rehydrateStore } from "@xstate/store/persist";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  ACTIVE_WORKSPACE_PERSIST_KEY,
  activeWorkspaceStore,
  setActiveWorkspaceId,
} from "@/systems/workspace";

import { useSessionListPreferences } from "../use-session-list-preferences";

type LegacyScope = { value: "workspace" | "all-workspaces"; explicit: boolean; revision: number };

const daemon: {
  sessions: { sort?: string; group_by?: string };
  legacy?: LegacyScope;
  hangWrites: boolean;
} = { sessions: {}, hangWrites: false };

function json(body: unknown) {
  return new Response(JSON.stringify(body), { headers: { "content-type": "application/json" } });
}

let fetcher: ReturnType<typeof vi.spyOn<typeof globalThis, "fetch">>;

function writes() {
  return fetcher.mock.calls.flatMap(([input]) => {
    const request = input as Request;
    return request.method === "PATCH" ? [request] : [];
  });
}

function renderPreferences() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
  return renderHook(() => useSessionListPreferences(), { wrapper });
}

function breadth() {
  return activeWorkspaceStore.getSnapshot().context.breadthMode;
}

function appliedRevision() {
  return activeWorkspaceStore.getSnapshot().context.legacySessionsScopeApplied;
}

/** A browser that has never observed a legacy revision, focused on one project. */
async function freshBrowser() {
  window.localStorage.setItem(
    ACTIVE_WORKSPACE_PERSIST_KEY,
    JSON.stringify({
      context: {
        breadthMode: "focused",
        focus: { kind: "user" },
        selectedWorkspaceId: null,
        worktreeByScope: {},
        worktreePartitionByScope: {},
        legacySessionsScopeApplied: null,
      },
      version: 2,
    })
  );
  await rehydrateStore(activeWorkspaceStore);
  setActiveWorkspaceId("ws_alpha");
}

describe("useSessionListPreferences", () => {
  beforeEach(async () => {
    daemon.sessions = {};
    daemon.legacy = undefined;
    daemon.hangWrites = false;
    fetcher = vi.spyOn(globalThis, "fetch").mockImplementation(async input => {
      const request = input as Request;
      if (new URL(request.url).pathname !== "/api/settings/shell") {
        return new Response(null, { status: 404 });
      }
      if (request.method === "PATCH") {
        if (daemon.hangWrites) return new Promise<Response>(() => undefined);
        return json({ applied: true });
      }
      return json({
        config: { sessions: { ...daemon.sessions } },
        ...(daemon.legacy ? { legacy_scope: daemon.legacy } : {}),
      });
    });
    await freshBrowser();
  });

  afterEach(() => {
    fetcher.mockRestore();
  });

  it("Should default an unset sort to Created and grouping to none", async () => {
    const { result } = renderPreferences();
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.sort).toBe("created");
    expect(result.current.groupBy).toBe("none");
  });

  it.each(["created", "name", "last_activity", "attention", "manual"] as const)(
    "Should read a stored %s sort",
    async sort => {
      daemon.sessions = { sort };
      const { result } = renderPreferences();
      await waitFor(() => expect(result.current.sort).toBe(sort));
    }
  );

  it.each([
    ["project", "project"],
    ["profile", "profile"],
    ["galaxy", "none"],
  ] as const)("Should read stored grouping %s as %s", async (stored, groupBy) => {
    daemon.sessions = { group_by: stored };
    const { result } = renderPreferences();
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.groupBy).toBe(groupBy);
  });

  it("Should reject a second full-section write until the first settles, never sending scope", async () => {
    daemon.hangWrites = true;
    const { result } = renderPreferences();
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      result.current.setSort("attention");
      result.current.setGroupBy("project");
    });

    await waitFor(() => expect(writes()).toHaveLength(1));
    expect(await writes()[0]!.clone().json()).toEqual({
      config: { sessions: { sort: "attention", group_by: "none" } },
    });
    expect(result.current.sort).toBe("attention");
    expect(result.current.saving).toBe(true);
  });

  describe("W-29: legacy breadth", () => {
    it("Should record but never apply a default (non-explicit) scope", async () => {
      daemon.legacy = { value: "all-workspaces", explicit: false, revision: 4 };
      renderPreferences();
      await waitFor(() => expect(appliedRevision()).toBe(4));
      expect(breadth()).toBe("focused");
    });

    it("Should apply an explicit all-workspaces on first observation", async () => {
      daemon.legacy = { value: "all-workspaces", explicit: true, revision: 2 };
      renderPreferences();
      await waitFor(() => expect(breadth()).toBe("all"));
    });

    it("Should record an explicit workspace on first observation without leaving All projects", async () => {
      act(() => activeWorkspaceStore.trigger.allProjectsEnabled());
      daemon.legacy = { value: "workspace", explicit: true, revision: 2 };
      renderPreferences();
      await waitFor(() => expect(appliedRevision()).toBe(2));
      expect(breadth()).toBe("all");
    });

    it("Should apply each newer explicit revision exactly once", async () => {
      daemon.legacy = { value: "all-workspaces", explicit: true, revision: 2 };
      const first = renderPreferences();
      await waitFor(() => expect(breadth()).toBe("all"));
      first.unmount();

      // An old client turned All projects off: a newer explicit revision applies.
      daemon.legacy = { value: "workspace", explicit: true, revision: 3 };
      const second = renderPreferences();
      await waitFor(() => expect(breadth()).toBe("focused"));
      second.unmount();

      // The operator turns All projects back on here; re-reading the same
      // revision (a refetch, a remount) does not undo that choice.
      act(() => activeWorkspaceStore.trigger.allProjectsEnabled());
      const third = renderPreferences();
      await waitFor(() => expect(third.result.current.loading).toBe(false));
      expect(appliedRevision()).toBe(3);
      expect(breadth()).toBe("all");
    });

    it("Should ignore an older revision than the one already applied", async () => {
      daemon.legacy = { value: "workspace", explicit: false, revision: 7 };
      const first = renderPreferences();
      await waitFor(() => expect(appliedRevision()).toBe(7));
      first.unmount();

      daemon.legacy = { value: "all-workspaces", explicit: true, revision: 6 };
      const second = renderPreferences();
      await waitFor(() => expect(second.result.current.loading).toBe(false));
      expect(breadth()).toBe("focused");
      expect(appliedRevision()).toBe(7);
    });
  });
});

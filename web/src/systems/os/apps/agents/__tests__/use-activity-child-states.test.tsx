// Suite: Activity child-state catalog reads
// Invariant: every child-state read is bound to one workspace and one profile,
// waits for scope to resolve, reads each root's catalog to completion, and fails
// open per root — a slow or failed catalog claims nothing rather than reporting
// live children as gone.
// Owning layer: `useActivityChildStates`. Canonical suite: this file.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";
import { compozyApiMock } from "@/storybook/openapi-msw";

import type { AgentCommsScope } from "@/systems/agent-comms";

import { useActivityChildStates } from "../use-activity-child-states";

const WORKSPACE = "ws_main";
const ROOT = "ses_root";

const SCOPE: AgentCommsScope = {
  workspaceId: WORKSPACE,
  profileKey: "default",
  params: { profile: "default" },
  actingProfile: "default",
};

interface SeededSession {
  id: string;
  /** Defaults to a live child; only the parked cases need to say otherwise. */
  state?: string;
  stop_detail?: string;
}

let requests: URL[] = [];
let pages: { sessions: SeededSession[]; next?: string }[] = [];
let failNext = false;
let rootPages = new Map<string, { sessions: SeededSession[]; next?: string }[]>();
let failingRoots = new Set<string>();

function sessionPage(seeded: SeededSession[], nextCursor?: string) {
  return {
    sessions: seeded.map(item => ({
      id: item.id,
      name: item.id,
      agent_name: "reviewer",
      workspace_id: WORKSPACE,
      state: item.state ?? "active",
      ...(item.stop_detail === undefined ? {} : { stop_detail: item.stop_detail }),
    })),
    page: { total: seeded.length, has_more: Boolean(nextCursor), next_cursor: nextCursor },
  };
}

function handlers() {
  return [
    compozyApiMock.get("/api/sessions", ({ request, response }) => {
      const url = new URL(request.url);
      requests.push(url);
      const root = url.searchParams.get("root") ?? "";
      if (failNext || failingRoots.has(root)) {
        return response(500).json({ error: "catalog unreachable" });
      }
      const cursor = url.searchParams.get("cursor");
      const index = cursor === null ? 0 : Number(cursor);
      const page = (rootPages.get(root) ?? pages)[index] ?? { sessions: [] };
      // The generated response type demands every session field; these cases
      // only exercise id/state/stop_detail, so the page is deliberately partial.
      return response(200).json(sessionPage(page.sessions, page.next) as never);
    }),
  ];
}

function setup(
  scope: AgentCommsScope,
  roots: { rootSessionId: string; childSessionIds: string[] }[]
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return {
    ...renderHook(() => useActivityChildStates(scope, roots, false), { wrapper }),
    queryClient,
  };
}

beforeEach(() => {
  requests = [];
  pages = [];
  failNext = false;
  rootPages = new Map();
  failingRoots = new Set();
  vi.stubGlobal("fetch", createMswFetch(handlers));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useActivityChildStates — scope", () => {
  it("Should bind every request and cache key to one workspace and one profile", async () => {
    pages = [{ sessions: [{ id: "ses_child" }] }];
    const { queryClient, result } = setup(SCOPE, [
      { rootSessionId: ROOT, childSessionIds: ["ses_child"] },
    ]);
    await waitFor(() => expect(result.current.size).toBe(1));

    const asked = requests[0]!;
    expect(asked.searchParams.get("workspace_id")).toBe(WORKSPACE);
    expect(asked.searchParams.get("root")).toBe(ROOT);
    expect(asked.searchParams.get("profile")).toBe("default");
    // The read that would leak: `useSessions` defaults to every workspace when
    // its workspace argument is blank, which is why this hook never uses it.
    expect(asked.searchParams.get("all_workspaces")).toBeNull();

    const queryKey = queryClient.getQueryCache().getAll()[0]?.queryKey;
    expect(queryKey).toContain(WORKSPACE);
    expect(queryKey).toContainEqual(
      expect.objectContaining({ workspace_id: WORKSPACE, root: ROOT, profile: "default" })
    );
  });

  it("Should ask nothing at all until the workspace resolves", async () => {
    pages = [{ sessions: [{ id: "ses_child" }] }];
    const { result } = setup({ ...SCOPE, workspaceId: "" }, [
      { rootSessionId: ROOT, childSessionIds: ["ses_child"] },
    ]);

    await waitFor(() => expect(result.current.size).toBe(0));
    expect(requests).toEqual([]);
  });
});

describe("useActivityChildStates — completeness", () => {
  it("Should follow a root's catalog to the last page before judging absence", async () => {
    // `ses_late` is only on page two. Judging from page one would call a live
    // child gone.
    pages = [{ sessions: [{ id: "ses_early" }], next: "1" }, { sessions: [{ id: "ses_late" }] }];
    const { result } = setup(SCOPE, [
      { rootSessionId: ROOT, childSessionIds: ["ses_early", "ses_late"] },
    ]);

    await waitFor(() => expect(result.current.size).toBe(2));
    expect(result.current.get("ses_late")).toBe("running");
    expect(requests).toHaveLength(2);
  });

  it("Should report a child the complete catalog omits as gone", async () => {
    pages = [{ sessions: [{ id: "ses_alive" }] }];
    const { result } = setup(SCOPE, [
      { rootSessionId: ROOT, childSessionIds: ["ses_alive", "ses_reaped"] },
    ]);

    await waitFor(() => expect(result.current.size).toBe(2));
    expect(result.current.get("ses_reaped")).toBe("gone");
  });

  it("Should read a settlement-parked child as parked", async () => {
    pages = [
      {
        sessions: [{ id: "ses_parked", state: "stopped", stop_detail: "call child parked" }],
      },
    ];
    const { result } = setup(SCOPE, [{ rootSessionId: ROOT, childSessionIds: ["ses_parked"] }]);

    await waitFor(() => expect(result.current.get("ses_parked")).toBe("parked"));
  });

  it("Should read a stopped non-parked child as gone", async () => {
    pages = [
      {
        sessions: [{ id: "ses_failed", state: "stopped", stop_detail: "provider crashed" }],
      },
    ];
    const { result } = setup(SCOPE, [{ rootSessionId: ROOT, childSessionIds: ["ses_failed"] }]);

    await waitFor(() => expect(result.current.get("ses_failed")).toBe("gone"));
  });
});

describe("useActivityChildStates — failing open", () => {
  it("Should claim nothing for a root whose catalog errored", async () => {
    failNext = true;
    const { result } = setup(SCOPE, [
      { rootSessionId: ROOT, childSessionIds: ["ses_alive", "ses_reaped"] },
    ]);

    // A transport failure must never read as "these children are gone".
    await waitFor(() => expect(requests.length).toBeGreaterThan(0));
    expect(result.current.size).toBe(0);
  });

  it("Should preserve a healthy root when another root's catalog errored", async () => {
    const healthyRoot = "ses_root_healthy";
    const failedRoot = "ses_root_failed";
    rootPages.set(healthyRoot, [{ sessions: [{ id: "ses_healthy_child" }] }]);
    failingRoots.add(failedRoot);

    const { result } = setup(SCOPE, [
      { rootSessionId: healthyRoot, childSessionIds: ["ses_healthy_child"] },
      { rootSessionId: failedRoot, childSessionIds: ["ses_failed_child"] },
    ]);

    await waitFor(() => expect(requests).toHaveLength(2));
    await waitFor(() => expect(result.current.get("ses_healthy_child")).toBe("running"));
    expect(result.current.has("ses_failed_child")).toBe(false);
  });
});

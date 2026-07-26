import { afterEach, describe, expect, it, vi } from "vitest";

import { type NavCountKey, createDefaultFetchers } from "../nav-counts-store";

const FIXTURES: Record<string, unknown> = {
  "/api/observe/tasks/dashboard": {
    dashboard: { totals: { tasks_total: 14 } },
  },
  "/api/automation/jobs": { jobs: [{ id: "j1" }, { id: "j2" }, { id: "j3" }] },
  "/api/network/status": {
    network: {
      enabled: true,
      status: "active",
      local_peers: 2,
      channels: 4,
    },
  },
  "/api/automation/triggers": { triggers: [{ id: "t1" }, { id: "t2" }] },
  "/api/agents": {
    agents: [{ name: "a1" }, { name: "a2" }, { name: "a3" }, { name: "a4" }],
  },
  "/api/memory": { memories: [{ filename: "m1.md" }] },
  "/api/skills": {
    skills: [{ name: "s1" }, { name: "s2" }, { name: "s3" }, { name: "s4" }, { name: "s5" }],
  },
  "/api/bridges": { bridge_health: {}, bridges: [{ id: "b1" }, { id: "b2" }] },
};

const EXPECTED_COUNTS: Record<NavCountKey, number> = {
  tasks: 14,
  jobs: 3,
  network: 6,
  triggers: 2,
  agents: 4,
  knowledge: 1,
  skills: 5,
  bridges: 2,
};

function urlOf(input: RequestInfo | URL): string {
  if (typeof input === "string") return input;
  if (input instanceof URL) return input.href;
  return input.url;
}

function buildOkFetcher() {
  return vi.fn(async (input: RequestInfo | URL) => {
    const url = urlOf(input);
    const match = Object.keys(FIXTURES).find(prefix => url.includes(prefix));
    if (!match) {
      return new Response(JSON.stringify({ error: `fixture not found for ${url}` }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      });
    }
    return new Response(JSON.stringify(FIXTURES[match]), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  });
}

function capturedUrl(fetcher: ReturnType<typeof buildOkFetcher>, pathname: string): URL {
  const match = fetcher.mock.calls
    .map(([input]) => urlOf(input))
    .find(url => new URL(url, "http://agh.test").pathname === pathname);
  if (!match) {
    throw new Error("No request captured for " + pathname);
  }
  return new URL(match, "http://agh.test");
}

function expectQueryParams(
  fetcher: ReturnType<typeof buildOkFetcher>,
  pathname: string,
  expected: Record<string, string>
): void {
  const params = capturedUrl(fetcher, pathname).searchParams;
  for (const [key, value] of Object.entries(expected)) {
    expect(params.get(key), pathname + " query param " + key).toBe(value);
  }
}

describe("createDefaultFetchers", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("Should derive every nav-count key from its real endpoint", async () => {
    vi.stubGlobal("fetch", buildOkFetcher());
    const fetchers = createDefaultFetchers();
    const controller = new AbortController();
    for (const key of Object.keys(EXPECTED_COUNTS) as NavCountKey[]) {
      const result = await fetchers[key](controller.signal);
      expect(result.count, key).toBe(EXPECTED_COUNTS[key]);
    }
  });

  it("Should pass workspace scope to endpoints that support workspace counts", async () => {
    const fetcher = buildOkFetcher();
    vi.stubGlobal("fetch", fetcher);
    const fetchers = createDefaultFetchers("ws_alpha");
    const controller = new AbortController();

    for (const key of Object.keys(EXPECTED_COUNTS) as NavCountKey[]) {
      const result = await fetchers[key](controller.signal);
      expect(result.count, key).toBe(EXPECTED_COUNTS[key]);
    }

    expectQueryParams(fetcher, "/api/observe/tasks/dashboard", {
      scope: "workspace",
      workspace: "ws_alpha",
    });
    expectQueryParams(fetcher, "/api/automation/jobs", {
      scope: "workspace",
      workspace_id: "ws_alpha",
    });
    expectQueryParams(fetcher, "/api/automation/triggers", {
      scope: "workspace",
      workspace_id: "ws_alpha",
    });
    expectQueryParams(fetcher, "/api/agents", { workspace: "ws_alpha" });
    expectQueryParams(fetcher, "/api/memory", {
      scope: "workspace",
      workspace_id: "ws_alpha",
    });
    expectQueryParams(fetcher, "/api/skills", { workspace: "ws_alpha" });
    expectQueryParams(fetcher, "/api/bridges", {
      scope: "workspace",
      workspace_id: "ws_alpha",
    });
    expect([...capturedUrl(fetcher, "/api/network/status").searchParams.keys()]).toEqual([]);
  });

  it("Should throw when an endpoint returns a non-OK response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("oops", { status: 500 }))
    );
    const fetchers = createDefaultFetchers();
    const controller = new AbortController();
    for (const key of Object.keys(EXPECTED_COUNTS) as NavCountKey[]) {
      await expect(fetchers[key](controller.signal)).rejects.toThrowError(/snapshot failed/);
    }
  });
});

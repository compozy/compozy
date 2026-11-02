import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import type { ReactNode } from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { resetProfileViews, setProfileView } from "@/systems/profiles";
import { handlers as profileHandlers } from "@/systems/profiles/mocks";

import { fetchAgentInstanceCheckIns } from "../../adapters/agent-instance-api";
import { useTryCheck } from "../use-agent-instance-checks";
import { agentKeys } from "../../lib/query-keys";
import { agentPayloadFixture, teamItemFixture } from "../../mocks/team-fixtures";
import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import {
  applyAgentInstanceUpdate,
  useAgentInstanceActions,
  useAgentInstanceCapacity,
  useAgentInstances,
  useAgentDefinitionInstances,
} from "../use-agent-instances";

const server = setupServer(...profileHandlers);

beforeAll(() => server.listen());
afterAll(() => server.close());

const CAPACITY = { default: { limit: 10, active: 2 } };
const defaultList = agentKeys.instanceList("default", ["default"]);

function cached(client: QueryClient) {
  return client.getQueryData<{ instances: unknown[] }>(defaultList)?.instances;
}

describe("applyAgentInstanceUpdate", () => {
  it("Should patch a known instance in place from agent_instance.updated", () => {
    const client = new QueryClient();
    const first = agentInstanceFixture({ id: "a" });
    const second = agentInstanceFixture({ id: "b" });
    client.setQueryData(defaultList, { instances: [first, second], capacity: CAPACITY });
    const { profile_name: _, ...wire } = second;

    applyAgentInstanceUpdate(client, { ...wire, activity: "working" });

    expect(cached(client)).toEqual([first, { ...second, activity: "working" }]);
  });

  it("Should patch every view that holds the instance", () => {
    const client = new QueryClient();
    const row = agentInstanceFixture({ id: "a", profile_name: "work" });
    const work = agentKeys.instanceList("work", ["work"]);
    const aggregate = agentKeys.instanceList("@all", ["default", "work"]);
    client.setQueryData(work, { instances: [row], capacity: {} });
    client.setQueryData(aggregate, { instances: [row], capacity: {} });

    applyAgentInstanceUpdate(client, { ...row, activity: "working" });

    for (const key of [work, aggregate]) {
      expect(client.getQueryData<{ instances: (typeof row)[] }>(key)?.instances[0]?.activity).toBe(
        "working"
      );
    }
  });

  it("Should reread for an instance it hasn't seen", () => {
    const client = new QueryClient();
    client.setQueryData(defaultList, { instances: [], capacity: CAPACITY });
    const invalidate = vi.spyOn(client, "invalidateQueries");

    applyAgentInstanceUpdate(client, agentInstanceFixture());

    expect(invalidate).toHaveBeenCalledWith({ queryKey: agentKeys.instanceLists() });
  });

  it("Should reread the server's count when a row gives back its slot", () => {
    const client = new QueryClient();
    const row = agentInstanceFixture({ id: "a" });
    client.setQueryData(defaultList, { instances: [row], capacity: CAPACITY });
    const invalidate = vi.spyOn(client, "invalidateQueries");

    applyAgentInstanceUpdate(client, { ...row, state: "paused", pause_reason: "fatigue" });

    expect(invalidate).toHaveBeenCalledWith({ queryKey: agentKeys.instanceLists() });
  });
});

describe("instance routes and the selected profile", () => {
  afterEach(() => {
    resetProfileViews();
    server.resetHandlers();
  });

  function wrapper({ children }: { children: ReactNode }) {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  }

  // Invariant: a late check response cannot replace a newer request or another instance.
  // Owner and canonical suite: agent instance interaction hooks, HTTP I/O boundary.
  it.each([false, true])(
    "Should ignore a stale check when switching instances is %s",
    async switched => {
      const replies: ((response: Response) => void)[] = [];
      server.use(
        http.post(
          "*/api/agent-instances/:id/check",
          () => new Promise<Response>(resolve => replies.push(resolve))
        )
      );
      const { result, rerender } = renderHook(
        ({ id }) => useTryCheck({ id, profile_name: "work" }),
        {
          initialProps: { id: "first" },
          wrapper,
        }
      );
      let first!: Promise<void>;
      act(() => {
        first = result.current.tryIt();
      });
      await waitFor(() => expect(replies).toHaveLength(1));
      if (switched) rerender({ id: "second" });
      let second!: Promise<void>;
      act(() => {
        second = result.current.tryIt();
      });
      await waitFor(() => expect(replies).toHaveLength(2));
      await act(async () => {
        replies[0]!(
          HttpResponse.json({ gate: "would_run", digest: [{ key: "old", summary: "Old" }] })
        );
        await first;
      });
      expect(result.current.state.phase).toBe("running");
      await act(async () => {
        replies[1]!(
          HttpResponse.json({ gate: "would_run", digest: [{ key: "new", summary: "New" }] })
        );
        await second;
      });
      expect(result.current.state).toEqual({
        phase: "result",
        digest: [{ key: "new", summary: "New" }],
      });
    }
  );

  /** Serves each profile only its own rows, the way the daemon's id and list routes do. */
  function serveProfiles(rows: Record<string, ReturnType<typeof agentInstanceFixture>[]>) {
    const seen: string[] = [];
    server.use(
      http.get("*/api/agent-instances", ({ request }) => {
        const url = new URL(request.url);
        const profile = url.searchParams.get("profile") ?? "default";
        seen.push(`GET ${url.pathname}?profile=${profile}`);
        return HttpResponse.json({
          instances: rows[profile] ?? [],
          next_cursor: "",
          max_active_instances: profile === "work" ? 25 : 10,
          active_count: (rows[profile] ?? []).length,
        });
      }),
      http.post("*/api/agent-instances/:id/pause", ({ params, request }) => {
        const profile = new URL(request.url).searchParams.get("profile") ?? "default";
        seen.push(`POST ${String(params.id)}/pause?profile=${profile}`);
        return HttpResponse.json({ instance: agentInstanceFixture({ id: String(params.id) }) });
      }),
      http.post("*/api/agent-instances", ({ request }) => {
        const profile = new URL(request.url).searchParams.get("profile") ?? "default";
        seen.push(`POST create?profile=${profile}`);
        return HttpResponse.json({ instance: agentInstanceFixture() }, { status: 201 });
      })
    );
    return seen;
  }

  it("Should read and create in the selected profile, never the default", async () => {
    setProfileView({ kind: "profile", profile: "work" });
    const seen = serveProfiles({ work: [agentInstanceFixture({ id: "a" })] });
    const { result } = renderHook(
      () => ({ instances: useAgentInstances(), actions: useAgentInstanceActions() }),
      { wrapper }
    );

    await waitFor(() => expect(result.current.instances[0]?.profile_name).toBe("work"));
    act(() => result.current.actions.add({ agent: "triage", workspace: "@user" }));

    await waitFor(() => expect(seen).toContain("POST create?profile=work"));
    expect(seen).toContain("GET /api/agent-instances?profile=work");
    expect(seen.some(entry => entry.endsWith("profile=default"))).toBe(false);
  });

  // Invariant: a definition sees only its destination profile and resolved project scopes.
  // Owner: instance association hook; canonical agent instance route suite.
  it("Should isolate same-named definitions across projects and profiles", async () => {
    setProfileView({ kind: "aggregate" });
    serveProfiles({
      default: [
        agentInstanceFixture({ id: "mine", workspace_id: "ws-compozy" }),
        agentInstanceFixture({ id: "shadow", workspace_id: "ws-advos" }),
      ],
      marketing: [agentInstanceFixture({ id: "foreign", workspace_id: "ws-compozy" })],
    });
    const agent = agentPayloadFixture({ definition_digest: "mine" });
    server.use(
      http.get("*/api/agents/catalog", () =>
        HttpResponse.json({
          items: [
            teamItemFixture(agent, { scopes: ["ws-compozy"], definition_digest: "mine" }),
            teamItemFixture(
              { definition_digest: "shadow" },
              { scopes: ["ws-advos"], definition_digest: "shadow" }
            ),
          ],
          page: { has_more: false, next_cursor: "" },
        })
      )
    );
    const { result } = renderHook(() => useAgentDefinitionInstances(agent), { wrapper });
    await waitFor(() => expect(result.current.map(instance => instance.id)).toEqual(["mine"]));
  });

  it("Should report the destination profile's slots from the list envelope", async () => {
    setProfileView({ kind: "profile", profile: "work" });
    serveProfiles({ work: [agentInstanceFixture({ id: "a" })] });

    const { result } = renderHook(() => useAgentInstanceCapacity(), { wrapper });

    await waitFor(() => expect(result.current).toEqual({ limit: 25, active: 1, reached: false }));
  });

  it("Should read every profile under the aggregate and act on a row in its own", async () => {
    setProfileView({ kind: "aggregate" });
    const seen = serveProfiles({
      default: [agentInstanceFixture({ id: "mine" })],
      marketing: [agentInstanceFixture({ id: "theirs" })],
    });
    const { result } = renderHook(
      () => ({ instances: useAgentInstances(), actions: useAgentInstanceActions() }),
      { wrapper }
    );

    await waitFor(() => expect(result.current.instances.length).toBe(2));
    const row = result.current.instances.find(item => item.id === "theirs");
    expect(row?.profile_name).toBe("marketing");
    act(() => result.current.actions.pause(row!));

    await waitFor(() => expect(seen).toContain("POST theirs/pause?profile=marketing"));
  });
});

describe("fetchAgentInstanceCheckIns", () => {
  it("Should follow the cursor without asking past the daemon's page cap", async () => {
    const limits: string[] = [];
    server.use(
      http.get("*/api/agent-instances/:id/check-ins", ({ request }) => {
        const url = new URL(request.url);
        limits.push(url.searchParams.get("limit") ?? "");
        const start = Number(url.searchParams.get("cursor") ?? 0);
        const size = Number(url.searchParams.get("limit"));
        return HttpResponse.json({
          check_ins: Array.from({ length: size }, (_, index) => ({ id: `c${start + index}` })),
          next_cursor: String(start + size),
        });
      })
    );

    const result = await fetchAgentInstanceCheckIns({ id: "a", profile_name: "default" }, 250);

    expect(limits).toEqual(["100", "100", "50"]);
    expect(result.check_ins).toHaveLength(250);
    expect(result.check_ins[249]?.id).toBe("c249");
  });
});

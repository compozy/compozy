import { http, HttpResponse } from "msw";
import { loopRunDetailByRunId } from "../../mocks/fixtures";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";
import { handlers } from "@/systems/loops/mocks";
import {
  useGoalTurns,
  useLoop,
  useLoopNodeInventory,
  useLoopAnnotations,
  useLoopConfig,
  useLoopRun,
  useLoopRuns,
  useLoops,
} from "@/systems/loops";

const WS = "ws_1";
const selection = vi.hoisted(() => ({ profile: "default" }));
vi.mock("@/systems/profiles", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/profiles")>()),
  useProfileReadScope: () => ({ params: { profile: selection.profile }, key: selection.profile }),
}));

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("loop read hooks", () => {
  beforeEach(() => {
    selection.profile = "default";
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should report Goal history loading only while an enabled initial read is fetching", async () => {
    let finishRead!: (response: Response) => void;
    const fetch = vi.fn(
      () =>
        new Promise<Response>(resolve => {
          finishRead = resolve;
        })
    );
    vi.stubGlobal("fetch", fetch);
    const { result, rerender } = renderHook(
      ({ enabled }) => useGoalTurns(WS, "looprun_running", enabled, false),
      { initialProps: { enabled: false }, wrapper: createWrapper() }
    );
    expect(result.current.isLoading).toBe(false);
    expect(result.current.turns).toEqual([]);
    expect(fetch).not.toHaveBeenCalled();
    rerender({ enabled: true });
    await waitFor(() => expect(result.current.isLoading).toBe(true));
    finishRead(Response.json({ turns: [], next_after_seq: null }));
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isError).toBe(false);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("Should fetch the catalog through useLoops", async () => {
    // Invariant: the hook returns every catalog row and derives totals/facets from the server page.
    // The read-hook suite owns this boundary; adapter fixture shape stays covered in loops-api.test.
    const { result } = renderHook(() => useLoops(WS), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.loops.map(loop => loop.name)).toEqual([
      "implement-tasks",
      "review-and-fix",
    ]);
    expect(result.current.total).toBe(2);
    expect(result.current.facets).toEqual({
      categories: { Engineering: 2 },
      kinds: { read_only: 2 },
      statuses: { running: 2 },
    });
  });

  it("Should fetch a definition, config and annotations for one loop", async () => {
    const wrapper = createWrapper();
    const detail = renderHook(() => useLoop(WS, "implement-tasks"), { wrapper });
    await waitFor(() => expect(detail.result.current.isSuccess).toBe(true));
    expect(detail.result.current.data?.definition.meta.name).toBe("implement-tasks");

    const config = renderHook(() => useLoopConfig(WS, "implement-tasks"), { wrapper });
    await waitFor(() => expect(config.result.current.isSuccess).toBe(true));
    expect(config.result.current.data?.iteration_cap).toBe(16);

    const annotations = renderHook(() => useLoopAnnotations(WS, "implement-tasks"), { wrapper });
    await waitFor(() => expect(annotations.result.current.isSuccess).toBe(true));
    expect(annotations.result.current.data).toHaveLength(2);
  });

  it("Should fetch the workspace runs list and a single run detail", async () => {
    const wrapper = createWrapper();
    const runs = renderHook(() => useLoopRuns(WS, { loop: "implement-tasks" }), { wrapper });
    await waitFor(() => expect(runs.result.current.isSuccess).toBe(true));
    expect(runs.result.current.data?.runs.every(run => run.loop_name === "implement-tasks")).toBe(
      true
    );

    const run = renderHook(() => useLoopRun(WS, "looprun_running"), { wrapper });
    await waitFor(() => expect(run.result.current.isSuccess).toBe(true));
    expect(run.result.current.data?.run.status).toBe("running");
    expect(run.result.current.data?.executed_definition?.contract.goal).toContain(
      "{{ .inputs.slug }}"
    );
    expect(run.result.current.data?.materialized_contract.goal).toContain("loops-catalog-api");
    expect(run.result.current.data?.materialized_contract.goal).not.toContain("{{");
  });

  it("Should stay idle when the workspace is not resolved yet", () => {
    const { result } = renderHook(() => useLoops(""), { wrapper: createWrapper() });
    expect(result.current.fetchStatus).toBe("idle");
    expect(result.current.data).toBeUndefined();
  });

  it("Should not report loading while the node inventory query is disabled", () => {
    const { result } = renderHook(() => useLoopNodeInventory(WS, { state: "waiting" }, false), {
      wrapper: createWrapper(),
    });
    expect(result.current.isLoading).toBe(false);
  });
  it("Should read a completed run only in its selected Profile and isolate cached results on switch", async () => {
    const fixture = loopRunDetailByRunId.get("looprun_running");
    if (!fixture) throw new Error("run fixture missing");
    selection.profile = "engineering";
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => [
        http.get("*/api/workspaces/:workspace/loop-runs/:run", ({ request }) => {
          if (new URL(request.url).searchParams.get("profile") !== "engineering") {
            return new HttpResponse(null, { status: 404 });
          }
          return HttpResponse.json({
            ...fixture,
            run: {
              ...fixture.run,
              status: "done",
              profile_id: "profile-engineering",
              profile_name: "engineering",
            },
          });
        }),
        ...handlers,
      ])
    );
    const { result, rerender } = renderHook(() => useLoopRun(WS, "looprun_running"), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.run.status).toBe("done");
    selection.profile = "default";
    rerender();
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.data).toBeUndefined();
    selection.profile = "engineering";
    rerender();
    await waitFor(() => expect(result.current.data?.run.profile_name).toBe("engineering"));
  });
});

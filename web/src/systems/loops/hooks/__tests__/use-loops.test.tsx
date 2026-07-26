import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createMswFetch } from "@/test/msw-fetch";
import { handlers } from "@/systems/loops/mocks";
import {
  mergeGoalTurnTimeline,
  mergeGoalTurns,
  useGoalTurns,
  useLoop,
  useLoopAnnotations,
  useLoopConfig,
  useLoopRun,
  useLoopRuns,
  useLoops,
} from "@/systems/loops";
import type { GoalTurn, LoopGoalTurnLive } from "@/systems/loops";

const WS = "ws_1";

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function goalTurn(overrides: Partial<GoalTurn> = {}): GoalTurn {
  return {
    actor_id: "implementer",
    actor_kind: "agent",
    binding_epoch: 1,
    binding_handle: "goal:abc",
    blocking_issues: [],
    ended_at: null,
    evidence_ref: null,
    generation: 1,
    item_index: 0,
    node_id: "goal",
    prompt_attempt: 1,
    prompt_id: "prompt_1",
    prompt_ref: null,
    reason_code: null,
    result_status: null,
    seq: 1,
    session_id: "session_1",
    started_at: "2026-07-10T12:00:00Z",
    stop_reason: null,
    tokens_used: null,
    turn: 1,
    verdict_outcome: null,
    ...overrides,
  };
}

describe("loop read hooks", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should fetch the catalog through useLoops", async () => {
    const { result } = renderHook(() => useLoops(WS), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.loops.map(loop => loop.name)).toEqual([
      "software-delivery",
      "reviews-watch",
    ]);
    expect(result.current.total).toBe(2);
    expect(result.current.facets).toEqual({
      categories: { delivery: 1, watch: 1 },
      kinds: { read_only: 1, workspace: 1 },
      statuses: { running: 1, watching: 1 },
    });
  });

  it("Should fetch a definition, config and annotations for one loop", async () => {
    const wrapper = createWrapper();
    const detail = renderHook(() => useLoop(WS, "software-delivery"), { wrapper });
    await waitFor(() => expect(detail.result.current.isSuccess).toBe(true));
    expect(detail.result.current.data?.definition.meta.name).toBe("software-delivery");

    const config = renderHook(() => useLoopConfig(WS, "software-delivery"), { wrapper });
    await waitFor(() => expect(config.result.current.isSuccess).toBe(true));
    expect(config.result.current.data?.iteration_cap).toBe(16);

    const annotations = renderHook(() => useLoopAnnotations(WS, "software-delivery"), { wrapper });
    await waitFor(() => expect(annotations.result.current.isSuccess).toBe(true));
    expect(annotations.result.current.data).toHaveLength(2);
  });

  it("Should fetch the workspace runs list and a single run detail", async () => {
    const wrapper = createWrapper();
    const runs = renderHook(() => useLoopRuns(WS, { loop: "software-delivery" }), { wrapper });
    await waitFor(() => expect(runs.result.current.isSuccess).toBe(true));
    expect(runs.result.current.data?.runs.every(run => run.loop_name === "software-delivery")).toBe(
      true
    );

    const run = renderHook(() => useLoopRun(WS, "looprun_running"), { wrapper });
    await waitFor(() => expect(run.result.current.isSuccess).toBe(true));
    expect(run.result.current.data?.run.status).toBe("running");
  });

  it("Should fetch an empty Goal turn page through the run-scoped infinite query", async () => {
    const turns = renderHook(() => useGoalTurns(WS, "looprun_running"), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(turns.result.current.isSuccess).toBe(true));
    expect(turns.result.current.data?.turns).toEqual([]);
    expect(turns.result.current.hasNextPage).toBe(false);
  });

  it("Should deduplicate persisted pages and let a completed live turn settle its open row", () => {
    const open = goalTurn();
    const completed = goalTurn({
      result_status: "completed",
      ended_at: "2026-07-10T12:01:00Z",
      stop_reason: "end_turn",
    });
    expect(mergeGoalTurns([open], [completed])).toEqual([completed]);

    const live: LoopGoalTurnLive = {
      seq: 1,
      generation: 1,
      nodeId: "goal",
      itemIndex: 0,
      turn: 1,
      promptAttempt: 1,
      promptId: "prompt_1",
      sessionId: "session_1",
      bindingHandle: "goal:abc",
      bindingEpoch: 1,
      actorKind: "agent",
      actorId: "implementer",
      resultStatus: "completed",
      stopReason: "end_turn",
      reasonCode: null,
      verdictOutcome: "approved",
      blockingIssues: [],
      evidenceRef: "blob_1",
      tokensUsed: 120,
      startedAt: "2026-07-10T12:00:00Z",
      endedAt: "2026-07-10T12:01:00Z",
    };
    expect(mergeGoalTurnTimeline([open], [live])).toEqual([
      expect.objectContaining({
        key: "goal:0:1:1:1:prompt_1",
        resultStatus: "completed",
        verdictOutcome: "approved",
        evidenceRef: "blob_1",
      }),
    ]);
  });

  it("Should stay idle when the workspace is not resolved yet", () => {
    const { result } = renderHook(() => useLoops(""), { wrapper: createWrapper() });
    expect(result.current.fetchStatus).toBe("idle");
    expect(result.current.data).toBeUndefined();
  });
});

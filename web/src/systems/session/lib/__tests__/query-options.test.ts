// Suite: session detail polling policy
// Invariant: durable starting sessions converge quickly while terminal sessions stop polling, and a
// session carrying a PR lookup is re-read at least every 60 s, only while the document is visible.
// Boundary IN: TanStack session detail query options.
// Boundary OUT: HTTP transport and lifecycle persistence, owned by their adapter/runtime suites.
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";

import { SessionApiError } from "../../adapters/session-api-errors";
import type { SessionInteractionRecord, SessionPayload } from "../../types";
import {
  sessionAcrossProfilesOptions,
  sessionCommandsOptions,
  sessionDetailOptions,
  sessionInputsOptions,
  sessionResolvedInteractionsOptions,
  sessionScopedDetailOptions,
  sessionUsageOptions,
  sessionUsageTurnsOptions,
} from "../query-options";
import { sessionKeys } from "../query-keys";
import {
  invalidateSessionLiveQueries,
  invalidateSessionMutationQueries,
  invalidateSessionUsageQueries,
  invalidateWorkspaceSessionCatalog,
} from "../session-query-invalidation";
import {
  rescheduleSessionStreamBackedReads,
  retainSessionStreamLiveness,
  sessionStreamBackedRefetchInterval,
} from "../session-stream-liveness";

function pollingIntervalFor(state: SessionPayload["state"] | undefined): number | false {
  const refetchInterval = sessionDetailOptions("ws_alpha", "sess_1").refetchInterval;
  if (typeof refetchInterval !== "function") {
    throw new Error("Session detail polling must be state-aware");
  }

  const interval = refetchInterval({
    state: { data: state ? ({ state } as SessionPayload) : undefined },
  } as never);
  if (interval === undefined) {
    throw new Error("Session detail polling returned no interval");
  }
  return interval;
}

describe("sessionDetailOptions", () => {
  it("Should stop polling missing sessions even when cached data still says active", () => {
    for (const options of [
      sessionDetailOptions("ws_alpha", "sess_1"),
      sessionScopedDetailOptions("sess_1", { profile: "default" }),
    ]) {
      if (typeof options.refetchInterval !== "function") throw new Error("expected interval");
      for (const status of [404, 410]) {
        expect(
          options.refetchInterval({
            state: { data: { state: "active" }, error: new SessionApiError("gone", status) },
          } as never)
        ).toBe(false);
      }
      expect(
        options.refetchInterval({
          state: { data: { state: "active" }, error: new SessionApiError("temporary", 503) },
        } as never)
      ).toBe(5000);
    }
  });

  it("Should poll starting sessions rapidly until startup reaches a durable result", () => {
    expect(pollingIntervalFor("starting")).toBe(500);
    expect(pollingIntervalFor("active")).toBe(5_000);
    expect(pollingIntervalFor("stopping")).toBe(5_000);
    expect(pollingIntervalFor("stopped")).toBe(false);
    expect(pollingIntervalFor(undefined)).toBe(false);
  });

  it("UT-072: re-reads a session with a PR lookup every 60 s, never while hidden", () => {
    const lookup = { state: "ok", cause: "", observed_at: null, stale: true } as const;
    for (const options of [
      sessionDetailOptions("ws_alpha", "sess_1"),
      sessionScopedDetailOptions("sess_1", { profile: "default" }),
    ]) {
      if (typeof options.refetchInterval !== "function") throw new Error("expected interval");
      const at = (data: Partial<SessionPayload>) =>
        (options.refetchInterval as (query: never) => number | false)({
          state: { data },
        } as never);
      // A stopped session keeps its chip fresh; a live one keeps its faster cadence.
      expect(at({ state: "stopped", pull_request_lookup: lookup })).toBe(60_000);
      expect(at({ state: "active", pull_request_lookup: lookup })).toBe(5_000);
      // No branch, no lookup: no extra reads.
      expect(at({ state: "stopped", pull_request_lookup: null })).toBe(false);
      // The interval pauses with the document (TanStack's background default).
      expect(options.refetchIntervalInBackground).not.toBe(true);
    }
  });
});

describe("sessionCommandsOptions", () => {
  it("Should key the command catalog by workspace and session", () => {
    const options = sessionCommandsOptions(" ws_alpha ", "sess_1");
    expect(options.queryKey).toEqual(["session-commands", "ws_alpha", "sess_1"]);
    expect(options.enabled).toBe(true);
    expect(sessionCommandsOptions("", "sess_1").enabled).toBe(false);
    expect(sessionCommandsOptions("   ", "sess_1").enabled).toBe(false);
    expect(sessionCommandsOptions("ws_alpha", "sess_1", false).enabled).toBe(false);
  });
});

describe("sessionScopedDetailOptions", () => {
  it("Should key the profile-enforced read by the lens so two profiles never share an entry", () => {
    expect(sessionScopedDetailOptions("sess_1", { profile: "marketing" }).queryKey).toEqual([
      "sessions",
      "by-id",
      "sess_1",
      "marketing",
    ]);
    expect(sessionScopedDetailOptions("sess_1", { profile: "marketing" }).queryKey).not.toEqual(
      sessionScopedDetailOptions("sess_1", { profile: "default" }).queryKey
    );
  });

  it("Should keep the id above the lens so one mutation can sweep every lens holding it", () => {
    const scoped = sessionScopedDetailOptions("sess_1", { profile: "marketing" }).queryKey;
    const aggregate = sessionAcrossProfilesOptions("sess_1").queryKey;
    // Both live under the same by-id prefix; a mutation knows the session, never
    // which lens some other surface is looking through.
    expect(scoped.slice(0, 3)).toEqual(["sessions", "by-id", "sess_1"]);
    expect(aggregate.slice(0, 3)).toEqual(["sessions", "by-id", "sess_1"]);
    expect(scoped).not.toEqual(aggregate);
  });

  it("Should give the aggregate the reserved identity rather than the default profile's", () => {
    expect(sessionAcrossProfilesOptions("sess_1").queryKey).not.toEqual(
      sessionScopedDetailOptions("sess_1", { profile: "default" }).queryKey
    );
  });

  it("Should refuse to run without a lens, because an absent selector resolves default", () => {
    expect(sessionScopedDetailOptions("sess_1", { profile: "" }).enabled).toBe(false);
    expect(sessionScopedDetailOptions("", { profile: "marketing" }).enabled).toBe(false);
  });

  it("Should not retry, so a 404 stays a boundary decision rather than a delay", () => {
    expect(sessionScopedDetailOptions("sess_1", { profile: "marketing" }).retry).toBe(false);
  });

  it("Should stop polling entirely when the window does not own the live tail", () => {
    const refetchInterval = sessionScopedDetailOptions(
      "sess_1",
      { profile: "marketing" },
      {
        liveTail: false,
      }
    ).refetchInterval;
    if (typeof refetchInterval !== "function") throw new Error("expected a state-aware interval");
    expect(refetchInterval({ state: { data: { state: "active" } } } as never)).toBe(false);
  });
});

describe("sessionResolvedInteractionsOptions", () => {
  const previousRows: SessionInteractionRecord[] = [
    {
      interaction_id: "int-1",
      kind: "permission",
      provider_request_id: "turn_001:perm_1",
      status: "resolved",
      created_at: "2026-09-05T10:00:00Z",
      resolution: "reject-once",
      resolved_by: "operator",
    },
  ];

  function placeholderFrom(
    options: ReturnType<typeof sessionResolvedInteractionsOptions>,
    previousKey: readonly unknown[]
  ) {
    const placeholderData = options.placeholderData;
    if (typeof placeholderData !== "function") throw new Error("expected a scoped placeholder");
    return placeholderData(previousRows, { queryKey: previousKey } as never);
  }

  it("Should fence the read by the decided request ids inside the session's interactions scope", () => {
    const options = sessionResolvedInteractionsOptions("ws_alpha", "sess_1", {
      decidedRequestIds: new Set(["b", "a"]),
    });
    expect(options.queryKey).toEqual([
      "sessions",
      "workspace",
      "ws_alpha",
      "detail",
      "sess_1",
      "interactions",
      "resolved",
      ["a", "b"],
    ]);
    expect(sessionResolvedInteractionsOptions("", "sess_1").enabled).toBe(false);
    expect(
      sessionResolvedInteractionsOptions("ws_alpha", "sess_1", { enabled: false }).enabled
    ).toBe(false);
  });

  it("Should keep previous rows only across a fence change in the same workspace and session", () => {
    const previous = sessionResolvedInteractionsOptions("ws_alpha", "sess_1", {
      decidedRequestIds: new Set(["turn_001:perm_1"]),
    });
    const next = sessionResolvedInteractionsOptions("ws_alpha", "sess_1", {
      decidedRequestIds: new Set(["turn_001:perm_1", "turn_001:perm_2"]),
    });

    expect(placeholderFrom(next, previous.queryKey)).toBe(previousRows);
  });

  it("Should never present another session's or workspace's rows while the new read is in flight", () => {
    const previous = sessionResolvedInteractionsOptions("ws_alpha", "sess_1", {
      decidedRequestIds: new Set(["turn_001:perm_1"]),
    });
    const otherSession = sessionResolvedInteractionsOptions("ws_alpha", "sess_2", {
      decidedRequestIds: new Set(["turn_001:perm_1"]),
    });
    const otherWorkspace = sessionResolvedInteractionsOptions("ws_beta", "sess_1", {
      decidedRequestIds: new Set(["turn_001:perm_1"]),
    });

    expect(placeholderFrom(otherSession, previous.queryKey)).toBeUndefined();
    expect(placeholderFrom(otherWorkspace, previous.queryKey)).toBeUndefined();
    expect(placeholderFrom(otherSession, [])).toBeUndefined();
  });
});

// Owner: session query policy; transport and daemon lifecycle are covered by integration suites.
describe("sessionInputsOptions", () => {
  it("Should read stopped queues once and poll only live sessions", () => {
    for (const state of ["starting", "active", "stopping", "stopped", undefined] as const) {
      const options = sessionInputsOptions("ws_alpha", "sess_1", { sessionState: state });
      expect(options.enabled).toBe(true);
      if (typeof options.refetchInterval !== "function")
        throw new Error("expected lifecycle polling");
      expect(options.refetchInterval({ state: { error: null } } as never)).toBe(
        state && state !== "stopped" ? 5_000 : false
      );
    }
  });

  it("Should stop retrying terminal queue responses while preserving backend failures", () => {
    const options = sessionInputsOptions("ws_alpha", "sess_1", {
      sessionState: "active",
      refetchInterval: 1_000,
    });
    if (typeof options.refetchInterval !== "function" || typeof options.retry !== "function") {
      throw new Error("expected bounded error policy");
    }
    for (const error of [
      new SessionApiError("gone", 404),
      new SessionApiError("gone", 410),
      new SessionApiError("stopped", 400, "sess_1", { code: "session_not_promptable" }),
    ]) {
      expect(options.retry(0, error)).toBe(false);
      expect(options.refetchInterval({ state: { error } } as never)).toBe(false);
    }
    const error = new SessionApiError("queue storage failed", 500);
    expect(options.retry(0, error)).toBe(true);
    expect(options.retry(1, error)).toBe(false);
    expect(options.refetchInterval({ state: { error } } as never)).toBe(1_000);
    expect(sessionInputsOptions("ws_alpha", "sess_2").queryKey).not.toEqual(options.queryKey);
  });
});

// Owner: session query policy. A live session stream invalidates these reads as they change,
// so every stream-backed read drops to the safety cadence while that session's stream is live.
describe("stream-backed session reads", () => {
  // Invariant: losing the live edge re-arms every mounted read and profile lens
  // for that session immediately, without fetching or touching another session.
  // Owning layer: session query policy; canonical suite: stream-backed reads.
  it("Should rearm scoped observers with custom hashes when the stream stops", async () => {
    vi.useFakeTimers();
    const client = new QueryClient({
      defaultOptions: {
        queries: {
          gcTime: Infinity,
          staleTime: Infinity,
          queryKeyHashFn: key => `custom:${JSON.stringify(key)}`,
        },
      },
    });
    const sessionId = "sess_rearm";
    const release = retainSessionStreamLiveness(sessionId);
    const requests = vi.fn(async () => ({ state: "active" }));
    const targetKeys = [
      sessionKeys.detail("ws_alpha", sessionId),
      sessionKeys.inputQueue("ws_alpha", sessionId),
      sessionKeys.usage("ws_alpha", sessionId),
      sessionKeys.usageTurns("ws_alpha", sessionId),
      sessionKeys.byId(sessionId, "profile-a"),
      sessionKeys.byId(sessionId, "profile-b"),
    ];
    const unsubscribes = targetKeys.map(queryKey => {
      const observer = new QueryObserver(client, {
        queryKey,
        queryFn: requests,
        initialData: { state: "active" },
        refetchInterval: () => sessionStreamBackedRefetchInterval(sessionId, "active"),
      });
      return observer.subscribe(() => undefined);
    });
    const unrelated = vi.fn(async () => ({ state: "active" }));
    const otherObserver = new QueryObserver(client, {
      queryKey: sessionKeys.detail("ws_alpha", "other-session"),
      queryFn: unrelated,
      initialData: { state: "active" },
      refetchInterval: 30_000,
    });
    unsubscribes.push(otherObserver.subscribe(() => undefined));
    try {
      release();
      rescheduleSessionStreamBackedReads(client, "ws_alpha", sessionId);
      expect(requests).not.toHaveBeenCalled();
      await vi.advanceTimersByTimeAsync(4_999);
      expect(requests).not.toHaveBeenCalled();
      await vi.advanceTimersByTimeAsync(1);
      expect(requests).toHaveBeenCalledTimes(6);
      expect(unrelated).not.toHaveBeenCalled();
    } finally {
      release();
      for (const unsubscribe of unsubscribes) unsubscribe();
      client.clear();
      vi.useRealTimers();
    }
  });

  function cadences(sessionId: string, state: SessionPayload["state"]) {
    const call = (interval: unknown, query: unknown) => {
      if (interval === false) return false;
      if (typeof interval !== "function") throw new Error("expected a stream-aware cadence");
      return interval(query as never) as number | false;
    };
    const data = { state: { data: { state } as SessionPayload, error: null } };
    return {
      detail: call(sessionDetailOptions("ws_alpha", sessionId).refetchInterval, data),
      byId: call(
        sessionScopedDetailOptions(sessionId, { all_profiles: true }).refetchInterval,
        data
      ),
      inputs: call(
        sessionInputsOptions("ws_alpha", sessionId, { sessionState: state }).refetchInterval,
        data
      ),
      controlPollInputs: call(
        sessionInputsOptions("ws_alpha", sessionId, { sessionState: state, refetchInterval: 1_000 })
          .refetchInterval,
        data
      ),
      usage: call(sessionUsageOptions("ws_alpha", sessionId, state).refetchInterval, data),
      usageTurns: call(
        sessionUsageTurnsOptions("ws_alpha", sessionId, state).refetchInterval,
        data
      ),
    };
  }

  it("Should use the safety cadence only while the session's own stream is live", () => {
    const fallback = {
      detail: 5_000,
      byId: 5_000,
      inputs: 5_000,
      controlPollInputs: 1_000,
      usage: 5_000,
      usageTurns: 5_000,
    };
    expect(cadences("sess_stream", "active")).toEqual(fallback);

    const release = retainSessionStreamLiveness("sess_stream");
    const second = retainSessionStreamLiveness("sess_stream");
    try {
      expect(cadences("sess_stream", "active")).toEqual({
        detail: 30_000,
        byId: 30_000,
        inputs: 30_000,
        controlPollInputs: 1_000,
        usage: 30_000,
        usageTurns: 30_000,
      });
      expect(cadences("sess_stream", "starting").detail).toBe(500);
      expect(
        Object.values(cadences("sess_stream", "stopped")).every(value => value === false)
      ).toBe(true);
      expect(cadences("sess_other", "active")).toEqual(fallback);
      release();
      release();
      expect(cadences("sess_stream", "active").detail).toBe(30_000);
    } finally {
      release();
      second();
    }
    expect(cadences("sess_stream", "active")).toEqual(fallback);
  });
});

// Invariant: each wake rereads its active read models once, invalidates inactive siblings,
// and preserves all other workspace/session data. A session's row also lives in the All projects
// aggregate, so every catalog wake reaches it too. Owner: canonical Query cache boundaries.
describe("session read invalidation", () => {
  const keys = {
    detail: sessionKeys.detail("ws", "session"),
    history: sessionKeys.history("ws", "session"),
    inputs: sessionKeys.inputQueue("ws", "session"),
    usage: sessionKeys.usage("ws", "session"),
    usageTurns: sessionKeys.usageTurns("ws", "session"),
    transcript: sessionKeys.transcript("ws", "session"),
    goal: sessionKeys.goal("ws", "session"),
    profile: sessionKeys.byId("session", "profile"),
    otherProfile: sessionKeys.byId("session", "other-profile"),
    catalog: sessionKeys.list({ workspace_id: "ws" }),
    folders: sessionKeys.folders("ws", "profile"),
    aggregate: sessionKeys.list(),
    otherCatalog: sessionKeys.list({ workspace_id: "other-ws" }),
    otherSession: sessionKeys.detail("ws", "other-session"),
    otherWorkspace: sessionKeys.detail("other-ws", "session"),
    otherUsage: sessionKeys.usage("ws", "other-session"),
    otherOwner: sessionKeys.byId("other-session", "profile"),
    usageDescendant: [...sessionKeys.usage("ws", "session"), "descendant"],
  };
  for (const testCase of [
    {
      name: "live",
      invalidate: invalidateSessionLiveQueries,
      affected: [
        "detail",
        "history",
        "inputs",
        "profile",
        "otherProfile",
        "catalog",
        "folders",
        "aggregate",
      ],
    },
    {
      name: "catalog",
      invalidate: invalidateWorkspaceSessionCatalog,
      affected: ["catalog", "folders", "aggregate"],
    },
    {
      name: "mutation",
      invalidate: invalidateSessionMutationQueries,
      affected: [
        "detail",
        "history",
        "inputs",
        "usage",
        "usageTurns",
        "transcript",
        "goal",
        "profile",
        "otherProfile",
        "catalog",
        "folders",
        "aggregate",
        "usageDescendant",
      ],
    },
    { name: "usage", invalidate: invalidateSessionUsageQueries, affected: ["usage", "usageTurns"] },
  ]) {
    it(`Should preserve the ${testCase.name} wake scope for active and inactive reads`, async () => {
      const client = new QueryClient({
        defaultOptions: { queries: { staleTime: Infinity, retry: false } },
      });
      const unsubscribers: (() => void)[] = [];
      const reads = Object.entries(keys).map(([name, queryKey], index) => {
        client.setQueryData(queryKey, "before");
        const queryFn = vi.fn(async () => "after");
        const active = index % 2 === 0;
        if (active) {
          const observer = new QueryObserver(client, { queryKey, queryFn });
          unsubscribers.push(observer.subscribe(() => undefined));
        }
        return { name, queryKey, queryFn, active };
      });
      try {
        await testCase.invalidate(client, "ws", "session");
        for (const { name, queryKey, queryFn, active } of reads) {
          const affected = testCase.affected.includes(name);
          expect(queryFn, name).toHaveBeenCalledTimes(active && affected ? 1 : 0);
          expect(client.getQueryData(queryKey), name).toBe(active && affected ? "after" : "before");
          expect(client.getQueryState(queryKey)?.isInvalidated, name).toBe(!active && affected);
        }
      } finally {
        for (const unsubscribe of unsubscribers) unsubscribe();
        client.clear();
      }
    });
  }
});

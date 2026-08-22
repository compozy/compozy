// Suite: attention delivery engine
// Invariant: needs-you transitions deliver immediately and per session with a
// 5s dedup, completions coalesce into one grouped delivery whose window never
// self-extends, the focused session and muted workspaces deliver nothing while
// their counts remain, decisions read the state at delivery time, and one batch
// is one delivery call (so one sound).
// Owning layer: unit (systems/os/lib) — pure policy with injected clock/timers.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionAttentionEventPayload } from "@/systems/session";

import {
  COMPLETION_COALESCE_MS,
  createAttentionDeliveryEngine,
  NEEDS_YOU_DEDUP_MS,
  RESOLVE_GRACE_MS,
  type AttentionDelivery,
  type AttentionDeliveryContext,
  type AttentionDeliveryEngine,
  type AttentionDeliveryTarget,
} from "../attention-delivery";

function target(overrides: Partial<AttentionDeliveryTarget> = {}): AttentionDeliveryTarget {
  return {
    id: "sess-1",
    title: "Refactor session store",
    agentName: "claude",
    workspaceId: "ws-compozy",
    workspaceLabel: "compozy",
    badge: "waiting-for-input",
    reason: "Should I drop the legacy column?",
    ...overrides,
  };
}

function edge(overrides: Partial<SessionAttentionEventPayload> = {}): SessionAttentionEventPayload {
  return {
    profile_name: "default",
    profile_id: "00000000000000000000000000",
    session_id: "sess-1",
    workspace_id: "ws-compozy",
    from: "running",
    to: "waiting-for-input",
    class: "needs-you",
    at: "2026-07-20T12:00:00Z",
    ...overrides,
  };
}

interface Harness {
  engine: AttentionDeliveryEngine;
  batches: AttentionDelivery[][];
  setTargets: (targets: AttentionDeliveryTarget[]) => void;
  setContext: (patch: Partial<AttentionDeliveryContext>) => void;
}

function harness(initial: AttentionDeliveryTarget[] = [target()]): Harness {
  const batches: AttentionDelivery[][] = [];
  let targets = new Map(initial.map(entry => [entry.id, entry]));
  let context: AttentionDeliveryContext = {
    mutedWorkspaceIds: new Set<string>(),
    focusedSessionId: null,
    appActive: true,
    resolve: id => targets.get(id) ?? null,
  };
  const engine = createAttentionDeliveryEngine({
    now: () => Date.now(),
    getContext: () => context,
    deliver: batch => batches.push(batch),
  });
  return {
    engine,
    batches,
    setTargets: next => {
      targets = new Map(next.map(entry => [entry.id, entry]));
    },
    setContext: patch => {
      context = { ...context, ...patch };
    },
  };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-07-20T12:00:00Z"));
});

afterEach(() => {
  vi.useRealTimers();
});

describe("needs-you delivery (UT-055)", () => {
  it("Should deliver five simultaneous transitions as five toasts in one batch", () => {
    // Five distinct blockers need five distinct actions, so none may be grouped
    // away — but one batch means one sound, not a chorus.
    const targets = ["a", "b", "c", "d", "e"].map(id => target({ id: `sess-${id}` }));
    const world = harness(targets);
    for (const entry of targets) world.engine.ingestEdge(edge({ session_id: entry.id }));

    vi.advanceTimersByTime(1);

    expect(world.batches).toHaveLength(1);
    expect(world.batches[0]).toHaveLength(5);
    expect(world.batches[0]?.every(delivery => delivery.kind === "needs-you")).toBe(true);
  });

  it("Should absorb the same session flapping inside the dedup window", () => {
    const world = harness();
    world.engine.ingestEdge(edge());
    vi.advanceTimersByTime(1);
    world.setTargets([target({ badge: "waiting-for-auth", reason: "Permission required" })]);
    world.engine.ingestEdge(
      edge({
        from: "running",
        to: "waiting-for-auth",
      })
    );
    vi.advanceTimersByTime(NEEDS_YOU_DEDUP_MS - 100);

    expect(world.batches).toHaveLength(1);
  });

  it("Should notify again once the dedup window has passed", () => {
    const world = harness();
    world.engine.ingestEdge(edge());
    vi.advanceTimersByTime(NEEDS_YOU_DEDUP_MS + 1);
    world.engine.ingestEdge(edge());
    vi.advanceTimersByTime(1);

    expect(world.batches).toHaveLength(2);
  });

  it("Should wait for the refetch that names the session, then drop what never arrives", () => {
    const world = harness([]);
    world.engine.ingestEdge(edge());
    vi.advanceTimersByTime(1);
    expect(world.batches).toHaveLength(0);

    // The catalog wake lands and the session becomes describable.
    world.setTargets([target()]);
    world.engine.refresh();
    vi.advanceTimersByTime(1);
    expect(world.batches).toHaveLength(1);

    // A second edge that never resolves is dropped rather than rendered blank.
    world.setTargets([]);
    world.engine.ingestEdge(edge({ session_id: "sess-ghost" }));
    vi.advanceTimersByTime(RESOLVE_GRACE_MS + 500);
    expect(world.batches).toHaveLength(1);
  });
});

describe("focus suppression (UT-057)", () => {
  it("Should stay silent for the session already on screen", () => {
    const world = harness();
    world.setContext({ focusedSessionId: "sess-1", appActive: true });

    world.engine.ingestEdge(edge());
    vi.advanceTimersByTime(1);

    expect(world.batches).toHaveLength(0);
  });

  it("Should notify a focused session when the app itself is hidden", () => {
    // The window holds focus, but the operator is looking at something else.
    const world = harness();
    world.setContext({ focusedSessionId: "sess-1", appActive: false });

    world.engine.ingestEdge(edge());
    vi.advanceTimersByTime(1);

    expect(world.batches).toHaveLength(1);
  });
});

describe("workspace mute (UT-071)", () => {
  it("Should deliver nothing for a muted workspace", () => {
    const world = harness();
    world.setContext({ mutedWorkspaceIds: new Set(["ws-compozy"]) });

    world.engine.ingestEdge(edge());
    vi.advanceTimersByTime(1);

    expect(world.batches).toHaveLength(0);
  });

  it("Should drop a workspace muted mid-window from the pending group", () => {
    const world = harness([
      target({ id: "sess-a", badge: "done" }),
      target({ id: "sess-b", badge: "done", workspaceId: "ws-infra", workspaceLabel: "infra" }),
    ]);
    world.engine.ingestEdge(edge({ session_id: "sess-a", to: "done", class: "finished" }));
    world.engine.ingestEdge(
      edge({ session_id: "sess-b", workspace_id: "ws-infra", to: "done", class: "finished" })
    );

    // Muting after the window opened must change what the window delivers.
    world.setContext({ mutedWorkspaceIds: new Set(["ws-infra"]) });
    vi.advanceTimersByTime(COMPLETION_COALESCE_MS + 1);

    const delivery = world.batches[0]?.[0];
    expect(delivery?.kind).toBe("finished");
    expect(delivery?.kind === "finished" ? delivery.targets.map(entry => entry.id) : []).toEqual([
      "sess-a",
    ]);
  });
});

describe("completion coalescing (UT-056)", () => {
  function finishedEdge(id: string): SessionAttentionEventPayload {
    return edge({ session_id: id, to: "done", class: "finished" });
  }

  it("Should group completions inside one window into a single delivery", () => {
    const done = ["a", "b", "c"].map(id => target({ id: `sess-${id}`, badge: "done" }));
    const world = harness(done);
    world.engine.ingestEdge(finishedEdge("sess-a"));
    vi.advanceTimersByTime(1_000);
    world.engine.ingestEdge(finishedEdge("sess-b"));
    vi.advanceTimersByTime(1_000);
    world.engine.ingestEdge(finishedEdge("sess-c"));
    vi.advanceTimersByTime(COMPLETION_COALESCE_MS);

    expect(world.batches).toHaveLength(1);
    const delivery = world.batches[0]?.[0];
    expect(delivery?.kind === "finished" ? delivery.targets : []).toHaveLength(3);
  });

  it("Should never extend the window — later completions start a new one", () => {
    // Trickling completions must not starve the signal by pushing the close out.
    const done = [target({ id: "sess-a", badge: "done" }), target({ id: "sess-b", badge: "done" })];
    const world = harness(done);
    world.engine.ingestEdge(finishedEdge("sess-a"));
    vi.advanceTimersByTime(COMPLETION_COALESCE_MS + 1);
    world.engine.ingestEdge(finishedEdge("sess-b"));
    vi.advanceTimersByTime(COMPLETION_COALESCE_MS + 1);

    expect(world.batches).toHaveLength(2);
  });

  it("Should collapse a group of one to the single form", () => {
    const world = harness([target({ id: "sess-a", badge: "done" })]);
    world.engine.ingestEdge(finishedEdge("sess-a"));
    vi.advanceTimersByTime(COMPLETION_COALESCE_MS + 1);

    const delivery = world.batches[0]?.[0];
    expect(delivery?.kind === "finished" ? delivery.targets : []).toHaveLength(1);
  });

  it("Should fire nothing when everything was seen before the window closed", () => {
    // Each session cleared its own marker: the badge is no longer `done`, so
    // there is nothing left to announce.
    const world = harness([target({ id: "sess-a", badge: "idle" })]);
    world.engine.ingestEdge(finishedEdge("sess-a"));
    vi.advanceTimersByTime(COMPLETION_COALESCE_MS + 1);

    expect(world.batches).toHaveLength(0);
  });
});

describe("mixed traffic (UT-058)", () => {
  it("Should deliver needs-you individually while completions coalesce separately", () => {
    const world = harness([
      target({ id: "sess-blocked" }),
      target({ id: "sess-done-a", badge: "done" }),
      target({ id: "sess-done-b", badge: "done" }),
    ]);
    world.engine.ingestEdge(edge({ session_id: "sess-blocked" }));
    world.engine.ingestEdge(edge({ session_id: "sess-done-a", to: "done", class: "finished" }));
    world.engine.ingestEdge(edge({ session_id: "sess-done-b", to: "done", class: "finished" }));

    vi.advanceTimersByTime(1);
    // The urgent signal is never held back waiting on a coalescing window.
    expect(world.batches).toHaveLength(1);
    expect(world.batches[0]?.[0]?.kind).toBe("needs-you");

    vi.advanceTimersByTime(COMPLETION_COALESCE_MS + 1);
    expect(world.batches).toHaveLength(2);
    const grouped = world.batches[1]?.[0];
    expect(grouped?.kind === "finished" ? grouped.targets : []).toHaveLength(2);
  });
});

describe("stream lifecycle (UT-067)", () => {
  it("Should fire nothing after disposal, so a superseded stream cannot replay", () => {
    const world = harness();
    world.engine.dispose();

    world.engine.ingestEdge(edge());
    world.engine.ingestEdge(edge({ session_id: "sess-2", to: "done", class: "finished" }));
    vi.advanceTimersByTime(COMPLETION_COALESCE_MS + 1);

    expect(world.batches).toHaveLength(0);
  });

  it("Should ignore transitions that leave the attention classes entirely", () => {
    const world = harness();
    world.engine.ingestEdge(edge({ from: "waiting-for-input", to: "idle", class: "none" }));
    vi.advanceTimersByTime(COMPLETION_COALESCE_MS + 1);

    expect(world.batches).toHaveLength(0);
  });
});

describe("agent notifications (UT-083)", () => {
  const notification = {
    notification_id: "ntf-1",
    session_id: "sess-1",
    workspace_id: "ws-compozy",
    title: "Deps audit done",
    body: "3 findings, 1 high severity",
    at: "2026-07-20T12:00:00Z",
    profile_id: "00000000000000000000000000",
    profile_name: "default",
  };

  it("Should deliver an agent notification with its sending session resolved", () => {
    const world = harness();
    world.engine.ingestNotification(notification);
    vi.advanceTimersByTime(1);

    const delivery = world.batches[0]?.[0];
    expect(delivery?.kind).toBe("agent");
    expect(delivery?.kind === "agent" ? delivery.target?.id : null).toBe("sess-1");
  });

  it("Should apply the same focus and mute rules as any other delivery", () => {
    const world = harness();
    world.setContext({ focusedSessionId: "sess-1" });
    world.engine.ingestNotification(notification);
    vi.advanceTimersByTime(1);
    expect(world.batches).toHaveLength(0);

    world.setContext({ focusedSessionId: null, mutedWorkspaceIds: new Set(["ws-compozy"]) });
    world.engine.ingestNotification({
      ...notification,
      notification_id: "ntf-2",
    });
    vi.advanceTimersByTime(1);
    expect(world.batches).toHaveLength(0);
  });
});

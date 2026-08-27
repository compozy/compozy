import { describe, expect, it } from "vitest";

import type { AgentCommsScope } from "../agent-comms-scope";
import { agentCommsKeys } from "../query-keys";
import {
  CALLS_TREE_PAGE_SIZE,
  CALLS_PANEL_PAGE_SIZE,
  callDetailOptions,
  callCountOptions,
  callMessagesOptions,
  callPromptOptions,
  callResultOptions,
  callSupersededOptions,
  callsListOptions,
} from "../query-options";

const SCOPE: AgentCommsScope = {
  workspaceId: "ws_main",
  profileKey: "default",
  params: { profile: "default" },
  actingProfile: "default",
};

describe("agentCommsKeys", () => {
  it("Should nest every population under its workspace and profile", () => {
    expect(agentCommsKeys.callsList("ws_main", "default", { state: "running" })).toEqual([
      "agent-comms",
      "workspace",
      "ws_main",
      "profile",
      "default",
      "calls",
      "list",
      "running",
      false,
      "",
      "",
      "",
      "",
      0,
    ]);
  });

  it("Should keep the attention population apart from any state population", () => {
    // `attention=true` is the daemon's *unresolved* subset; the raw state filter
    // is permanent. Sharing a cache entry would show a resolved call as pending.
    expect(agentCommsKeys.callCount("ws_main", "default", { attention: true })).not.toEqual(
      agentCommsKeys.callCount("ws_main", "default", { state: "invalid-result" })
    );
    expect(agentCommsKeys.callCount("ws_main", "default", { attention: true })).not.toEqual(
      agentCommsKeys.callCount("ws_main", "default", {})
    );
    // Absent and explicitly false are the same population.
    expect(agentCommsKeys.callCount("ws_main", "default", { attention: false })).toEqual(
      agentCommsKeys.callCount("ws_main", "default", {})
    );
  });

  it("Should keep a count probe on a different entry than the rows it counts", () => {
    const rows = agentCommsKeys.callsList("ws_main", "default", { caller: "ses_a" });
    const count = agentCommsKeys.callCount("ws_main", "default", { caller: "ses_a" });
    expect(count).not.toEqual(rows);
    // A badge refresh must not evict the page the operator is reading.
    expect(count.slice(0, 6)).toEqual(rows.slice(0, 6));
  });

  it("Should normalize blank and padded segments so one population has one key", () => {
    expect(agentCommsKeys.callCount("  ws_main  ", " default ", { caller: "  " })).toEqual(
      agentCommsKeys.callCount("ws_main", "default", {})
    );
  });

  it("Should separate the two directions, so Made never reads Received's cache", () => {
    expect(agentCommsKeys.callCount("ws_main", "default", { caller: "ses_x" })).not.toEqual(
      agentCommsKeys.callCount("ws_main", "default", { child_session_id: "ses_x" })
    );
  });

  it("Should separate profiles and workspaces at the key, not at the filter", () => {
    expect(agentCommsKeys.callsList("ws_main", "default")).not.toEqual(
      agentCommsKeys.callsList("ws_main", "@all")
    );
    expect(agentCommsKeys.callsList("ws_main", "default")).not.toEqual(
      agentCommsKeys.callsList("ws_other", "default")
    );
  });

  it("Should address one call by workspace, profile, and id", () => {
    // The daemon derives a call's scope from the route it was read through, so
    // the same id under another workspace is a different record. Caching it
    // without a workspace segment would serve one workspace's call to another.
    expect(agentCommsKeys.callDetail("ws_main", "default", "call_1")).toEqual([
      "agent-comms",
      "workspace",
      "ws_main",
      "profile",
      "default",
      "call",
      "detail",
      "call_1",
    ]);
    expect(agentCommsKeys.callDetail("ws_main", "default", "call_1")).not.toEqual(
      agentCommsKeys.callDetail("ws_other", "default", "call_1")
    );
  });

  it("Should give a call's four reads four entries", () => {
    const detail = agentCommsKeys.callDetail("ws_main", "default", "call_1");
    const attachments = [
      agentCommsKeys.callResult("ws_main", "default", "call_1"),
      agentCommsKeys.callPrompt("ws_main", "default", "call_1"),
      agentCommsKeys.callSuperseded("ws_main", "default", "call_1"),
    ];
    for (const key of attachments) {
      expect(key).not.toEqual(detail);
      // All four still hang off `callDetails`, so one invalidation clears them.
      expect(key.slice(0, 6)).toEqual(detail.slice(0, 6));
    }
    expect(new Set(attachments.map(key => key.join("/"))).size).toBe(attachments.length);
  });
});

describe("call query options", () => {
  it("Should page by cursor and keep continuation out of the key", () => {
    const options = callsListOptions(SCOPE, { root_session_id: "ses_root" }, false);
    expect(options.initialPageParam).toBeNull();
    expect(options.getNextPageParam({ items: [], total: 0, next_cursor: "c1" }, [], null, [])).toBe(
      "c1"
    );
    expect(options.getNextPageParam({ items: [], total: 0 }, [], null, [])).toBeUndefined();
    expect(options.queryKey).toEqual(
      agentCommsKeys.callsList("ws_main", "default", {
        root_session_id: "ses_root",
        limit: CALLS_TREE_PAGE_SIZE,
      })
    );
  });

  it("Should poll only while the window is live", () => {
    expect(callsListOptions(SCOPE, {}, true).refetchInterval).toBe(5_000);
    expect(callsListOptions(SCOPE, {}, false).refetchInterval).toBe(false);
    expect(callCountOptions(SCOPE, {}, true).refetchInterval).toBe(10_000);
    expect(callCountOptions(SCOPE, {}, false).refetchInterval).toBe(false);
  });

  it("Should poll a live detail until the call settles, and stay still when the window is not live", () => {
    const idle = callDetailOptions(SCOPE, "call_1", false).refetchInterval;
    expect(idle).toBeTypeOf("function");
    if (typeof idle !== "function") return;
    expect(idle({ state: { data: { state: "running" }, error: null } } as never)).toBe(false);
    expect(idle({ state: { data: { state: "completed" }, error: null } } as never)).toBe(false);

    const liveInterval = callDetailOptions(SCOPE, "call_1", true).refetchInterval;
    expect(liveInterval).toBeTypeOf("function");
    if (typeof liveInterval !== "function") return;
    expect(liveInterval({ state: { data: { state: "running" }, error: null } } as never)).toBe(
      5_000
    );
    expect(liveInterval({ state: { data: { state: "completed" }, error: null } } as never)).toBe(
      false
    );
    expect(liveInterval({ state: { data: undefined, error: new Error("offline") } } as never)).toBe(
      false
    );
  });

  it("Should refuse to fetch before the workspace resolves", () => {
    const pending: AgentCommsScope = { ...SCOPE, workspaceId: "" };
    expect(callsListOptions(pending, {}, true).enabled).toBe(false);
    expect(callCountOptions(pending, {}, true).enabled).toBe(false);
    expect(callMessagesOptions(pending, { session: "ses_1" }, false).enabled).toBe(false);
  });

  it("Should key and enable mailbox reads from the resolved scope", () => {
    const options = callMessagesOptions(SCOPE, { session: "ses_1" }, false);
    expect(options.enabled).toBe(true);
    expect(options.queryKey).toEqual(
      agentCommsKeys.messagesList("ws_main", "default", {
        session: "ses_1",
        limit: CALLS_PANEL_PAGE_SIZE,
      })
    );
  });

  it("Should leave every unbounded payload unfetched until the operator asks", () => {
    for (const build of [callResultOptions, callPromptOptions, callSupersededOptions]) {
      expect(build(SCOPE, "call_1").enabled).toBe(false);
      expect(build(SCOPE, "call_1", true).enabled).toBe(true);
      // A settled call's payloads never change, so one read is the only read.
      expect(build(SCOPE, "call_1", true).staleTime).toBe(Number.POSITIVE_INFINITY);
    }
  });
});

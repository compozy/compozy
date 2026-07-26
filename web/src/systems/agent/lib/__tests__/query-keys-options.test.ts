import { describe, expect, it } from "vitest";

import { agentKeys } from "../query-keys";
import {
  agentCatalogOptions,
  agentDetailOptions,
  agentHeartbeatHistoryOptions,
  agentHeartbeatOptions,
  agentHeartbeatStatusOptions,
  agentSoulHistoryOptions,
  agentSoulOptions,
  agentsListOptions,
} from "../query-options";

describe("agent query keys/options", () => {
  it("Should normalize catalog filters into a workspace-stable infinite key", () => {
    const options = agentCatalogOptions(" ws_alpha ", {
      q: "  release ",
      category: " Engineering / Release ",
      status: "active",
      limit: 50,
    });

    expect(options.queryKey).toEqual([
      "agents",
      "catalog",
      "ws_alpha",
      {
        q: "release",
        category: "Engineering / Release",
        status: "active",
        limit: 50,
      },
    ]);
    expect(options.initialPageParam).toBeUndefined();
    expect(options.staleTime).toBe(5_000);
  });

  it("Should nest soul/heartbeat keys under workspace-scoped detail", () => {
    expect(agentKeys.soul("coder", "ws_alpha")).toEqual([
      "agents",
      "detail",
      "coder",
      "ws_alpha",
      "soul",
    ]);
    expect(
      agentKeys.heartbeatStatus("coder", { workspaceId: "ws_alpha", sessionId: "sess-1" })
    ).toEqual([
      "agents",
      "detail",
      "coder",
      "ws_alpha",
      "heartbeat",
      "status",
      "sess-1",
      null,
      null,
    ]);
  });

  it("Should expose queryOptions factories with matching keys", () => {
    expect(agentsListOptions("ws_alpha").queryKey).toEqual(agentKeys.list("ws_alpha"));
    expect(agentDetailOptions("coder", "ws_alpha").queryKey).toEqual(
      agentKeys.detail("coder", "ws_alpha")
    );
    expect(agentSoulOptions("coder", "ws_alpha").queryKey).toEqual(
      agentKeys.soul("coder", "ws_alpha")
    );
    expect(agentSoulHistoryOptions("coder", "ws_alpha").queryKey).toEqual(
      agentKeys.soulHistory("coder", "ws_alpha")
    );
    expect(agentHeartbeatOptions("coder", null).queryKey).toEqual(
      agentKeys.heartbeat("coder", null)
    );
    expect(agentHeartbeatHistoryOptions("coder", "ws_alpha").queryKey).toEqual(
      agentKeys.heartbeatHistory("coder", "ws_alpha")
    );
    expect(
      agentHeartbeatStatusOptions("coder", { workspaceId: "ws_alpha", sessionId: "sess-1" })
        .queryKey
    ).toEqual(agentKeys.heartbeatStatus("coder", { workspaceId: "ws_alpha", sessionId: "sess-1" }));
  });

  it("Should isolate heartbeat status cache entries by response-shaping options", () => {
    const base = agentHeartbeatStatusOptions("coder", {
      workspaceId: "ws_alpha",
      sessionId: "sess-1",
    }).queryKey;
    const withSessionHealth = agentHeartbeatStatusOptions("coder", {
      workspaceId: "ws_alpha",
      sessionId: "sess-1",
      includeSessionHealth: true,
    }).queryKey;
    const withRecentWakeEvents = agentHeartbeatStatusOptions("coder", {
      workspaceId: "ws_alpha",
      sessionId: "sess-1",
      includeRecentWakeEvents: true,
    }).queryKey;

    expect(withSessionHealth).not.toEqual(base);
    expect(withRecentWakeEvents).not.toEqual(base);
    expect(withSessionHealth).not.toEqual(withRecentWakeEvents);
  });
});

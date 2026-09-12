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
        profile: "default",
        q: "release",
        category: "Engineering / Release",
        status: "active",
        limit: 50,
      },
    ]);
    expect(options.initialPageParam).toBeUndefined();
    expect(options.staleTime).toBe(5_000);
  });

  it.each(["default", "open-design"])(
    "Should nest soul/heartbeat keys under workspace and profile %s",
    profile => {
      expect(agentKeys.soul("coder", "ws_alpha", profile)).toEqual([
        "agents",
        "detail",
        "coder",
        "ws_alpha",
        profile,
        "soul",
      ]);
      expect(
        agentKeys.heartbeatStatus("coder", {
          workspaceId: "ws_alpha",
          sessionId: "sess-1",
          profile,
        })
      ).toEqual([
        "agents",
        "detail",
        "coder",
        "ws_alpha",
        profile,
        "heartbeat",
        "status",
        "sess-1",
        null,
        null,
      ]);
    }
  );

  it.each([undefined, "open-design"])(
    "Should expose queryOptions factories with matching keys for profile %s",
    profile => {
      expect(agentsListOptions("ws_alpha").queryKey).toEqual(agentKeys.list("ws_alpha"));
      expect(agentDetailOptions("coder", "ws_alpha").queryKey).toEqual(
        agentKeys.detail("coder", "ws_alpha")
      );
      expect(agentSoulOptions("coder", "ws_alpha", profile).queryKey).toEqual(
        agentKeys.soul("coder", "ws_alpha", profile)
      );
      expect(agentSoulHistoryOptions("coder", "ws_alpha", profile).queryKey).toEqual(
        agentKeys.soulHistory("coder", "ws_alpha", profile)
      );
      expect(agentHeartbeatOptions("coder", null, profile).queryKey).toEqual(
        agentKeys.heartbeat("coder", null, profile)
      );
      expect(agentHeartbeatHistoryOptions("coder", "ws_alpha", profile).queryKey).toEqual(
        agentKeys.heartbeatHistory("coder", "ws_alpha", profile)
      );
      expect(
        agentHeartbeatStatusOptions("coder", {
          workspaceId: "ws_alpha",
          sessionId: "sess-1",
          profile,
        }).queryKey
      ).toEqual(
        agentKeys.heartbeatStatus("coder", {
          workspaceId: "ws_alpha",
          sessionId: "sess-1",
          profile,
        })
      );
    }
  );

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

    const otherProfile = agentHeartbeatStatusOptions("coder", {
      workspaceId: "ws_alpha",
      sessionId: "sess-1",
      profile: "open-design",
    }).queryKey;

    expect(otherProfile).not.toEqual(base);
    expect(withSessionHealth).not.toEqual(base);
    expect(withRecentWakeEvents).not.toEqual(base);
    expect(withSessionHealth).not.toEqual(withRecentWakeEvents);
  });
});

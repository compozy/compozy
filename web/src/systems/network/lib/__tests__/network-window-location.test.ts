import { describe, expect, it } from "vitest";

import type { NetworkDirectRoomDetail, NetworkThreadDetail } from "../../types";
import {
  networkChannelLocation,
  networkThreadsLocation,
  networkWindowTrail,
  parseNetworkWindowLocation,
  resolveNetworkConversationLabel,
  type ParsedNetworkWindowLocation,
} from "../network-window-location";

function parsedLocation(
  overrides: Partial<ParsedNetworkWindowLocation> = {}
): ParsedNetworkWindowLocation {
  return {
    workspaceId: "ws_1",
    channel: "general",
    activeTab: "threads",
    activeThreadId: null,
    activeDirectId: null,
    view: undefined,
    ...overrides,
  };
}

const threadDetail: NetworkThreadDetail = {
  channel: "general",
  message_count: 3,
  open_work_count: 4,
  participant_count: 2,
  root_message_id: "msg_1",
  thread_id: "th_1",
  title: "Cut v0.16 release notes",
};

const directDetail: NetworkDirectRoomDetail = {
  channel: "general",
  direct_id: "dr_1",
  message_count: 2,
  open_work_count: 1,
  session_a: "session-remote",
  session_b: "session-self",
};

describe("parseNetworkWindowLocation", () => {
  it("Should parse channel tab locations and scope detail ids to their tab", () => {
    const threads = parseNetworkWindowLocation({
      pathname: "/network/ws_1/general/threads/th_1",
      search: { view: "full" },
    });
    expect(threads).toMatchObject({
      workspaceId: "ws_1",
      channel: "general",
      activeTab: "threads",
      activeThreadId: "th_1",
      activeDirectId: null,
      view: "full",
    });

    const directs = parseNetworkWindowLocation({
      pathname: "/network/ws_1/general/directs/dr_1",
      search: {},
    });
    expect(directs).toMatchObject({
      activeTab: "directs",
      activeDirectId: "dr_1",
      activeThreadId: null,
      view: undefined,
    });
  });

  it("Should fall back to the root location for unrecognized pathnames", () => {
    expect(parseNetworkWindowLocation({ pathname: "/network", search: {} })).toEqual({
      workspaceId: null,
      channel: null,
      activeTab: "threads",
      activeThreadId: null,
      activeDirectId: null,
      view: undefined,
    });
  });
});

describe("networkWindowTrail", () => {
  it("Should keep the root head while only a channel is selected", () => {
    expect(networkWindowTrail(parsedLocation())).toEqual({
      parents: [],
      leaf: "Network",
      drilledIn: false,
    });
  });

  it("Should drill into a conversation with the conversation label as leaf", () => {
    expect(
      networkWindowTrail(parsedLocation({ activeThreadId: "th_1" }), threadDetail.title)
    ).toEqual({
      parents: [
        {
          id: "root",
          label: "Network",
          location: { pathname: "/network", search: {} },
        },
        {
          id: "channel",
          label: "#general",
          location: { pathname: "/network/ws_1/general/threads", search: {} },
        },
      ],
      leaf: "Cut v0.16 release notes",
      drilledIn: true,
    });
  });

  it("Should fall back to generic conversation leaves when no detail label resolves", () => {
    expect(networkWindowTrail(parsedLocation({ activeThreadId: "th_1" })).leaf).toBe("Thread");
    expect(
      networkWindowTrail(parsedLocation({ activeTab: "directs", activeDirectId: "dr_1" }), null)
        .leaf
    ).toBe("Direct");
  });
});

describe("network locations", () => {
  it("Should preserve the selected tab when building a channel parent", () => {
    expect(networkChannelLocation("ws 1", "rel/coord", "directs")).toEqual({
      pathname: "/network/ws%201/rel%2Fcoord/directs",
      search: {},
    });
  });

  it("Should encode thread segments and carry the full view flag", () => {
    expect(networkThreadsLocation("ws 1", "rel/coord", "th 9", "full")).toEqual({
      pathname: "/network/ws%201/rel%2Fcoord/threads/th%209",
      search: { view: "full" },
    });
  });

  it("Should omit search when the thread view uses its default mode", () => {
    expect(networkThreadsLocation("ws_1", "general").search).toEqual({});
  });
});

describe("resolveNetworkConversationLabel", () => {
  it("Should resolve a thread leaf from its authoritative detail", () => {
    expect(
      resolveNetworkConversationLabel(parsedLocation({ activeThreadId: "th_1" }), {
        thread: threadDetail,
        direct: null,
        selfSessionId: null,
      })
    ).toBe("Cut v0.16 release notes");
  });

  it("Should resolve a direct leaf only when the local session is known", () => {
    const location = parsedLocation({ activeTab: "directs", activeDirectId: "dr_1" });
    expect(
      resolveNetworkConversationLabel(location, {
        thread: null,
        direct: directDetail,
        selfSessionId: "session-self",
      })
    ).toBe("@session-remote");
    expect(
      resolveNetworkConversationLabel(location, {
        thread: null,
        direct: directDetail,
        selfSessionId: null,
      })
    ).toBeNull();
  });

  it("Should reject detail from a different conversation", () => {
    expect(
      resolveNetworkConversationLabel(parsedLocation({ activeThreadId: "th_other" }), {
        thread: threadDetail,
        direct: null,
        selfSessionId: null,
      })
    ).toBeNull();
  });
});

import { describe, expect, it } from "vitest";

import { validateSessionDeepLinkSearch } from "../session-deeplink-search";

describe("validateSessionDeepLinkSearch", () => {
  it("Should resolve an empty search to no confirmation state", () => {
    expect(validateSessionDeepLinkSearch({})).toEqual({});
  });

  it("Should preserve replayable confirmation and decline states", () => {
    expect(validateSessionDeepLinkSearch({ workspaceSwitch: "confirm" })).toEqual({
      workspaceSwitch: "confirm",
    });
    expect(validateSessionDeepLinkSearch({ workspaceSwitch: "declined" })).toEqual({
      workspaceSwitch: "declined",
    });
  });

  it("Should drop untrusted values instead of manufacturing a confirmation payload", () => {
    expect(validateSessionDeepLinkSearch({ workspaceSwitch: "ws_06366aad69887872" })).toEqual({});
    expect(validateSessionDeepLinkSearch({ workspaceSwitch: true })).toEqual({});
    expect(
      validateSessionDeepLinkSearch({
        workspaceSwitch: { workspaceId: "ws_06366aad69887872", workspaceName: "primary" },
      })
    ).toEqual({});
  });

  it("Should discard unknown search keys so no extra state reaches the route", () => {
    expect(
      validateSessionDeepLinkSearch({
        workspaceSwitch: "confirm",
        workspaceName: "primary",
        workspaceId: "ws_06366aad69887872",
      })
    ).toEqual({ workspaceSwitch: "confirm" });
  });
});

// Suite: window-manager query keys
// Invariant: an absent client is a structural null in the config key, never a
// string that could collide with a real client id.
// Owning layer: unit — the query-key factory.
import { describe, expect, it } from "vitest";

import { windowManagerKeys } from "../window-manager-query";

describe("windowManagerKeys.config", () => {
  it("Should encode a missing client as null instead of a sentinel string", () => {
    expect(windowManagerKeys.config()).toEqual([
      "settings",
      "section",
      "window-manager",
      "global",
      null,
    ]);
    expect(windowManagerKeys.config("ws-a", "  ")).toEqual([
      "settings",
      "section",
      "window-manager",
      "ws-a",
      null,
    ]);
    expect(windowManagerKeys.config("ws-a", "client:web")).toEqual([
      "settings",
      "section",
      "window-manager",
      "ws-a",
      "client:web",
    ]);
  });
});

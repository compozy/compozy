import { describe, expect, it } from "vitest";

import { skillKeys } from "../query-keys";

describe("skillKeys", () => {
  it("has hierarchically structured keys", () => {
    expect(skillKeys.all).toEqual(["computer", "local", "skills"]);
    expect(skillKeys.list("ws_123")).toEqual(["computer", "local", "skills", "list", "ws_123", ""]);
    expect(skillKeys.detail("my-skill", "ws_123")).toEqual([
      "computer",
      "local",
      "skills",
      "detail",
      "my-skill",
      "ws_123",
      "",
    ]);
    expect(skillKeys.shadows("my-skill", "ws_123")).toEqual([
      "computer",
      "local",
      "skills",
      "shadows",
      "my-skill",
      "ws_123",
      "",
    ]);
  });

  it("keeps profile projections in separate cache entries", () => {
    expect(skillKeys.list("ws_123", "research")).not.toEqual(
      skillKeys.list("ws_123", "operations")
    );
    expect(skillKeys.detail("my-skill", "ws_123", "research")).toEqual([
      "computer",
      "local",
      "skills",
      "detail",
      "my-skill",
      "ws_123",
      "research",
    ]);
  });
});

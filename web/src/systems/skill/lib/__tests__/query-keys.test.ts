import { describe, expect, it } from "vitest";

import { skillKeys } from "../query-keys";

describe("skillKeys", () => {
  it("has hierarchically structured keys", () => {
    expect(skillKeys.all).toEqual(["skills"]);
    expect(skillKeys.list("ws_123")).toEqual(["skills", "list", "ws_123", ""]);
    expect(skillKeys.detail("my-skill", "ws_123")).toEqual([
      "skills",
      "detail",
      "my-skill",
      "ws_123",
      "",
    ]);
    expect(skillKeys.shadows("my-skill", "ws_123")).toEqual([
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
      "skills",
      "detail",
      "my-skill",
      "ws_123",
      "research",
    ]);
  });

  it("list keys extend all keys", () => {
    const list = skillKeys.list("ws_123");
    expect(list[0]).toBe(skillKeys.all[0]);
  });

  it("detail keys extend all keys", () => {
    const detail = skillKeys.detail("my-skill", "ws_123");
    expect(detail[0]).toBe(skillKeys.all[0]);
  });
});

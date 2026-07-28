import { describe, expect, it } from "vitest";

import { bridgeKeys } from "../query-keys";

describe("bridgeKeys", () => {
  it("creates stable list and providers keys", () => {
    expect(bridgeKeys.list()).toEqual(["bridges", "list", "all", "", "", "", "", "", "", ""]);
    expect(
      bridgeKeys.list({
        scope: "all",
        workspace_id: "ws_alpha",
        q: " support ",
        platform: "slack",
        status: "ready",
        sort: "name",
        limit: 25,
      })
    ).toEqual([
      "bridges",
      "list",
      "all",
      "ws_alpha",
      "",
      "support",
      "slack",
      "ready",
      "name",
      "25",
    ]);
    expect(bridgeKeys.providers()).toEqual(["bridges", "providers"]);
  });

  it("normalizes omitted detail and routes ids to empty strings", () => {
    expect(bridgeKeys.detail("")).toEqual(["bridges", "detail", ""]);
    expect(bridgeKeys.routes("")).toEqual(["bridges", "routes", ""]);
    expect(bridgeKeys.targets("")).toEqual(["bridges", "targets", "", "", ""]);
    expect(bridgeKeys.secretBindings("")).toEqual(["bridges", "secret-bindings", ""]);
    expect(bridgeKeys.slackManifest("   ")).toEqual(["bridges", "manifest", "slack", ""]);
  });

  it("includes bridge ids in detail and route query keys", () => {
    expect(bridgeKeys.detail("brg_support")).toEqual(["bridges", "detail", "brg_support"]);
    expect(bridgeKeys.routes("brg_support")).toEqual(["bridges", "routes", "brg_support"]);
    expect(bridgeKeys.targets("brg_support", { limit: 25, q: "launch" })).toEqual([
      "bridges",
      "targets",
      "brg_support",
      "launch",
      "25",
    ]);
    expect(bridgeKeys.targetsForBridge("brg_support")).toEqual([
      "bridges",
      "targets",
      "brg_support",
    ]);
    expect(bridgeKeys.secretBindings("brg_support")).toEqual([
      "bridges",
      "secret-bindings",
      "brg_support",
    ]);
    expect(bridgeKeys.slackManifest(" brg_support ")).toEqual([
      "bridges",
      "manifest",
      "slack",
      "brg_support",
    ]);
  });
});

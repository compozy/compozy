import { describe, expect, it } from "vitest";

import { bridgeKeys } from "../query-keys";

describe("bridgeKeys", () => {
  it("creates stable list and providers keys", () => {
    expect(bridgeKeys.list()).toEqual(["bridges", "list", "all", "", "", "", "", "", "", "", ""]);
    expect(
      bridgeKeys.list({
        scope: "all",
        workspace_id: "ws_alpha",
        q: " support ",
        platform: "slack",
        profile: "marketing",
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
      "marketing",
    ]);
    expect(bridgeKeys.providers()).toEqual(["bridges", "providers"]);
  });

  it("Should read a different entry per lens rather than inheriting the last one", () => {
    const scoped = bridgeKeys.list({ scope: "all", profile: "marketing" });
    const other = bridgeKeys.list({ scope: "all", profile: "default" });
    const aggregate = bridgeKeys.list({ scope: "all", all_profiles: true });

    // Same filters, three lenses, three cache entries: a switch cannot serve one
    // profile's bridges as another's.
    expect(scoped).not.toEqual(other);
    expect(scoped).not.toEqual(aggregate);
    expect(aggregate.at(-1)).toBe("@all");

    expect(bridgeKeys.detail("brg_support", { profile: "marketing" })).not.toEqual(
      bridgeKeys.detail("brg_support", { all_profiles: true })
    );
    expect(bridgeKeys.routes("brg_support", { profile: "marketing" })).not.toEqual(
      bridgeKeys.routes("brg_support", { all_profiles: true })
    );
  });

  it("Should let a mutation invalidate every lens of one bridge", () => {
    // The prefix a mutation uses: the bridge changed, so each lens holding it
    // must reread, not only the active one.
    const prefix = bridgeKeys.detailFor("brg_support");
    for (const scope of [{ profile: "default" }, { all_profiles: true } as const]) {
      expect(bridgeKeys.detail("brg_support", scope).slice(0, prefix.length)).toEqual([...prefix]);
      expect(bridgeKeys.routes("brg_support", scope).slice(0, 3)).toEqual([
        ...bridgeKeys.routesFor("brg_support"),
      ]);
    }
  });

  it("uses the exact empty identity for omitted detail and route reads", () => {
    expect(bridgeKeys.detail("", { profile: "default" })).toEqual([
      "bridges",
      "detail",
      "",
      "default",
    ]);
    expect(bridgeKeys.routes("", { profile: "default" })).toEqual([
      "bridges",
      "routes",
      "",
      "default",
    ]);
    expect(bridgeKeys.targets("")).toEqual(["bridges", "targets", "", "", "", ""]);
    expect(bridgeKeys.secretBindings("", { profile: "default" })).toEqual([
      "bridges",
      "secret-bindings",
      "",
      "default",
    ]);
    expect(bridgeKeys.slackManifest("", { profile: "default" })).toEqual([
      "bridges",
      "manifest",
      "slack",
      "",
      "default",
    ]);
  });

  it("includes bridge ids in detail and route query keys", () => {
    expect(bridgeKeys.detail("brg_support", { profile: "default" })).toEqual([
      "bridges",
      "detail",
      "brg_support",
      "default",
    ]);
    expect(bridgeKeys.routes("brg_support", { profile: "default" })).toEqual([
      "bridges",
      "routes",
      "brg_support",
      "default",
    ]);
    expect(
      bridgeKeys.targets("brg_support", { all_profiles: true, limit: 25, q: "launch" })
    ).toEqual(["bridges", "targets", "brg_support", "launch", "25", "@all"]);
    expect(bridgeKeys.targetsForBridge("brg_support")).toEqual([
      "bridges",
      "targets",
      "brg_support",
    ]);
    expect(bridgeKeys.secretBindings("brg_support", { all_profiles: true })).toEqual([
      "bridges",
      "secret-bindings",
      "brg_support",
      "@all",
    ]);
    expect(bridgeKeys.slackManifest(" brg_support ", { profile: "default" })).toEqual([
      "bridges",
      "manifest",
      "slack",
      " brg_support ",
      "default",
    ]);
    expect(bridgeKeys.slackManifest("   ", { profile: "default" })).not.toEqual(
      bridgeKeys.slackManifest("", { profile: "default" })
    );
  });
});

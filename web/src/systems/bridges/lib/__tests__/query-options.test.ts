import { describe, expect, it } from "vitest";

import {
  bridgeDetailOptions,
  bridgeProvidersOptions,
  bridgeRoutesOptions,
  bridgeSecretBindingsOptions,
  bridgeTargetsOptions,
  bridgesListOptions,
  slackBridgeManifestOptions,
} from "@/systems/bridges/lib/query-options";

describe("bridgesListOptions", () => {
  it("uses a stable filtered key and cursor-only page parameter", () => {
    const options = bridgesListOptions({
      limit: 25,
      platform: "slack",
      profile: "default",
      q: "support",
      scope: "all",
      sort: "name",
      status: "ready",
      workspace_id: "ws_alpha",
    });

    expect(options.queryKey).toEqual([
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
      "default",
    ]);
    expect(options.initialPageParam).toBeUndefined();
    expect(
      options.getNextPageParam?.(
        { page: { has_more: true, next_cursor: "next" } } as never,
        [],
        undefined,
        []
      )
    ).toBe("next");
    expect(options.staleTime).toBe(15_000);
    expect(options.refetchInterval).toBeUndefined();
  });
});

describe("bridgeProvidersOptions", () => {
  it("uses the providers key and slower refetch cadence", () => {
    const options = bridgeProvidersOptions();

    expect(options.queryKey).toEqual(["bridges", "providers"]);
    expect(options.refetchInterval).toBe(60_000);
  });
});

describe("slackBridgeManifestOptions", () => {
  it("preserves the persisted instance id and disables only empty requests", () => {
    const enabledOptions = slackBridgeManifestOptions(" brg_slack ", { all_profiles: true });
    const whitespaceOptions = slackBridgeManifestOptions("   ", { profile: "default" });
    const disabledOptions = slackBridgeManifestOptions("", { profile: "default" });

    expect(enabledOptions.queryKey).toEqual([
      "bridges",
      "manifest",
      "slack",
      " brg_slack ",
      "@all",
    ]);
    expect(enabledOptions.enabled).toBe(true);
    expect(enabledOptions.staleTime).toBe(15_000);
    expect(whitespaceOptions.queryKey).toEqual(["bridges", "manifest", "slack", "   ", "default"]);
    expect(whitespaceOptions.enabled).toBe(true);
    expect(disabledOptions.queryKey).toEqual(["bridges", "manifest", "slack", "", "default"]);
    expect(disabledOptions.enabled).toBe(false);
  });
});

describe("bridgeDetailOptions", () => {
  it("is disabled when the bridge id is missing", () => {
    const options = bridgeDetailOptions("", { profile: "default" });

    expect(options.queryKey).toEqual(["bridges", "detail", "", "default"]);
    expect(options.enabled).toBe(false);
  });

  it("is enabled for real bridge ids", () => {
    const options = bridgeDetailOptions("brg_support", { profile: "default" });

    expect(options.enabled).toBe(true);
  });
});

describe("bridgeRoutesOptions", () => {
  it("uses the expected routes key and is gated by id", () => {
    const options = bridgeRoutesOptions("brg_support", { profile: "default" });

    expect(options.queryKey).toEqual(["bridges", "routes", "brg_support", "default"]);
    expect(options.enabled).toBe(true);
  });
});

describe("bridgeTargetsOptions", () => {
  it("uses the target directory key and is gated by id", () => {
    const options = bridgeTargetsOptions("brg_support", {
      limit: 50,
      q: "support",
      profile: "default",
    });

    expect(options.queryKey).toEqual([
      "bridges",
      "targets",
      "brg_support",
      "support",
      "50",
      "default",
    ]);
    expect(options.enabled).toBe(true);
    expect(options.refetchInterval).toBe(30_000);
  });
});

describe("bridgeSecretBindingsOptions", () => {
  it("uses the secret bindings key and is gated by id", () => {
    const options = bridgeSecretBindingsOptions("brg_support", { all_profiles: true });

    expect(options.queryKey).toEqual(["bridges", "secret-bindings", "brg_support", "@all"]);
    expect(options.enabled).toBe(true);
    expect(options.refetchInterval).toBe(30_000);
  });
});

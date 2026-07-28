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
  it("normalizes the persisted instance id and disables blank requests", () => {
    const enabledOptions = slackBridgeManifestOptions(" brg_slack ");
    const disabledOptions = slackBridgeManifestOptions("   ");

    expect(enabledOptions.queryKey).toEqual(["bridges", "manifest", "slack", "brg_slack"]);
    expect(enabledOptions.enabled).toBe(true);
    expect(enabledOptions.staleTime).toBe(15_000);
    expect(disabledOptions.queryKey).toEqual(["bridges", "manifest", "slack", ""]);
    expect(disabledOptions.enabled).toBe(false);
  });
});

describe("bridgeDetailOptions", () => {
  it("is disabled when the bridge id is missing", () => {
    const options = bridgeDetailOptions("");

    expect(options.queryKey).toEqual(["bridges", "detail", ""]);
    expect(options.enabled).toBe(false);
  });

  it("is enabled for real bridge ids", () => {
    const options = bridgeDetailOptions("brg_support");

    expect(options.enabled).toBe(true);
  });
});

describe("bridgeRoutesOptions", () => {
  it("uses the expected routes key and is gated by id", () => {
    const options = bridgeRoutesOptions("brg_support");

    expect(options.queryKey).toEqual(["bridges", "routes", "brg_support"]);
    expect(options.enabled).toBe(true);
  });
});

describe("bridgeTargetsOptions", () => {
  it("uses the target directory key and is gated by id", () => {
    const options = bridgeTargetsOptions("brg_support", { limit: 50, q: "support" });

    expect(options.queryKey).toEqual(["bridges", "targets", "brg_support", "support", "50"]);
    expect(options.enabled).toBe(true);
    expect(options.refetchInterval).toBe(30_000);
  });
});

describe("bridgeSecretBindingsOptions", () => {
  it("uses the secret bindings key and is gated by id", () => {
    const options = bridgeSecretBindingsOptions("brg_support");

    expect(options.queryKey).toEqual(["bridges", "secret-bindings", "brg_support"]);
    expect(options.enabled).toBe(true);
    expect(options.refetchInterval).toBe(30_000);
  });
});

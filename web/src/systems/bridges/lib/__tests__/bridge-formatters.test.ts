import { describe, expect, it, vi } from "vitest";

import {
  bridgeScopeTone,
  bridgeStatusTone,
  compactBridgeDeliveryDefaults,
  describeBridgeDeliveryDefaults,
  describeBridgeDmPolicy,
  describeBridgeProviderConfigSchema,
  describeBridgeRouteTarget,
  describeBridgeSecretSlot,
  describeBridgeTestTarget,
  formatBridgeDateTime,
  formatBridgeProviderConfig,
  formatBridgeRelativeTime,
  normalizeBridgeDeliveryDefaults,
} from "../bridge-formatters";

describe("bridge-formatters", () => {
  it("normalizes and compacts delivery defaults", () => {
    expect(
      normalizeBridgeDeliveryDefaults({
        group_id: "  group_123  ",
        mode: "reply",
        parse_mode: " MarkdownV2 ",
        peer_id: " peer_123 ",
        thread_id: "",
      })
    ).toEqual({
      group_id: "group_123",
      mode: "reply",
      parse_mode: " MarkdownV2 ",
      peer_id: "peer_123",
      thread_id: undefined,
    });

    expect(
      compactBridgeDeliveryDefaults({
        group_id: " ",
        mode: undefined,
        peer_id: "peer_123",
        thread_id: undefined,
      })
    ).toEqual({
      group_id: undefined,
      mode: undefined,
      peer_id: "peer_123",
      thread_id: undefined,
    });
    expect(compactBridgeDeliveryDefaults({})).toBeUndefined();
    expect(compactBridgeDeliveryDefaults({ parse_mode: "MarkdownV2" })).toEqual({
      parse_mode: "MarkdownV2",
    });
  });

  it("describes dm policy, provider config schema, and secret slots", () => {
    expect(describeBridgeDmPolicy("open")).toBe("Open direct messages");
    expect(describeBridgeDmPolicy("allowlist")).toBe("Allowlisted direct messages only");
    expect(describeBridgeDmPolicy("pairing")).toBe("Pairing required before direct messages");
    expect(describeBridgeDmPolicy(undefined)).toBe("Provider default");

    expect(
      describeBridgeProviderConfigSchema({
        schema: "provider-config",
        version: "2026-04-15",
      })
    ).toBe("provider-config · v2026-04-15");
    expect(describeBridgeProviderConfigSchema({ version: "2026-04-15" })).toBe("v2026-04-15");
    expect(describeBridgeProviderConfigSchema()).toBe("No structured config schema published");

    expect(
      describeBridgeSecretSlot({
        description: "Bot token",
        name: "bot_token",
        required: true,
      })
    ).toBe("Required · Bot token");
    expect(
      describeBridgeSecretSlot({
        name: "webhook_secret",
        required: false,
      })
    ).toBe("Optional");
  });

  it("formats provider config and target descriptions", () => {
    expect(formatBridgeProviderConfig({ mode: "bot" })).toContain('"mode": "bot"');
    expect(formatBridgeProviderConfig({})).toBe("");
    expect(formatBridgeProviderConfig(null)).toBe("");

    expect(
      describeBridgeDeliveryDefaults({
        group_id: "group_123",
        mode: "reply",
        peer_id: "peer_123",
      })
    ).toBe("reply · peer:peer_123 · group:group_123");
    expect(describeBridgeDeliveryDefaults(undefined)).toBe("No delivery defaults configured");

    expect(
      describeBridgeRouteTarget({
        group_id: "group_123",
        peer_id: "peer_123",
        thread_id: "thread_123",
      })
    ).toBe("peer:peer_123 · group:group_123 · thread:thread_123");
    expect(describeBridgeRouteTarget({})).toBe("default target");

    expect(
      describeBridgeTestTarget({
        group_id: "group_123",
        mode: "reply",
        peer_id: "peer_123",
        thread_id: "thread_123",
      })
    ).toBe("reply · peer:peer_123 · group:group_123 · thread:thread_123");
    expect(describeBridgeTestTarget({})).toBe("Bridge defaults");
  });

  it.each([
    {
      expected: "progress:all · grouping:separate · typing:false · reactions:true",
      progress: {
        grouping: "separate" as const,
        reactions: true,
        tool_progress: "all" as const,
        typing: false,
      },
    },
    {
      expected: "progress:new · grouping:accumulate · typing:false · reactions:false",
      progress: {
        grouping: "accumulate" as const,
        tool_progress: "new" as const,
      },
    },
  ])("describes progress-only delivery defaults as $expected", ({ expected, progress }) => {
    expect(describeBridgeDeliveryDefaults({ progress })).toBe(expected);
  });

  it("Should map every bridge status to a PillTone with no violet emitters", () => {
    expect(bridgeStatusTone("ready")).toBe("success");
    expect(bridgeStatusTone("auth_required")).toBe("info");
    expect(bridgeStatusTone("error")).toBe("danger");
    expect(bridgeStatusTone("starting")).toBe("info");
    expect(bridgeStatusTone("degraded")).toBe("warning");
    expect(bridgeStatusTone("disabled")).toBe("neutral");
    expect(bridgeScopeTone("workspace")).toBe("info");
    expect(bridgeScopeTone("global")).toBe("neutral");
  });

  it("formats absolute and relative times", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-04-15T12:00:00Z"));

    expect(formatBridgeDateTime("2026-04-13T12:00:00Z")).toContain("Apr");
    expect(formatBridgeDateTime("invalid-date")).toBe("invalid-date");
    expect(formatBridgeDateTime(undefined)).toBe("Unavailable");

    expect(formatBridgeRelativeTime("2026-04-15T11:59:40Z")).toBe("Just now");
    expect(formatBridgeRelativeTime("2026-04-15T11:45:00Z")).toBe("15m ago");
    expect(formatBridgeRelativeTime("2026-04-15T16:00:00Z")).toBe("In 4h");
    expect(formatBridgeRelativeTime(undefined)).toBe("Never");

    vi.useRealTimers();
  });
});

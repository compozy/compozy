import { describe, expect, it, vi } from "vitest";

import type { BridgeProviderSecretSlot } from "../../types";
import {
  collectFilledBridgeSecretSlots,
  missingRequiredBridgeSecretSlots,
  submitBridgeSecretSlots,
} from "../bridge-secret-slot-submission";

const slots: BridgeProviderSecretSlot[] = [
  { name: "bot_token", description: "Bot token", required: true },
  { name: "signing_secret", description: "Signing secret", required: true },
  { name: "app_token", description: "Socket-mode app token", required: false },
];

describe("collectFilledBridgeSecretSlots", () => {
  it("Should keep provider-declared order and drop blank values", () => {
    const filled = collectFilledBridgeSecretSlots(slots, {
      signing_secret: "  shhh  ",
      bot_token: "xoxb-1",
      app_token: "   ",
    });

    expect(filled).toEqual([
      { name: "bot_token", value: "xoxb-1" },
      { name: "signing_secret", value: "shhh" },
    ]);
  });

  it("Should return nothing when the provider declares no slots", () => {
    expect(collectFilledBridgeSecretSlots(undefined, { bot_token: "x" })).toEqual([]);
  });
});

describe("missingRequiredBridgeSecretSlots", () => {
  it("Should treat an undeclared required flag as required", () => {
    const missing = missingRequiredBridgeSecretSlots(
      [{ name: "bot_token" }, { name: "app_token", required: false }],
      {}
    );

    expect(missing).toEqual(["bot_token"]);
  });

  it("Should report nothing once every required slot carries a value", () => {
    expect(
      missingRequiredBridgeSecretSlots(slots, { bot_token: "a", signing_secret: "b" })
    ).toEqual([]);
  });
});

describe("submitBridgeSecretSlots", () => {
  it("Should write every filled slot against the created bridge", async () => {
    const write = vi.fn().mockResolvedValue(undefined);

    const outcome = await submitBridgeSecretSlots(
      "brg_support",
      [
        { name: "bot_token", value: "xoxb-1" },
        { name: "signing_secret", value: "shhh" },
      ],
      write
    );

    expect(write).toHaveBeenNthCalledWith(1, {
      bridgeId: "brg_support",
      name: "bot_token",
      value: "xoxb-1",
    });
    expect(outcome).toEqual({
      bridgeId: "brg_support",
      succeeded: ["bot_token", "signing_secret"],
      failed: [],
    });
  });

  it("Should attempt every slot after a failure and never echo the plaintext back", async () => {
    const write = vi
      .fn()
      .mockRejectedValueOnce(new Error("vault unavailable"))
      .mockResolvedValueOnce(undefined);

    const outcome = await submitBridgeSecretSlots(
      "brg_support",
      [
        { name: "bot_token", value: "xoxb-1" },
        { name: "signing_secret", value: "shhh" },
      ],
      write
    );

    expect(write).toHaveBeenCalledTimes(2);
    expect(outcome.succeeded).toEqual(["signing_secret"]);
    expect(outcome.failed).toEqual([{ name: "bot_token", message: "vault unavailable" }]);
    expect(JSON.stringify(outcome)).not.toContain("xoxb-1");
  });

  it("Should describe a non-Error rejection with the slot name", async () => {
    const outcome = await submitBridgeSecretSlots(
      "brg_support",
      [{ name: "bot_token", value: "xoxb-1" }],
      vi.fn().mockRejectedValue("boom")
    );

    expect(outcome.failed).toEqual([{ name: "bot_token", message: "Failed to bind bot_token." }]);
  });
});

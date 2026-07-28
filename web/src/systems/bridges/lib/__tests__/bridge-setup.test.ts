// Suite: Bridge setup projection
// Invariant: setup state is derived only from current daemon-observable bridge facts.
// Boundary IN: fixed platform profiles and the pure setup projection.
// Boundary OUT: HTTP ordering, React rendering, and transient-state invalidation by route hooks.

import { describe, expect, it } from "vitest";

import {
  getBridgeSetupProfile,
  projectBridgeSetup,
  type BridgeSetupChecklistItemId,
  type BridgeSetupProjectionInput,
} from "../bridge-setup";
import type {
  BridgeDetailResponse,
  BridgeHealth,
  BridgeProvider,
  BridgeSecretBinding,
  BridgeVerifyResponse,
  BridgeWebhookRegistrationResponse,
} from "../../types";

type Bridge = BridgeDetailResponse["bridge"];

describe("bridge setup profiles", () => {
  it("Should expose only the fixed Slack manifest and Telegram registration affordances", () => {
    expect(getBridgeSetupProfile("slack")).toMatchObject({
      dashboardUrl: "https://api.slack.com/apps",
      manifest: "slack",
      webhookRegistration: null,
    });
    expect(getBridgeSetupProfile("telegram")).toMatchObject({
      dashboardUrl: "https://t.me/BotFather",
      manifest: null,
      webhookRegistration: "telegram",
    });
    expect(getBridgeSetupProfile("custom-provider")).toBeNull();
  });

  it.each([
    ["discord", "https://discord.com/developers/applications"],
    ["gchat", "https://console.cloud.google.com/apis/api/chat.googleapis.com/overview"],
    ["github", "https://github.com/settings/apps"],
    ["linear", "https://linear.app/settings/api/applications"],
    ["slack", "https://api.slack.com/apps"],
    ["teams", "https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade"],
    ["telegram", "https://t.me/BotFather"],
    ["whatsapp", "https://developers.facebook.com/apps/"],
  ] as const)("Should expose the fixed %s operator dashboard", (platform, dashboardUrl) => {
    expect(getBridgeSetupProfile(platform)?.dashboardUrl).toBe(dashboardUrl);
  });

  it("Should map checks to secret slots only by exact validated profile entries", () => {
    const input = makeSlackInput({
      verification: makeVerification([
        { check: "provider.identity", status: "pass", remediation: "" },
        { check: "webhook.signing_secret", status: "fail", remediation: "Rotate it." },
        { check: "scope.chat:write", status: "pass", remediation: "" },
        {
          check: "scope.bot_token",
          status: "fail",
          remediation: "This prose mentions signing_secret and bot_token.",
        },
      ]),
    });

    const result = projectBridgeSetup(input);

    expect(result.checksBySecretSlot.bot_token).toEqual([
      input.verification?.checks[0],
      input.verification?.checks[2],
    ]);
    expect(result.checksBySecretSlot.signing_secret).toEqual([input.verification?.checks[1]]);
    expect(result.globalChecks).toEqual([input.verification?.checks[3]]);
  });

  it("Should keep a mapped check global when the provider descriptor omits its exact slot", () => {
    const input = makeSlackInput({
      provider: makeProvider("slack", [{ name: "bot_token", required: true }]),
      verification: makeVerification([
        { check: "webhook.signing_secret", status: "fail", remediation: "Bind it." },
      ]),
    });

    const result = projectBridgeSetup(input);

    expect(result.profileIssues).toContainEqual({
      kind: "missing-secret-slot",
      check: "webhook.signing_secret",
      slot: "signing_secret",
    });
    expect(result.checksBySecretSlot).toEqual({});
    expect(result.globalChecks).toEqual(input.verification?.checks);
  });

  it("Should keep the collective Teams identity check global instead of blaming both secrets", () => {
    const bridge = makeBridge({ platform: "teams", extension_name: "ext-teams" });
    const verification = makeVerification([
      {
        check: "provider.identity",
        status: "fail",
        remediation: "Bind the missing Teams credential.",
      },
    ]);
    const result = projectBridgeSetup(
      makeInput({
        bridge,
        provider: makeProvider("teams", [
          { name: "app_id", required: true },
          { name: "app_password", required: true },
        ]),
        verification,
      })
    );

    expect(result.checksBySecretSlot).toEqual({});
    expect(result.globalChecks).toEqual(verification.checks);
  });
});

describe("projectBridgeSetup", () => {
  it("Should report missing required secrets while ignoring optional slots and other bridge bindings", () => {
    const telegramProvider = makeProvider("telegram", [
      { name: "bot_token", required: true },
      { name: "webhook_secret", required: false },
    ]);
    const bridge = makeBridge({ platform: "telegram", extension_name: "ext-telegram" });
    const missing = projectBridgeSetup(
      makeInput({
        bridge,
        provider: telegramProvider,
        bindings: [makeBinding("bot_token", "another-bridge")],
      })
    );
    const complete = projectBridgeSetup(
      makeInput({
        bridge,
        provider: telegramProvider,
        bindings: [makeBinding("bot_token", bridge.id)],
      })
    );

    expect(missing.facts.missingRequiredSecretSlots).toEqual(["bot_token"]);
    expect(stateOf(missing, "secrets")).toBe("action-needed");
    expect(complete.facts.missingRequiredSecretSlots).toEqual([]);
    expect(stateOf(complete, "secrets")).toBe("complete");
  });

  it("Should treat a secret slot with omitted required metadata as required", () => {
    const provider = makeProvider("slack", [
      { name: "bot_token" },
      { name: "signing_secret", required: false },
    ]);
    const missing = projectBridgeSetup(makeInput({ provider }));
    const complete = projectBridgeSetup(
      makeInput({ provider, bindings: [makeBinding("bot_token")] })
    );

    expect(missing.facts.requiredSecretSlots).toEqual(["bot_token"]);
    expect(stateOf(missing, "secrets")).toBe("action-needed");
    expect(stateOf(complete, "secrets")).toBe("complete");
  });

  it("Should mark provider-dependent facts unknown for an absent or mismatched descriptor", () => {
    const absent = projectBridgeSetup(makeInput({ provider: null }));
    const mismatch = projectBridgeSetup(
      makeInput({ provider: makeProvider("telegram", [{ name: "bot_token", required: true }]) })
    );

    expect(stateOf(absent, "provider")).toBe("action-needed");
    expect(stateOf(absent, "secrets")).toBe("unknown");
    expect(stateOf(mismatch, "provider")).toBe("unknown");
    expect(mismatch.profileIssues).toContainEqual({
      kind: "provider-mismatch",
      expectedPlatform: "slack",
      actualPlatform: "telegram",
    });
  });

  it("Should derive webhook readiness only from the daemon-owned public URL", () => {
    const missing = projectBridgeSetup(
      makeSlackInput({
        bridge: makeBridge({
          webhook_public_url: undefined,
          provider_config: { webhook: { public_url: "https://plausible.example.test/slack" } },
        }),
      })
    );
    const configured = projectBridgeSetup(
      makeSlackInput({
        bridge: makeBridge({
          webhook_public_url: "https://hooks.example.test/slack",
          provider_config: { webhook: { public_url: "not browser-validated" } },
        }),
      })
    );

    expect(stateOf(missing, "webhook")).toBe("action-needed");
    expect(missing.facts.webhookPublicUrl).toBeNull();
    expect(stateOf(configured, "webhook")).toBe("complete");
    expect(configured.facts.webhookPublicUrl).toBe("https://hooks.example.test/slack");
  });

  it.each([
    ["pass", "complete"],
    ["warn", "warn"],
    ["fail", "fail"],
    ["skipped", "skipped"],
  ] as const)("Should preserve current verify status %s as %s", (status, expected) => {
    const result = projectBridgeSetup(
      makeSlackInput({
        verification: makeVerification([
          {
            check: "provider.identity",
            status,
            remediation: status === "pass" ? "" : `${status} remediation`,
          },
        ]),
      })
    );

    expect(stateOf(result, "verified")).toBe(expected);
  });

  it("Should prefer fail over warn and skipped when summarizing current verification", () => {
    const result = projectBridgeSetup(
      makeSlackInput({
        verification: makeVerification([
          { check: "check.skipped", status: "skipped", remediation: "Skipped." },
          { check: "check.warn", status: "warn", remediation: "Warn." },
          { check: "check.fail", status: "fail", remediation: "Fail." },
        ]),
      })
    );

    expect(result.checklist.find(item => item.id === "verified")).toEqual({
      id: "verified",
      state: "fail",
      remediation: "Fail.",
    });
  });

  it("Should complete verification when passed checks include a pre-enable skipped companion", () => {
    const verification = makeVerification([
      { check: "provider.identity", status: "pass", remediation: "" },
      {
        check: "webhook.reachability",
        status: "skipped",
        remediation: "Enable the bridge before checking reachability.",
      },
    ]);
    const result = projectBridgeSetup(makeSlackInput({ verification }));

    expect(stateOf(result, "verified")).toBe("complete");
    expect(result.globalChecks).toContainEqual(verification.checks[1]);
  });

  it("Should never infer verification from ready health or accept another bridge response", () => {
    const noVerification = projectBridgeSetup(
      makeSlackInput({ health: makeHealth({ status: "ready" }), verification: null })
    );
    const staleVerification = makeVerification(
      [{ check: "provider.identity", status: "pass", remediation: "" }],
      "another-bridge"
    );
    const stale = projectBridgeSetup(makeSlackInput({ verification: staleVerification }));

    expect(stateOf(noVerification, "healthy")).toBe("complete");
    expect(stateOf(noVerification, "verified")).toBe("action-needed");
    expect(stateOf(stale, "verified")).toBe("unknown");
    expect(stale.facts.verificationCurrent).toBe(false);
    expect(stale.checksBySecretSlot).toEqual({});
    expect(stale.globalChecks).toEqual([]);
  });

  it.each([
    { label: "missing", registration: null, expected: "action-needed" },
    { label: "pass", registration: makeRegistration("pass"), expected: "complete" },
    { label: "warn", registration: makeRegistration("warn"), expected: "warn" },
    { label: "fail", registration: makeRegistration("fail"), expected: "fail" },
    { label: "skipped", registration: makeRegistration("skipped"), expected: "skipped" },
    {
      label: "another bridge",
      registration: makeRegistration("pass", "another-bridge"),
      expected: "unknown",
    },
  ] as const)(
    "Should project transient Telegram registration $label as $expected",
    ({ registration, expected }) => {
      const bridge = makeBridge({ platform: "telegram", extension_name: "ext-telegram" });
      const result = projectBridgeSetup(
        makeInput({
          bridge,
          provider: makeProvider("telegram", [
            { name: "bot_token", required: true },
            { name: "webhook_secret", required: false },
          ]),
          registration,
        })
      );

      expect(stateOf(result, "webhook")).toBe(expected);
    }
  );

  it("Should ignore a registration response for a provider without that fixed affordance", () => {
    const result = projectBridgeSetup(makeSlackInput({ registration: makeRegistration("fail") }));

    expect(stateOf(result, "webhook")).toBe("complete");
    expect(result.facts.registrationCurrent).toBe(false);
  });

  it.each([
    ["ready", "complete"],
    ["starting", "warn"],
    ["degraded", "warn"],
    ["auth_required", "fail"],
    ["error", "fail"],
    ["disabled", "skipped"],
  ] as const)("Should project health status %s as %s", (status, expected) => {
    const result = projectBridgeSetup(makeSlackInput({ health: makeHealth({ status }) }));

    expect(stateOf(result, "healthy")).toBe(expected);
  });

  it("Should mark enabled and ready setup complete only from their own daemon fields", () => {
    const result = projectBridgeSetup(
      makeSlackInput({
        bridge: makeBridge({ enabled: true, status: "ready" }),
        bindings: [makeBinding("bot_token"), makeBinding("signing_secret")],
        health: makeHealth({ status: "ready" }),
        verification: makeVerification([
          { check: "provider.identity", status: "pass", remediation: "" },
          { check: "webhook.signing_secret", status: "pass", remediation: "" },
          { check: "webhook.configuration", status: "pass", remediation: "" },
          { check: "webhook.reachability", status: "pass", remediation: "" },
        ]),
      })
    );

    expect(result.checklist.map(item => item.state)).toEqual([
      "complete",
      "complete",
      "complete",
      "complete",
      "complete",
      "complete",
    ]);
  });

  it("Should keep unsupported-platform webhook and checks unknown instead of inventing capabilities", () => {
    const bridge = makeBridge({
      platform: "custom-provider",
      extension_name: "ext-custom-provider",
    });
    const verification = makeVerification([
      { check: "provider.identity", status: "pass", remediation: "" },
    ]);
    const result = projectBridgeSetup(
      makeInput({
        bridge,
        provider: makeProvider("custom-provider", [{ name: "api_key", required: true }]),
        verification,
      })
    );

    expect(result.profile).toBeNull();
    expect(stateOf(result, "provider")).toBe("complete");
    expect(stateOf(result, "webhook")).toBe("unknown");
    expect(result.globalChecks).toEqual(verification.checks);
    expect(result.checksBySecretSlot).toEqual({});
  });
});

function stateOf(
  projection: ReturnType<typeof projectBridgeSetup>,
  id: BridgeSetupChecklistItemId
) {
  return projection.checklist.find(item => item.id === id)?.state;
}

function makeSlackInput(
  overrides: Partial<BridgeSetupProjectionInput> = {}
): BridgeSetupProjectionInput {
  return makeInput({
    provider: makeProvider("slack", [
      { name: "bot_token", required: true },
      { name: "signing_secret", required: true },
    ]),
    ...overrides,
  });
}

function makeInput(
  overrides: Partial<BridgeSetupProjectionInput> = {}
): BridgeSetupProjectionInput {
  return {
    provider: makeProvider("slack", [
      { name: "bot_token", required: true },
      { name: "signing_secret", required: true },
    ]),
    bridge: makeBridge(),
    health: makeHealth(),
    bindings: [],
    verification: null,
    registration: null,
    ...overrides,
  };
}

function makeProvider(
  platform: string,
  secretSlots: NonNullable<BridgeProvider["secret_slots"]>
): BridgeProvider {
  return {
    display_name: platform,
    enabled: true,
    extension_name: `ext-${platform}`,
    health: "healthy",
    platform,
    secret_slots: secretSlots,
    state: "active",
  };
}

function makeBridge(overrides: Partial<Bridge> = {}): Bridge {
  return {
    created_at: "2026-07-12T12:00:00Z",
    display_name: "Setup bridge",
    enabled: false,
    extension_name: "ext-slack",
    id: "bridge-setup",
    notification_suppress: false,
    platform: "slack",
    provider_config: { webhook: { public_url: "https://hooks.example.com/bridge-setup" } },
    webhook_public_url: "https://hooks.example.com/bridge-setup",
    routing_policy: {
      include_group: true,
      include_peer: true,
      include_thread: true,
    },
    scope: "workspace",
    source: "dynamic",
    status: "disabled",
    updated_at: "2026-07-12T12:00:00Z",
    workspace_id: "workspace-alpha",
    ...overrides,
  };
}

function makeBinding(bindingName: string, bridgeID = "bridge-setup"): BridgeSecretBinding {
  return {
    binding_name: bindingName,
    bridge_instance_id: bridgeID,
    created_at: "2026-07-12T12:00:00Z",
    kind: "vault",
    secret_ref: `vault:bridges/${bridgeID}/${bindingName}`,
    updated_at: "2026-07-12T12:00:00Z",
  };
}

function makeHealth(overrides: Partial<BridgeHealth> = {}): BridgeHealth {
  return {
    auth_failures_total: 0,
    bridge_instance_id: "bridge-setup",
    delivery_backlog: 0,
    delivery_dropped_total: 0,
    delivery_failures_total: 0,
    route_count: 0,
    status: "disabled",
    ...overrides,
  };
}

function makeVerification(
  checks: BridgeVerifyResponse["checks"],
  bridgeID = "bridge-setup"
): BridgeVerifyResponse {
  return { bridge_instance_id: bridgeID, checks };
}

function makeRegistration(
  status: BridgeWebhookRegistrationResponse["status"],
  bridgeID = "bridge-setup"
): BridgeWebhookRegistrationResponse {
  return {
    bridge_instance_id: bridgeID,
    remediation: status === "pass" ? "" : `${status} remediation`,
    status,
  };
}

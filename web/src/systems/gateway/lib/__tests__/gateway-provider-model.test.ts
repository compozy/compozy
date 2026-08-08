// Suite: connectivity provider projection
// Invariant: the provider surface offers exactly the installed extensions that declare the
// connectivity capability, reports every blocker from the extension's own payload, and carries the
// real install source, control digest and generation the daemon's enable contract fences on.
// Owning layer: the gateway read model between GET /api/extensions + GET /api/gateway/status and
// the provider section.
import { describe, expect, it } from "vitest";

import {
  connectivityProviderFixture,
  gatewayStatusFixture,
  gatewayTierFixture,
} from "../../mocks/fixtures";
import {
  activationForTier,
  buildGatewayProviderModel,
  connectivityProviderExtensions,
  providerGeneration,
} from "../gateway-provider-model";

describe("gateway provider model", () => {
  it("Should offer only extensions that declare the connectivity capability", () => {
    const model = buildGatewayProviderModel(gatewayStatusFixture(), [
      connectivityProviderFixture(),
      connectivityProviderFixture({ capabilities: ["tool.provider"], name: "otel-bridge" }),
    ]);

    expect(model.candidates.map(candidate => candidate.name)).toEqual(["connectivity-overlay"]);
  });

  it("Should state that nothing is installed rather than implying a provider exists", () => {
    const model = buildGatewayProviderModel(gatewayStatusFixture(), []);

    expect(model.isEmpty).toBe(true);
    expect(model.candidates).toEqual([]);
    expect(model.orphanedActivations).toEqual([]);
  });

  it("Should carry the install source and control digest the enable contract fences on", () => {
    const model = buildGatewayProviderModel(gatewayStatusFixture(), [
      connectivityProviderFixture({
        network_requirement_digest: "sha256:live-digest",
        source: "marketplace",
      }),
    ]);
    const candidate = model.candidates[0];

    expect(candidate.installSource).toBe("marketplace");
    expect(candidate.digest).toBe("sha256:live-digest");
    expect(candidate.blockers).toEqual([]);
  });

  it("Should name the extension action that clears each blocker", () => {
    const model = buildGatewayProviderModel(gatewayStatusFixture(), [
      connectivityProviderFixture({
        enabled: false,
        missing_env: ["OVERLAY_AUTH_KEY"],
        network_confirmation_required: true,
      }),
    ]);
    const blockers = model.candidates[0].blockers;

    expect(blockers.map(blocker => blocker.id)).toEqual([
      "extension-disabled",
      "network-confirmation",
      "missing-secrets",
    ]);
    expect(blockers.every(blocker => blocker.message.includes("Settings → Extensions"))).toBe(true);
    expect(blockers.find(blocker => blocker.id === "missing-secrets")?.message).toContain(
      "OVERLAY_AUTH_KEY"
    );
  });

  it("Should report a workspace-scoped provider as disqualified, not merely unready", () => {
    const model = buildGatewayProviderModel(gatewayStatusFixture(), [
      connectivityProviderFixture({ enabled: false, source: "workspace" }),
    ]);
    const blockers = model.candidates[0].blockers;

    expect(blockers).toHaveLength(1);
    expect(blockers[0]).toMatchObject({ id: "workspace-source" });
  });

  it("Should attach the durable activation and its generation for the tier it serves", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({ tier: "private", observed: "up", verified: true })
    );
    const model = buildGatewayProviderModel(status, [
      connectivityProviderFixture({ name: "connectivity-fixture" }),
    ]);
    const candidate = model.candidates[0];

    expect(activationForTier(candidate, "private")?.health).toBe("healthy");
    expect(activationForTier(candidate, "public")).toBeUndefined();
    expect(providerGeneration(candidate, "private")).toBe(1);
    // No row yet for the public tier, so the enable call fences on generation 0.
    expect(providerGeneration(candidate, "public")).toBe(0);
  });

  it("Should keep an activation whose extension is gone visible so it can be disabled", () => {
    const status = gatewayStatusFixture(
      gatewayTierFixture({ tier: "private", observed: "degraded", verified: false })
    );
    const model = buildGatewayProviderModel(status, []);

    expect(model.isEmpty).toBe(false);
    expect(model.orphanedActivations.map(activation => activation.name)).toEqual([
      "connectivity-fixture",
    ]);
  });

  it("Should expose the capability filter used by the surface", () => {
    const extensions = [
      connectivityProviderFixture(),
      connectivityProviderFixture({ capabilities: undefined, name: "no-capabilities" }),
    ];

    expect(connectivityProviderExtensions(extensions).map(entry => entry.name)).toEqual([
      "connectivity-overlay",
    ]);
  });
});

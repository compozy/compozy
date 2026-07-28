// Suite: Extensions management transport
// Invariant: Every management action uses its generated HTTP route and preserves daemon errors.
// Boundary IN: Generated OpenAPI client requests and typed management payloads.
// Boundary OUT: Query-cache behavior and visible lifecycle feedback, owned by hook/component suites.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  deactivateBundle,
  disableExtension,
  enableExtension,
  ExtensionsApiError,
  getBundleActivation,
  getExtensionProvenance,
  listBundleActivations,
  listExtensions,
  removeExtension,
  updateBundleActivation,
  updateExtension,
} from "../extensions-api";
import {
  bundleActivationFixtures,
  extensionFixtures,
  extensionProvenanceFixtures,
} from "../../mocks/fixtures";
import { expectFetchRequest, mockEmptyResponse, mockJsonResponse } from "@/test/fetch-test-utils";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("extensions management reads", () => {
  it("Should read extension inventory and provenance with abortable generated requests", async () => {
    const controller = new AbortController();
    mockJsonResponse({ extensions: extensionFixtures });
    await expect(listExtensions(controller.signal)).resolves.toEqual(extensionFixtures);
    await expectFetchRequest({ path: "/api/extensions", signal: controller.signal });

    mockJsonResponse({ provenance: extensionProvenanceFixtures["otel-bridge"] });
    await expect(getExtensionProvenance("otel-bridge")).resolves.toEqual(
      extensionProvenanceFixtures["otel-bridge"]
    );
    await expectFetchRequest({ callIndex: 1, path: "/api/extensions/otel-bridge/provenance" });
  });

  it("Should read bundle inventory and one activation by stable id", async () => {
    mockJsonResponse({ activations: bundleActivationFixtures });
    await expect(listBundleActivations()).resolves.toEqual(bundleActivationFixtures);
    await expectFetchRequest({ path: "/api/bundles/activations" });

    mockJsonResponse({ activation: bundleActivationFixtures[0] });
    await expect(getBundleActivation("activation-ops-starter")).resolves.toEqual(
      bundleActivationFixtures[0]
    );
    await expectFetchRequest({
      callIndex: 1,
      path: "/api/bundles/activations/activation-ops-starter",
    });
  });
});

describe("extensions management mutations", () => {
  it("Should enable and disable one extension through distinct lifecycle routes", async () => {
    const controller = new AbortController();
    mockJsonResponse({ extension: extensionFixtures[0] });
    await expect(enableExtension("otel-bridge", controller.signal)).resolves.toEqual(
      extensionFixtures[0]
    );
    await expectFetchRequest({
      method: "POST",
      path: "/api/extensions/otel-bridge/enable",
      signal: controller.signal,
    });

    mockJsonResponse({ extension: extensionFixtures[0] });
    await expect(disableExtension("otel-bridge", controller.signal)).resolves.toEqual(
      extensionFixtures[0]
    );
    await expectFetchRequest({
      callIndex: 1,
      method: "POST",
      path: "/api/extensions/otel-bridge/disable",
      signal: controller.signal,
    });
  });

  it("Should update and remove one extension through generated management routes", async () => {
    const controller = new AbortController();
    mockEmptyResponse();
    await expect(updateExtension("otel-bridge", {}, controller.signal)).resolves.toBeUndefined();
    await expectFetchRequest({
      body: {},
      method: "PUT",
      path: "/api/extensions/otel-bridge",
      signal: controller.signal,
    });

    mockEmptyResponse({ status: 204 });
    await expect(removeExtension("otel-bridge", controller.signal)).resolves.toBeUndefined();
    await expectFetchRequest({
      callIndex: 1,
      method: "DELETE",
      path: "/api/extensions/otel-bridge",
      signal: controller.signal,
    });
  });

  it("Should confirm a network requirement when updating and deactivating a bundle", async () => {
    const controller = new AbortController();
    const body = { confirm_network_requirement: true, expected_version: 7 };
    mockJsonResponse({ activation: bundleActivationFixtures[0] });
    await expect(
      updateBundleActivation("activation-ops-starter", body, controller.signal)
    ).resolves.toEqual(bundleActivationFixtures[0]);
    await expectFetchRequest({
      body,
      method: "PATCH",
      path: "/api/bundles/activations/activation-ops-starter",
      signal: controller.signal,
    });

    mockEmptyResponse({ status: 204 });
    await expect(
      deactivateBundle("activation-ops-starter", controller.signal)
    ).resolves.toBeUndefined();
    await expectFetchRequest({
      callIndex: 1,
      method: "DELETE",
      path: "/api/bundles/activations/activation-ops-starter",
      signal: controller.signal,
    });
  });
});

describe("extensions management failures", () => {
  it("Should surface the daemon error body for a rejected mutation", async () => {
    mockJsonResponse({ error: "active bundle still depends on this extension" }, { status: 409 });

    const error = await removeExtension("slack-notify").catch(reason => reason);
    expect(error).toBeInstanceOf(ExtensionsApiError);
    expect(error).toMatchObject({
      kind: "daemon",
      message: "active bundle still depends on this extension",
      status: 409,
    });
  });

  it("Should fall back to operation and status when the daemon has no error body", async () => {
    mockEmptyResponse({ status: 503 });

    const error = await listBundleActivations().catch(reason => reason);
    expect(error).toBeInstanceOf(ExtensionsApiError);
    expect(error).toMatchObject({
      kind: "transport",
      message: "Failed to list bundle activations (503)",
      status: 503,
    });
  });

  it("Should reject a successful response that omits required data", async () => {
    mockEmptyResponse();

    const error = await listExtensions().catch(reason => reason);
    expect(error).toBeInstanceOf(ExtensionsApiError);
    expect(error).toMatchObject({
      kind: "malformed_response",
      message: "Failed to list extensions: empty response (200)",
      status: 200,
    });
  });

  it.each([
    ["extension inventory", "extensions", () => listExtensions()],
    ["extension provenance", "provenance", () => getExtensionProvenance("otel-bridge")],
    ["extension lifecycle", "extension", () => enableExtension("otel-bridge")],
    ["bundle inventory", "activations", () => listBundleActivations()],
    ["bundle detail", "activation", () => getBundleActivation("activation-ops-starter")],
    [
      "bundle update",
      "activation",
      () =>
        updateBundleActivation("activation-ops-starter", {
          confirm_network_requirement: true,
          expected_version: 7,
        }),
    ],
  ])("Should reject a successful empty %s envelope", async (_name, field, invoke) => {
    mockJsonResponse({});

    const error = await invoke().catch(reason => reason);
    expect(error).toBeInstanceOf(ExtensionsApiError);
    expect(error).toMatchObject({
      kind: "malformed_response",
      message: expect.stringContaining(`missing ${field}`),
      status: 200,
    });
  });
});

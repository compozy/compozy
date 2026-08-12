// Suite: Extensions management transport
// Invariant: Every management action uses its generated HTTP route and preserves daemon errors.
// Boundary IN: Generated OpenAPI client requests and typed management payloads.
// Boundary OUT: Query-cache behavior and visible lifecycle feedback, owned by hook/component suites.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  disableExtension,
  enableExtension,
  ExtensionsApiError,
  getExtensionInventory,
  getExtensionProvenance,
  listExtensionLogs,
  listExtensions,
  removeExtension,
  updateExtension,
} from "../extensions-api";
import {
  extensionFixtures,
  extensionInventoryFixtures,
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
    await expect(listExtensions({}, controller.signal)).resolves.toEqual(extensionFixtures);
    await expectFetchRequest({ path: "/api/extensions", signal: controller.signal });

    mockJsonResponse({ provenance: extensionProvenanceFixtures["otel-bridge"] });
    await expect(getExtensionProvenance("otel-bridge")).resolves.toEqual(
      extensionProvenanceFixtures["otel-bridge"]
    );
    await expectFetchRequest({ callIndex: 1, path: "/api/extensions/otel-bridge/provenance" });
  });

  it("Should read the kit inventory for one extension without a workspace selector", async () => {
    const items = extensionInventoryFixtures["dep-kit-ops"]!;
    const payload = { enabled: false, extension: "dep-kit-ops", items };
    mockJsonResponse(payload);

    await expect(getExtensionInventory("dep-kit-ops")).resolves.toEqual(payload);
    await expectFetchRequest({ path: "/api/extensions/dep-kit-ops/inventory" });
  });
});

describe("extensions management mutations", () => {
  it("Should enable and disable one extension through distinct lifecycle routes", async () => {
    const controller = new AbortController();
    const result = { automation_started: ["span-export"], extension: extensionFixtures[0] };
    mockJsonResponse(result);
    await expect(enableExtension("otel-bridge", {}, controller.signal)).resolves.toEqual(result);
    await expectFetchRequest({
      body: {},
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
    await expect(removeExtension("otel-bridge", {}, controller.signal)).resolves.toBeUndefined();
    await expectFetchRequest({
      callIndex: 1,
      method: "DELETE",
      path: "/api/extensions/otel-bridge",
      signal: controller.signal,
    });
  });

  it("Should bind the workspace instance on scoped reads, removal, and log history", async () => {
    mockJsonResponse({ extensions: extensionFixtures });
    await listExtensions({ workspaceId: "ws_northstar" });
    await expectFetchRequest({ path: "/api/extensions?workspace=ws_northstar" });

    mockEmptyResponse({ status: 204 });
    await removeExtension("otel-bridge", { workspaceId: "ws_northstar" });
    await expectFetchRequest({
      callIndex: 1,
      method: "DELETE",
      path: "/api/extensions/otel-bridge?workspace=ws_northstar",
    });

    mockJsonResponse({ logs: [], stream_epoch: "epoch-northstar" });
    await expect(
      listExtensionLogs("otel-bridge", {
        after: 12,
        streamEpoch: "epoch-northstar",
        workspaceId: "ws_northstar",
      })
    ).resolves.toEqual({ logs: [], stream_epoch: "epoch-northstar" });
    await expectFetchRequest({
      callIndex: 2,
      path: "/api/extensions/otel-bridge/logs?workspace=ws_northstar&after=12&stream_epoch=epoch-northstar",
    });
  });

  it("Should address the global instance when no workspace is active", async () => {
    mockJsonResponse({ logs: [], stream_epoch: "epoch-global" });
    await expect(listExtensionLogs("otel-bridge", { workspaceId: "   " })).resolves.toEqual({
      logs: [],
      stream_epoch: "epoch-global",
    });
    await expectFetchRequest({ path: "/api/extensions/otel-bridge/logs" });
  });

  it("Should ratify the exact digest on both the enable and update routes", async () => {
    const digest = "sha256:6f1c0a94d3b27e58";
    mockJsonResponse({ automation_started: [], extension: extensionFixtures[0] });
    await enableExtension("dep-kit-ops", { confirmNetworkDigest: digest });
    await expectFetchRequest({
      body: { confirm_network_digest: digest },
      method: "POST",
      path: "/api/extensions/dep-kit-ops/enable",
    });

    mockEmptyResponse();
    await updateExtension("dep-kit-ops", {
      allow_unverified: true,
      confirm_network_digest: digest,
      version: "1.1.0",
    });
    await expectFetchRequest({
      body: { allow_unverified: true, confirm_network_digest: digest, version: "1.1.0" },
      callIndex: 1,
      method: "PUT",
      path: "/api/extensions/dep-kit-ops",
    });
  });
});

describe("extensions management failures", () => {
  it("Should surface the daemon error body for a rejected mutation", async () => {
    mockJsonResponse({ error: "extension files are locked by another operation" }, { status: 409 });

    const error = await removeExtension("slack-notify").catch(reason => reason);
    expect(error).toBeInstanceOf(ExtensionsApiError);
    expect(error).toMatchObject({
      kind: "daemon",
      message: "extension files are locked by another operation",
      status: 409,
    });
  });

  /**
   * The digest is the whole remediation: without it the operator cannot ratify what the daemon
   * refused, so the typed error has to carry it rather than flatten to a message.
   */
  it("Should expose the daemon code and current digest from a network-confirmation refusal", async () => {
    mockJsonResponse(
      {
        code: "extension_network_confirmation_required",
        current_digest: "sha256:6f1c0a94d3b27e58",
        error: "dep-kit-ops declares Live network participation that has not been confirmed",
      },
      { status: 409 }
    );

    const error = await enableExtension("dep-kit-ops").catch((reason: unknown) => reason);
    expect(error).toBeInstanceOf(ExtensionsApiError);
    expect(error).toMatchObject({
      code: "extension_network_confirmation_required",
      currentDigest: "sha256:6f1c0a94d3b27e58",
      kind: "daemon",
      status: 409,
    });
  });

  it("Should fall back to operation and status when the daemon has no error body", async () => {
    mockEmptyResponse({ status: 503 });

    const error = await getExtensionInventory("dep-kit-ops").catch((reason: unknown) => reason);
    expect(error).toBeInstanceOf(ExtensionsApiError);
    expect(error).toMatchObject({
      kind: "transport",
      message: "Failed to load kit inventory for dep-kit-ops (503)",
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

  it("Should reject a log snapshot without its stream identity", async () => {
    mockJsonResponse({ logs: [] });

    const error = await listExtensionLogs("otel-bridge").catch(reason => reason);
    expect(error).toBeInstanceOf(ExtensionsApiError);
    expect(error).toMatchObject({
      kind: "malformed_response",
      message: expect.stringContaining("missing stream_epoch"),
      status: 200,
    });
  });

  it.each([
    ["extension inventory", "extensions", () => listExtensions()],
    ["extension provenance", "provenance", () => getExtensionProvenance("otel-bridge")],
    ["extension lifecycle", "extension", () => enableExtension("otel-bridge")],
    ["kit inventory", "items", () => getExtensionInventory("dep-kit-ops")],
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

  it("Should reject kit inventory for a different extension identity", async () => {
    mockJsonResponse({ enabled: false, extension: "otel-bridge", items: [] });

    const error = await getExtensionInventory("dep-kit-ops").catch(reason => reason);
    expect(error).toBeInstanceOf(ExtensionsApiError);
    expect(error).toMatchObject({
      kind: "malformed_response",
      message: "Failed to load kit inventory for dep-kit-ops: extension identity mismatch (200)",
      status: 200,
    });
  });
});

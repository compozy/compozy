// Suite: Extensions Storybook transport state
// Invariant: Successful lifecycle mutations survive the GET refetch that follows them.
// Boundary IN: Stateful extensions MSW handlers and their explicit reset contract.
// Boundary OUT: Query invalidation timing and component rendering, owned by hook/component suites.
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, it } from "vitest";

import { handlers, resetExtensionMockState } from "../handlers";

const server = setupServer(...handlers);
const API = "http://localhost";

beforeAll(() => {
  server.listen({ onUnhandledRequest: "error" });
});

afterEach(() => {
  resetExtensionMockState();
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});

describe("extensions MSW handlers", () => {
  it("Should retain extension enablement and deletion across inventory refetches", async () => {
    const disable = await fetch(`${API}/api/extensions/otel-bridge/disable`, { method: "POST" });
    expect(disable.status).toBe(200);

    let inventory = await fetch(`${API}/api/extensions`);
    let body = (await inventory.json()) as {
      extensions: Array<{ enabled: boolean; name: string }>;
    };
    expect(body.extensions.find(extension => extension.name === "otel-bridge")?.enabled).toBe(
      false
    );

    const enable = await fetch(`${API}/api/extensions/otel-bridge/enable`, { method: "POST" });
    expect(enable.status).toBe(200);
    inventory = await fetch(`${API}/api/extensions`);
    body = (await inventory.json()) as { extensions: Array<{ enabled: boolean; name: string }> };
    expect(body.extensions.find(extension => extension.name === "otel-bridge")?.enabled).toBe(true);

    const remove = await fetch(`${API}/api/extensions/otel-bridge`, { method: "DELETE" });
    expect(remove.status).toBe(200);
    inventory = await fetch(`${API}/api/extensions`);
    body = (await inventory.json()) as { extensions: Array<{ enabled: boolean; name: string }> };
    expect(body.extensions.some(extension => extension.name === "otel-bridge")).toBe(false);
  });

  it("Should retain bundle updates and deactivation across activation refetches", async () => {
    const id = "activation-ops-starter";
    const update = await fetch(`${API}/api/bundles/activations/${id}`, {
      body: JSON.stringify({ confirm_network_requirement: true, expected_version: 7 }),
      headers: { "Content-Type": "application/json" },
      method: "PATCH",
    });
    expect(update.status).toBe(200);

    let detail = await fetch(`${API}/api/bundles/activations/${id}`);
    let body = (await detail.json()) as {
      activation: {
        network_requirement_confirmed_by?: string;
        spec_drift: boolean;
        version: number;
      };
    };
    expect(body.activation).toMatchObject({
      network_requirement_confirmed_by: "operator",
      spec_drift: false,
      version: 8,
    });

    const deactivate = await fetch(`${API}/api/bundles/activations/${id}`, {
      method: "DELETE",
    });
    expect(deactivate.status).toBe(204);
    detail = await fetch(`${API}/api/bundles/activations/${id}`);
    expect(detail.status).toBe(404);
  });

  it("Should mirror the daemon's instance scope, log ring, and union install contract", async () => {
    const global = (await (await fetch(`${API}/api/extensions`)).json()) as {
      extensions: Array<{ name: string }>;
    };
    expect(global.extensions.some(extension => extension.name === "ops-dev-extension")).toBe(false);

    const scoped = (await (await fetch(`${API}/api/extensions?workspace=ws_northstar`)).json()) as {
      extensions: Array<{ dev?: boolean; name: string }>;
    };
    expect(scoped.extensions[0]).toMatchObject({ dev: true, name: "ops-dev-extension" });

    const logs = (await (
      await fetch(`${API}/api/extensions/ops-dev-extension/logs?workspace=ws_northstar&after=2`)
    ).json()) as { logs: Array<{ sequence: number }> };
    expect(logs.logs.map(entry => entry.sequence)).toEqual([3]);

    const globalLogs = (await (
      await fetch(`${API}/api/extensions/ops-dev-extension/logs?after=2`)
    ).json()) as { logs: Array<{ sequence: number }> };
    expect(globalLogs.logs).toEqual([]);

    const unlink = await fetch(`${API}/api/extensions/ops-dev-extension?workspace=ws_northstar`, {
      method: "DELETE",
    });
    expect(unlink.status).toBe(200);

    const afterUnlink = (await (
      await fetch(`${API}/api/extensions?workspace=ws_northstar`)
    ).json()) as { extensions: Array<{ name: string }> };
    expect(afterUnlink.extensions.some(extension => extension.name === "ops-dev-extension")).toBe(
      false
    );

    const logsAfterUnlink = (await (
      await fetch(`${API}/api/extensions/ops-dev-extension/logs?workspace=ws_northstar`)
    ).json()) as { logs: Array<{ sequence: number }> };
    expect(logsAfterUnlink.logs).toEqual([]);

    const blocked = await fetch(`${API}/api/extensions`, {
      body: JSON.stringify({ ref: "acme/hello", source: "github" }),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    });
    expect(blocked.status).toBe(422);

    const consented = await fetch(`${API}/api/extensions`, {
      body: JSON.stringify({ allow_unverified: true, ref: "acme/hello", source: "github" }),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    });
    expect(consented.status).toBe(201);
    const installed = (await consented.json()) as {
      extension: { digest_matched: boolean; name: string; source: string };
    };
    expect(installed.extension).toMatchObject({
      digest_matched: true,
      name: "hello",
      source: "github",
    });
  });

  it("Should preserve not-found responses without mutating mock inventory", async () => {
    const missingExtensionRoutes = [
      ["/api/extensions/missing/provenance", "GET"],
      ["/api/extensions/missing/enable", "POST"],
      ["/api/extensions/missing/disable", "POST"],
      ["/api/extensions/missing", "PUT"],
      ["/api/extensions/missing", "DELETE"],
    ] as const;
    for (const [path, method] of missingExtensionRoutes) {
      const response = await fetch(`${API}${path}`, { method });
      expect(response.status).toBe(404);
    }

    const missingActivationRoutes = [
      ["/api/bundles/activations/missing", "GET"],
      ["/api/bundles/activations/missing", "PATCH"],
      ["/api/bundles/activations/missing", "DELETE"],
    ] as const;
    for (const [path, method] of missingActivationRoutes) {
      const response = await fetch(`${API}${path}`, { method });
      expect(response.status).toBe(404);
    }

    const inventory = await fetch(`${API}/api/extensions`);
    const body = (await inventory.json()) as { extensions: Array<{ name: string }> };
    expect(body.extensions.map(extension => extension.name)).toEqual([
      "otel-bridge",
      "slack-notify",
    ]);
  });
});

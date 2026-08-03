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

  /**
   * The mock has to gate on the digest exactly as the daemon does; a mock that enabled anyway
   * would let the confirm affordance pass its own tests while the real lifecycle refused.
   */
  it("Should refuse an unconfirmed enable and go live only once the digest is ratified", async () => {
    const refused = await fetch(`${API}/api/extensions/dep-kit-ops/enable`, {
      body: JSON.stringify({}),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    });
    expect(refused.status).toBe(409);
    const refusal = (await refused.json()) as { code: string; current_digest: string };
    expect(refusal).toMatchObject({
      code: "extension_network_confirmation_required",
      current_digest: "sha256:6f1c0a94d3b27e58",
    });

    const shipped = (await (await fetch(`${API}/api/extensions/dep-kit-ops/inventory`)).json()) as {
      enabled: boolean;
      items: Array<{ live: boolean }>;
    };
    expect(shipped.enabled).toBe(false);
    expect(shipped.items.every(item => item.live)).toBe(false);

    const confirmed = await fetch(`${API}/api/extensions/dep-kit-ops/enable`, {
      body: JSON.stringify({ confirm_network_digest: refusal.current_digest }),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    });
    expect(confirmed.status).toBe(200);
    const enabled = (await confirmed.json()) as { automation_started: string[] };
    expect(enabled.automation_started).toEqual(["weekly-audit"]);

    const live = (await (await fetch(`${API}/api/extensions/dep-kit-ops/inventory`)).json()) as {
      enabled: boolean;
      items: Array<{ live: boolean }>;
    };
    expect(live.enabled).toBe(true);
    expect(live.items.every(item => item.live)).toBe(true);

    await fetch(`${API}/api/extensions/dep-kit-ops/disable`, { method: "POST" });
    const afterDisable = (await (
      await fetch(`${API}/api/extensions/dep-kit-ops/inventory`)
    ).json()) as { items: Array<{ live: boolean }> };
    expect(afterDisable.items.some(item => item.live)).toBe(false);
  });

  it("Should refuse an unconfirmed update and accept an exact digest retry", async () => {
    const refused = await fetch(`${API}/api/extensions/dep-kit-ops`, {
      body: JSON.stringify({ version: "1.1.0" }),
      headers: { "Content-Type": "application/json" },
      method: "PUT",
    });
    expect(refused.status).toBe(409);
    const refusal = (await refused.json()) as { code: string; current_digest: string };
    expect(refusal).toMatchObject({
      code: "extension_network_confirmation_required",
      current_digest: "sha256:6f1c0a94d3b27e58",
    });

    const confirmed = await fetch(`${API}/api/extensions/dep-kit-ops`, {
      body: JSON.stringify({
        confirm_network_digest: refusal.current_digest,
        version: "1.1.0",
      }),
      headers: { "Content-Type": "application/json" },
      method: "PUT",
    });
    expect(confirmed.status).toBe(200);
    await expect(confirmed.json()).resolves.toMatchObject({
      update: { name: "dep-kit-ops", status: "current" },
    });
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
      ["/api/extensions/missing/inventory", "GET"],
      ["/api/extensions/missing/enable", "POST"],
      ["/api/extensions/missing/disable", "POST"],
      ["/api/extensions/missing", "PUT"],
      ["/api/extensions/missing", "DELETE"],
    ] as const;
    for (const [path, method] of missingExtensionRoutes) {
      const response = await fetch(`${API}${path}`, { method });
      expect(response.status).toBe(404);
    }

    const inventory = await fetch(`${API}/api/extensions`);
    const body = (await inventory.json()) as { extensions: Array<{ name: string }> };
    expect(body.extensions.map(extension => extension.name)).toEqual([
      "otel-bridge",
      "slack-notify",
      "dep-kit-ops",
    ]);
  });
});

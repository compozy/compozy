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
  it("Should retain profile enablement and deletion across inventory refetches", async () => {
    const disable = await fetch(`${API}/api/extensions/otel-bridge/enablement`, {
      body: JSON.stringify({ enabled: false, profile: "growth" }),
      headers: { "Content-Type": "application/json" },
      method: "PUT",
    });
    expect(disable.status).toBe(200);

    let inventory = await fetch(`${API}/api/extensions?profile=growth`);
    let body = (await inventory.json()) as {
      extensions: Array<{ enabled: boolean; name: string }>;
    };
    expect(body.extensions.find(extension => extension.name === "otel-bridge")?.enabled).toBe(
      false
    );

    const enable = await fetch(`${API}/api/extensions/otel-bridge/enablement`, {
      body: JSON.stringify({ enabled: true, profile: "growth" }),
      headers: { "Content-Type": "application/json" },
      method: "PUT",
    });
    expect(enable.status).toBe(200);
    inventory = await fetch(`${API}/api/extensions?profile=growth`);
    body = (await inventory.json()) as { extensions: Array<{ enabled: boolean; name: string }> };
    expect(body.extensions.find(extension => extension.name === "otel-bridge")?.enabled).toBe(true);

    const remove = await fetch(`${API}/api/extensions/otel-bridge`, { method: "DELETE" });
    expect(remove.status).toBe(200);
    inventory = await fetch(`${API}/api/extensions`);
    body = (await inventory.json()) as { extensions: Array<{ enabled: boolean; name: string }> };
    expect(body.extensions.some(extension => extension.name === "otel-bridge")).toBe(false);
  });

  it("Should preview network requirements and require the exact digest before install", async () => {
    const request = { ref: "dep-kit-ops", source: "curated", version: "1.1.0" };
    const previewResponse = await fetch(`${API}/api/extensions/preview-install`, {
      body: JSON.stringify(request),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    });
    expect(previewResponse.status).toBe(200);
    const preview = (await previewResponse.json()) as { network_requirement_digest: string };

    const refused = await fetch(`${API}/api/extensions`, {
      body: JSON.stringify(request),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    });
    expect(refused.status).toBe(409);
    const refusal = (await refused.json()) as { code: string; current_digest: string };
    expect(refusal).toMatchObject({
      code: "extension_network_confirmation_required",
      current_digest: "sha256:6f1c0a94d3b27e58",
    });

    expect(refusal.current_digest).toBe(preview.network_requirement_digest);

    const confirmed = await fetch(`${API}/api/extensions`, {
      body: JSON.stringify({
        ...request,
        confirm_network_digest: preview.network_requirement_digest,
      }),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    });
    expect(confirmed.status).toBe(201);
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

    const initialLogs = (await (
      await fetch(`${API}/api/extensions/ops-dev-extension/logs?workspace=ws_northstar`)
    ).json()) as { logs: Array<{ sequence: number }>; stream_epoch: string };
    expect(initialLogs.stream_epoch).toBe("epoch-ops-dev");

    const invalidCursor = await fetch(
      `${API}/api/extensions/ops-dev-extension/logs?workspace=ws_northstar&after=2`
    );
    expect(invalidCursor.status).toBe(400);

    const logs = (await (
      await fetch(
        `${API}/api/extensions/ops-dev-extension/logs?workspace=ws_northstar&after=2&stream_epoch=${initialLogs.stream_epoch}`
      )
    ).json()) as { logs: Array<{ sequence: number }>; stream_epoch: string };
    expect(logs.logs.map(entry => entry.sequence)).toEqual([3]);
    expect(logs.stream_epoch).toBe(initialLogs.stream_epoch);

    const globalLogs = (await (
      await fetch(`${API}/api/extensions/ops-dev-extension/logs`)
    ).json()) as { logs: Array<{ sequence: number }>; stream_epoch: string };
    expect(globalLogs.logs).toEqual([]);
    expect(globalLogs.stream_epoch).toBe("mock-global-ops-dev-extension");

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
    ).json()) as { logs: Array<{ sequence: number }>; stream_epoch: string };
    expect(logsAfterUnlink.logs).toEqual([]);
    expect(logsAfterUnlink.stream_epoch).not.toBe(initialLogs.stream_epoch);

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
      ["/api/extensions/missing/enablement", "PUT"],
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
      "acme.tools",
      "legacy-notes",
    ]);
  });

  /**
   * The inventory route carries the daemon's recorded skips beside the kit items; a mock that
   * dropped them would let the Skipped section pass its own tests against data the route never sends.
   */
  it("Should replay recorded ingest diagnostics on the inventory route", async () => {
    const degraded = (await (
      await fetch(`${API}/api/extensions/legacy-notes/inventory`)
    ).json()) as {
      diagnostics: Array<{ code: string; evidence?: Record<string, unknown> }>;
      format: string;
      items: unknown[];
    };
    expect(degraded.format).toBe("agent-plugin");
    expect(degraded.items).toEqual([]);
    expect(degraded.diagnostics.map(item => item.evidence?.scope)).toEqual([
      "mcp:notes-index",
      "skill:release-notes",
    ]);

    const healthy = (await (await fetch(`${API}/api/extensions/otel-bridge/inventory`)).json()) as {
      diagnostics: unknown[];
      format: string;
    };
    expect(healthy.format).toBe("compozy");
    expect(healthy.diagnostics).toEqual([]);
  });
});

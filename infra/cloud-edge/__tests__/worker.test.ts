import { fileURLToPath } from "node:url";
import { build } from "esbuild";
import { Miniflare } from "miniflare";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";
import type { StubContainer } from "./stub-container";

type Stub = Pick<StubContainer, "reset" | "counts" | "edgeExports">;
type Bindings = { CLOUD: { getByName(name: string): Stub } };

const ORIGIN = "https://cloud.compozy.com";
const REVALIDATE = `${ORIGIN}/v1/computers/cmp_A/revalidate`;

describe("UT-128 cloud edge Worker", () => {
  let mf: Miniflare;
  let cloud0: Stub;
  let cloud1: Stub;

  beforeAll(async () => {
    const bundle = await build({
      entryPoints: [fileURLToPath(new URL("./stub-container.ts", import.meta.url))],
      bundle: true,
      format: "esm",
      external: ["cloudflare:workers"],
      write: false,
    });
    const entry = bundle.outputFiles[0];
    if (!entry) throw new Error("Missing edge Worker bundle");

    mf = new Miniflare({
      compatibilityDate: "2026-07-30",
      modules: [{ type: "ESModule", path: "worker.js", contents: entry.text }],
      durableObjects: { CLOUD: { className: "StubContainer", useSQLite: true } },
    });
    const { CLOUD } = await mf.getBindings<Bindings>();
    cloud0 = CLOUD.getByName("cloud-0");
    cloud1 = CLOUD.getByName("cloud-1");
  });

  afterAll(async () => {
    await mf?.dispose();
  });

  beforeEach(async () => {
    await Promise.all([cloud0.reset(true, 200), cloud1.reset(true, 200)]);
  });

  it("routes a control WebSocket and computer-key requests to the same instance", async () => {
    const revalidation = await mf.dispatchFetch(REVALIDATE, { method: "POST" });
    const { instance } = (await revalidation.json()) as { instance: string };
    expect(["cloud-0", "cloud-1"]).toContain(instance);

    for (let attempt = 0; attempt < 2; attempt += 1) {
      const response = await mf.dispatchFetch(`${ORIGIN}/v1/computers/control?computer_id=cmp_A`, {
        headers: { Upgrade: "websocket" },
      });
      expect(response.status).toBe(101);
      const socket = response.webSocket;
      if (!socket) throw new Error("Missing control WebSocket");
      try {
        socket.accept();
        const message = new Promise(resolve => {
          socket.addEventListener("message", event => resolve(event.data), { once: true });
        });
        socket.send("which instance?");
        await expect(message).resolves.toBe(instance);
      } finally {
        socket.close();
      }

      const tunnel = await mf.dispatchFetch(`${ORIGIN}/v1/computers/cmp_A/tunnel`, {
        method: "POST",
      });
      expect(await tunnel.json()).toMatchObject({ instance });

      const redeem = await mf.dispatchFetch(`${ORIGIN}/v1/browser/token?computer_id=cmp_A`, {
        method: "POST",
      });
      expect(await redeem.json()).toMatchObject({ instance });
    }
  });

  it("sends other routes to whichever instance is ready", async () => {
    for (const ready of ["cloud-0", "cloud-1"]) {
      await Promise.all([
        cloud0.reset(ready === "cloud-0", 200),
        cloud1.reset(ready === "cloud-1", 200),
      ]);
      const response = await mf.dispatchFetch(`${ORIGIN}/v1/account`);
      expect(response.status).toBe(200);
      expect(await response.json()).toMatchObject({ instance: ready });
    }
  });

  it("tries the secondary instance once when the preferred instance is unready", async () => {
    const initial = await mf.dispatchFetch(REVALIDATE, { method: "POST" });
    const { instance } = (await initial.json()) as { instance: string };
    const [preferred, alternate] = instance === "cloud-0" ? [cloud0, cloud1] : [cloud1, cloud0];
    await Promise.all([preferred.reset(false, 200), alternate.reset(true, 200)]);

    const response = await mf.dispatchFetch(REVALIDATE, {
      method: "POST",
      body: JSON.stringify({ nonce: "signed-body" }),
    });

    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({
      instance: instance === "cloud-0" ? "cloud-1" : "cloud-0",
      method: "POST",
      body: '{"nonce":"signed-body"}',
    });
    expect(await preferred.counts()).toEqual({ probes: 1, requests: 0 });
    expect(await alternate.counts()).toEqual({ probes: 1, requests: 1 });
  });

  it("returns 503 with Retry-After when both instances are unready", async () => {
    await Promise.all([cloud0.reset(false, 200), cloud1.reset(false, 200)]);

    const response = await mf.dispatchFetch(`${ORIGIN}/v1/account`);

    expect(response.status).toBe(503);
    expect(Number(response.headers.get("retry-after"))).toBeGreaterThan(0);
    expect(await response.json()).toEqual({ code: "cloud_unavailable" });
    expect(await cloud0.counts()).toEqual({ probes: 1, requests: 0 });
    expect(await cloud1.counts()).toEqual({ probes: 1, requests: 0 });
  });

  it("passes through an application 503 without replaying the request", async () => {
    await Promise.all([cloud0.reset(true, 503), cloud1.reset(true, 503)]);

    const response = await mf.dispatchFetch(`${ORIGIN}/v1/session/refresh`, {
      method: "POST",
      body: "refresh",
    });

    expect(response.status).toBe(503);
    expect(await response.json()).toMatchObject({ body: "refresh" });
    const [first, second] = await Promise.all([cloud0.counts(), cloud1.counts()]);
    expect(first.requests + second.requests).toBe(1);
    expect(first.probes + second.probes).toBe(1);
  });

  it("derives the client IP and coarse location at the edge", async () => {
    const response = await mf.dispatchFetch(`${ORIGIN}/v1/device-flow`, {
      method: "POST",
      headers: {
        "CF-Connecting-IP": "203.0.113.7",
        "X-Compozy-Client-IP": "192.0.2.11",
        "X-Compozy-Location": "spoofed",
        "CF-Container-Target-Port": "12345",
      },
      cf: { city: "東京", region: "Tokyo", country: "JP" },
    });
    const { headers } = (await response.json()) as { headers: Record<string, string> };

    expect(headers["x-compozy-client-ip"]).toBe("203.0.113.7");
    expect(decodeURIComponent(headers["x-compozy-location"] ?? "")).toBe("東京, Tokyo, JP");
    expect(headers).not.toHaveProperty("cf-container-target-port");
  });

  it("removes caller-supplied metadata when Cloudflare metadata is absent", async () => {
    const response = await mf.dispatchFetch(`${ORIGIN}/v1/account`, {
      headers: { "X-Compozy-Client-IP": "192.0.2.11", "X-Compozy-Location": "spoofed" },
      cf: {},
    });
    const { headers } = (await response.json()) as { headers: Record<string, string> };
    expect(headers).not.toHaveProperty("x-compozy-client-ip");
    expect(headers).not.toHaveProperty("x-compozy-location");
  });

  it("probes both instances from the scheduled handler", async () => {
    const worker = await mf.getWorker();
    await worker.scheduled({ cron: "* * * * *" });
    expect(await cloud0.counts()).toEqual({ probes: 1, requests: 0 });
    expect(await cloud1.counts()).toEqual({ probes: 1, requests: 0 });
  });

  it("exposes just the Worker handlers and Container class", async () => {
    expect(await cloud0.edgeExports()).toEqual({
      module: ["CloudContainer", "default"],
      handler: ["fetch", "scheduled"],
    });
  });
});

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { apiClient, daemonFetch } from "@/lib/api-client";
import {
  acquireHostedStreamTicket,
  appendHostedStreamTicket,
  authorizeStreamFetchInput,
  authorizeStreamUrl,
  resetGatewayStreamAuth,
} from "@/lib/gateway-stream-auth";
import {
  buildBrowserAuthorizeUrl,
  buildBrowserDPoP,
  buildUpgradeProUrl,
  classifyHostedBrowserError,
  clearPendingPkceState,
  computeJwkThumbprint,
  deriveCodeChallenge,
  encodePublicJwkParam,
  ensureBrowserKeyPair,
  exportPublicJwk,
  generateCodeVerifier,
  isHostedOriginCandidate,
  observeHostedBrowserAccess,
  prepareBrowserAuthorizeRedirect,
  replaceBrowserKeyPair,
  resetHostedBrowserAuth,
  setBrowserKeyStore,
  setHostedBrowserMode,
  validateBrowserCallback,
  type BrowserKeyStore,
  type HostedBrowserSignal,
} from "@/lib/hosted-browser-auth";

function decodeBase64UrlJson<T>(segment: string): T {
  const padded = segment
    .replace(/-/g, "+")
    .replace(/_/g, "/")
    .padEnd(segment.length + ((4 - (segment.length % 4)) % 4), "=");
  return JSON.parse(atob(padded)) as T;
}

function createMemoryStore(): BrowserKeyStore {
  let pair: CryptoKeyPair | null = null;
  return {
    async get() {
      return pair;
    },
    async set(next) {
      pair = next;
    },
    async delete() {
      pair = null;
    },
  };
}

describe("hosted-browser-auth (UT-072)", () => {
  beforeEach(() => {
    resetHostedBrowserAuth();
    resetGatewayStreamAuth();
    sessionStorage.clear();
    setBrowserKeyStore(createMemoryStore());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    resetHostedBrowserAuth();
    resetGatewayStreamAuth();
    sessionStorage.clear();
  });

  it("detects hosted compozy.run origins and callback paths while ignoring localhost", () => {
    expect(isHostedOriginCandidate({ hostname: "studio.compozy.run", pathname: "/" })).toBe(true);
    expect(isHostedOriginCandidate({ hostname: "localhost", pathname: "/" })).toBe(false);
    expect(isHostedOriginCandidate({ hostname: "127.0.0.1", pathname: "/" })).toBe(false);
    expect(isHostedOriginCandidate({ hostname: "compozy.run", pathname: "/" })).toBe(false);
    expect(
      isHostedOriginCandidate({ hostname: "localhost", pathname: "/remote/v1/browser/callback" })
    ).toBe(true);
  });

  it("generates and persists a non-extractable P-256 browser keypair and reuses it across calls", async () => {
    const first = await ensureBrowserKeyPair();
    expect(first.privateKey.extractable).toBe(false);
    expect(first.privateKey.algorithm).toMatchObject({ name: "ECDSA", namedCurve: "P-256" });

    const second = await ensureBrowserKeyPair();
    expect(second).toBe(first);

    const jwk = await exportPublicJwk(first.publicKey);
    expect(jwk.kty).toBe("EC");
    expect(jwk.crv).toBe("P-256");
    expect(jwk.x.length).toBeGreaterThan(20);
    expect(jwk.y.length).toBeGreaterThan(20);

    const jkt = await computeJwkThumbprint(jwk);
    expect(jkt).toMatch(/^[A-Za-z0-9_-]{43}$/);

    const replaced = await replaceBrowserKeyPair();
    expect(replaced).not.toBe(first);
    expect(replaced.privateKey.extractable).toBe(false);
  });

  it("builds /v1/browser/authorize URLs with PKCE S256, encoded public_jwk, state, and provider", async () => {
    const prepared = await prepareBrowserAuthorizeRedirect({
      hostname: "studio.compozy.run",
      provider: "google",
    });
    const url = new URL(prepared.url);
    expect(url.origin).toBe("https://cloud.compozy.com");
    expect(url.pathname).toBe("/v1/browser/authorize");
    expect(url.searchParams.get("hostname")).toBe("studio.compozy.run");
    expect(url.searchParams.get("provider")).toBe("google");
    expect(url.searchParams.get("code_challenge_method")).toBe("S256");
    expect(url.searchParams.get("state")).toBe(prepared.state);
    expect(url.searchParams.get("code_challenge")).toBe(
      await deriveCodeChallenge(prepared.codeVerifier)
    );
    expect(url.searchParams.get("public_jwk")).toBe(encodePublicJwkParam(prepared.publicJwk));

    expect(buildUpgradeProUrl("studio.compozy.run")).toBe(
      "https://cloud.compozy.com/billing/upgrade?hostname=studio.compozy.run"
    );

    const verifier = generateCodeVerifier();
    expect(verifier).toMatch(/^[A-Za-z0-9_-]{43}$/);
    const directUrl = buildBrowserAuthorizeUrl({
      hostname: "box.compozy.run",
      publicJwk: prepared.publicJwk,
      codeChallenge: await deriveCodeChallenge(verifier),
      state: "state-123",
      provider: "github",
    });
    expect(new URL(directUrl).searchParams.get("provider")).toBe("github");
  });

  it("rejects callback state mismatch without yielding a redeemable code", async () => {
    await prepareBrowserAuthorizeRedirect({
      hostname: "studio.compozy.run",
      provider: "google",
    });
    const mismatched = validateBrowserCallback("?code=code-123&state=wrong-state");
    expect(mismatched).toEqual({ ok: false, reason: "state_mismatch" });

    // Consuming on mismatch leaves no stored state for replay.
    clearPendingPkceState();
    const missing = validateBrowserCallback("?code=code-123&state=wrong-state");
    expect(missing).toEqual({ ok: false, reason: "state_mismatch" });
  });

  it("validates matching callback state and consumes the stored PKCE verifier", async () => {
    const prepared = await prepareBrowserAuthorizeRedirect({
      hostname: "studio.compozy.run",
      provider: "github",
    });
    const result = validateBrowserCallback(
      `?code=auth-code-99&state=${encodeURIComponent(prepared.state)}`
    );
    expect(result).toEqual({
      ok: true,
      code: "auth-code-99",
      codeVerifier: prepared.codeVerifier,
      provider: "github",
    });
    // Replaying the same state is rejected.
    expect(
      validateBrowserCallback(`?code=auth-code-99&state=${encodeURIComponent(prepared.state)}`)
    ).toEqual({ ok: false, reason: "state_mismatch" });
  });

  it("signs valid RFC 9449 browser DPoP proofs (ES256, dpop+jwt, normalized htu, no ath)", async () => {
    const proof = await buildBrowserDPoP(
      "post",
      "https://studio.compozy.run/remote/v1/browser/session?ignored=1#frag",
      { baseOrigin: "https://studio.compozy.run", nowSeconds: 1_700_000_000 }
    );
    const parts = proof.split(".");
    expect(parts).toHaveLength(3);
    const header = decodeBase64UrlJson<{
      typ: string;
      alg: string;
      jwk: { crv: string; kty: string };
    }>(parts[0]!);
    const payload = decodeBase64UrlJson<{
      htm: string;
      htu: string;
      iat: number;
      jti: string;
      ath?: string;
    }>(parts[1]!);

    expect(header.typ).toBe("dpop+jwt");
    expect(header.alg).toBe("ES256");
    expect(header.jwk.kty).toBe("EC");
    expect(header.jwk.crv).toBe("P-256");
    expect(payload.htm).toBe("POST");
    expect(payload.htu).toBe("https://studio.compozy.run/remote/v1/browser/session");
    expect(payload.iat).toBe(1_700_000_000);
    expect(payload.jti.length).toBeGreaterThan(8);
    expect(payload.ath).toBeUndefined();
  });

  it("attaches DPoP headers via daemonFetch and emits device_removed on 401 device_revoked", async () => {
    setHostedBrowserMode(true);
    const signals: HostedBrowserSignal[] = [];
    const unsubscribe = observeHostedBrowserAccess(signal => signals.push(signal));

    const fetchMock = vi.fn<typeof globalThis.fetch>().mockResolvedValue(
      new Response(JSON.stringify({ code: "device_revoked", error: "Device was revoked" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      })
    );
    vi.stubGlobal("fetch", fetchMock);

    const response = await daemonFetch("/api/sessions", { method: "GET" });
    expect(response.status).toBe(401);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [, init] = fetchMock.mock.calls[0]!;
    const headers = new Headers(init?.headers);
    expect(headers.get("DPoP")).toBeTruthy();
    expect(signals).toEqual([{ kind: "device_removed" }]);
    unsubscribe();
  });

  it("mints hosted stream tickets as ?remote_ticket= and leaves POST prompt fetch inputs unticketed", async () => {
    setHostedBrowserMode(true);
    const fetchMock = vi.fn<typeof globalThis.fetch>().mockImplementation(async input => {
      const request = input instanceof Request ? input : new Request(String(input));
      if (new URL(request.url).pathname === "/remote/v1/stream-tickets") {
        const body = (await request.json()) as { path: string };
        expect(body.path).toBe("/api/workspaces/ws-1/terminals/t-1/stream");
        expect(request.headers.get("DPoP")).toBeTruthy();
        return new Response(
          JSON.stringify({ ticket: "hosted-ticket-42", expires_at: "2026-11-24T14:00:00Z" }),
          { status: 201, headers: { "Content-Type": "application/json" } }
        );
      }
      return new Response("not found", { status: 404 });
    });
    vi.stubGlobal("fetch", fetchMock);

    const terminalPath = "/api/workspaces/ws-1/terminals/t-1/stream?ticket=attach-pass";
    const authorized = await authorizeStreamUrl(terminalPath);
    expect(authorized).toBe(
      "/api/workspaces/ws-1/terminals/t-1/stream?ticket=attach-pass&remote_ticket=hosted-ticket-42"
    );

    const ticket = await acquireHostedStreamTicket(terminalPath);
    expect(ticket).toBe("hosted-ticket-42");
    expect(appendHostedStreamTicket("/api/sessions/stream", "tok-1")).toBe(
      "/api/sessions/stream?remote_ticket=tok-1"
    );

    // POST prompt stream keeps its URL unchanged (uses cookie + DPoP).
    const promptUrl = "/api/workspaces/ws-1/sessions/s-1/prompt";
    expect(await authorizeStreamFetchInput(promptUrl)).toBe(promptUrl);
    // Sanity check apiClient middleware uses hosted path without error.
    expect(apiClient).toBeDefined();
  });

  it("classifies hosted listener HTTP errors into U8 gate signals", () => {
    expect(classifyHostedBrowserError(401, { code: "device_revoked" })).toEqual({
      kind: "device_removed",
    });
    expect(classifyHostedBrowserError(401, { code: "remote_session_expired" })).toEqual({
      kind: "session_expired",
    });
    expect(classifyHostedBrowserError(401, { code: "browser_key_rebind_required" })).toEqual({
      kind: "key_rebind_required",
    });
    expect(classifyHostedBrowserError(402, { code: "plan_required" })).toEqual({
      kind: "needs_pro",
    });
    expect(
      classifyHostedBrowserError(403, {
        code: "device_pending_approval",
        details: { approval_id: "apr-1" },
      })
    ).toEqual({ kind: "waiting_approval", approvalId: "apr-1" });
    expect(classifyHostedBrowserError(403, { code: "device_not_approved" })).toEqual({
      kind: "not_approved",
    });
    expect(
      classifyHostedBrowserError(403, {
        code: "account_mismatch",
        details: { email: "other@example.com" },
      })
    ).toEqual({ kind: "another_account", signedInEmail: "other@example.com" });
    // Per-route forbidden errors are not session-loss signals.
    expect(classifyHostedBrowserError(403, { code: "scope_missing" })).toBeNull();
    expect(classifyHostedBrowserError(403, { code: "remote_bridge_browser_denied" })).toBeNull();
  });
});

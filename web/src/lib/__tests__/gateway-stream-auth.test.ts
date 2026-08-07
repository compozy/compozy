import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  acquireStreamTicket,
  appendStreamTicket,
  authorizeStreamFetchInput,
  authorizeStreamUrl,
  gatewayStreamAuthMode,
  GatewayStreamAuthError,
  resetGatewayStreamAuth,
} from "@/lib/gateway-stream-auth";
import { observeGatewayAccess } from "@/lib/gateway-access-signal";

function ticketResponse(ticket: string): Response {
  return new Response(JSON.stringify({ ticket, expires_at: "2026-08-07T00:00:00Z" }), {
    status: 201,
    headers: { "Content-Type": "application/json" },
  });
}

function errorResponse(status: number, code?: string): Response {
  return new Response(JSON.stringify({ error: "denied", ...(code ? { code } : {}) }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("gateway stream auth", () => {
  beforeEach(() => {
    resetGatewayStreamAuth();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    resetGatewayStreamAuth();
  });

  it("Should latch local mode when the listener does not register the minting route", async () => {
    const fetchMock = vi.fn().mockResolvedValue(errorResponse(404));
    vi.stubGlobal("fetch", fetchMock);

    expect(await acquireStreamTicket()).toBeNull();
    expect(gatewayStreamAuthMode()).toBe("local");

    expect(await acquireStreamTicket()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("Should leave a local stream URL untouched", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(errorResponse(405)));

    expect(await authorizeStreamUrl("/api/sessions/catalog-stream")).toBe(
      "/api/sessions/catalog-stream"
    );
  });

  it("Should mint a distinct single-use ticket for every remote connection", async () => {
    let issued = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(() => {
        issued += 1;
        return Promise.resolve(ticketResponse(`tkt_${issued}`));
      })
    );

    expect(await authorizeStreamUrl("/api/tasks/t1/stream")).toBe(
      "/api/tasks/t1/stream?ticket=tkt_1"
    );
    expect(await authorizeStreamUrl("/api/tasks/t1/stream")).toBe(
      "/api/tasks/t1/stream?ticket=tkt_2"
    );
    expect(gatewayStreamAuthMode()).toBe("remote");
  });

  it("Should preserve existing query parameters and the fragment when appending a ticket", () => {
    expect(appendStreamTicket("/api/tasks/t1/stream?after_sequence=7", "tkt")).toBe(
      "/api/tasks/t1/stream?after_sequence=7&ticket=tkt"
    );
    expect(appendStreamTicket("/api/logs/stream#tail", "tkt")).toBe(
      "/api/logs/stream?ticket=tkt#tail"
    );
  });

  it("Should percent-encode the ticket so it cannot break out of the query string", () => {
    expect(appendStreamTicket("/api/logs/stream", "a b&c=d")).toBe(
      "/api/logs/stream?ticket=a%20b%26c%3Dd"
    );
  });

  it("Should report an ended session and fail the connection when the mint is unauthenticated", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(errorResponse(401, "gateway_device_unauthenticated"))
    );
    const signals: string[] = [];
    const unsubscribe = observeGatewayAccess(signal => signals.push(signal));

    await expect(acquireStreamTicket()).rejects.toBeInstanceOf(GatewayStreamAuthError);
    await vi.waitFor(() => expect(signals).toEqual(["unauthenticated"]));
    unsubscribe();

    expect(gatewayStreamAuthMode()).toBe("remote");
  });

  it("Should fail the connection without ending the session on a transient mint failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(errorResponse(503)));
    const signals: string[] = [];
    const unsubscribe = observeGatewayAccess(signal => signals.push(signal));

    await expect(acquireStreamTicket()).rejects.toBeInstanceOf(GatewayStreamAuthError);
    unsubscribe();

    expect(signals).toEqual([]);
  });

  it("Should authorize a streaming prompt POST target on a remote session", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(ticketResponse("tkt_prompt")));

    const target = await authorizeStreamFetchInput("/api/workspaces/ws1/sessions/s1/prompt");

    expect(target).toBe("/api/workspaces/ws1/sessions/s1/prompt?ticket=tkt_prompt");
  });

  it("Should return the prompt POST target unchanged on a local session", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(errorResponse(404)));

    const target = await authorizeStreamFetchInput("/api/workspaces/ws1/sessions/s1/prompt");

    expect(target).toBe("/api/workspaces/ws1/sessions/s1/prompt");
  });
});

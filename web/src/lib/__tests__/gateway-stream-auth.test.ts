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

function listenerResponse(tier: "local" | "private" | "public"): Response {
  return new Response(JSON.stringify({}), {
    status: 200,
    headers: {
      "Content-Type": "application/json",
      "X-Compozy-Gateway-Tier": tier,
    },
  });
}

function requestPath(input: RequestInfo | URL): string {
  if (input instanceof Request) return new URL(input.url).pathname;
  if (input instanceof URL) return input.pathname;
  return new URL(String(input)).pathname;
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

  it("Should discover local mode without requesting the remote minting route", async () => {
    const fetchMock = vi.fn().mockResolvedValue(listenerResponse("local"));
    vi.stubGlobal("fetch", fetchMock);

    expect(await acquireStreamTicket()).toBeNull();
    expect(gatewayStreamAuthMode()).toBe("local");

    expect(await acquireStreamTicket()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls.map(([input]) => requestPath(input))).toEqual(["/api/status"]);
  });

  it("Should share listener discovery across simultaneous connections", async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(listenerResponse("local")));
    vi.stubGlobal("fetch", fetchMock);

    expect(await Promise.all(Array.from({ length: 6 }, () => acquireStreamTicket()))).toEqual(
      Array.from({ length: 6 }, () => null)
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("Should mint distinct tickets after shared remote discovery", async () => {
    let issued = 0;
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (requestPath(input) === "/api/status") return Promise.resolve(listenerResponse("private"));
      return Promise.resolve(ticketResponse(`tkt_${++issued}`));
    });
    vi.stubGlobal("fetch", fetchMock);

    const tickets = await Promise.all(Array.from({ length: 6 }, () => acquireStreamTicket()));
    expect(new Set(tickets).size).toBe(6);
    expect(issued).toBe(6);
    expect(
      fetchMock.mock.calls.filter(([input]) => requestPath(input) === "/api/status")
    ).toHaveLength(1);
  });

  it("Should abort one discovery waiter without canceling the other connection", async () => {
    let complete!: (response: Response) => void;
    const fetchMock = vi.fn().mockImplementation(
      () =>
        new Promise<Response>(resolve => {
          complete = resolve;
        })
    );
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();
    const canceled = acquireStreamTicket(controller.signal);
    const surviving = acquireStreamTicket();
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const request = fetchMock.mock.calls[0]?.[0] as Request;
    const rejected = expect(canceled).rejects.toMatchObject({ name: "AbortError" });

    controller.abort();
    await rejected;
    expect(request.signal.aborted).toBe(false);
    complete(listenerResponse("local"));
    await expect(surviving).resolves.toBeNull();
  });

  it("Should cancel abandoned discovery and ignore its late result before retrying", async () => {
    let complete!: (response: Response) => void;
    const fetchMock = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise<Response>(resolve => {
            complete = resolve;
          })
      )
      .mockImplementation(() => Promise.resolve(listenerResponse("local")));
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();
    const canceled = acquireStreamTicket(controller.signal);
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const request = fetchMock.mock.calls[0]?.[0] as Request;
    const rejected = expect(canceled).rejects.toMatchObject({ name: "AbortError" });

    controller.abort();
    await rejected;
    expect(request.signal.aborted).toBe(true);
    await expect(acquireStreamTicket()).resolves.toBeNull();
    complete(listenerResponse("private"));
    await new Promise(resolve => setTimeout(resolve, 0));
    expect(gatewayStreamAuthMode()).toBe("local");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it.each(["http", "network"])(
    "Should retry listener discovery after a shared %s failure",
    async failure => {
      const fetchMock = vi
        .fn()
        .mockImplementationOnce(() =>
          failure === "http"
            ? Promise.resolve(errorResponse(503))
            : Promise.reject(new TypeError("Failed to fetch"))
        )
        .mockImplementation(() => Promise.resolve(listenerResponse("local")));
      vi.stubGlobal("fetch", fetchMock);

      const outcomes = await Promise.allSettled([acquireStreamTicket(), acquireStreamTicket()]);
      expect(outcomes.map(outcome => outcome.status)).toEqual(["rejected", "rejected"]);
      expect(gatewayStreamAuthMode()).toBeUndefined();
      await expect(acquireStreamTicket()).resolves.toBeNull();
      expect(fetchMock).toHaveBeenCalledTimes(2);
    }
  );

  it("Should leave a local stream URL untouched", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(listenerResponse("local")));

    expect(await authorizeStreamUrl("/api/sessions/catalog-stream")).toBe(
      "/api/sessions/catalog-stream"
    );
  });

  it("Should mint a distinct single-use ticket for every remote connection", async () => {
    let issued = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        if (requestPath(input) === "/api/status") {
          return Promise.resolve(listenerResponse("private"));
        }
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
      vi
        .fn()
        .mockResolvedValueOnce(listenerResponse("private"))
        .mockResolvedValueOnce(errorResponse(401, "gateway_device_unauthenticated"))
    );
    const signals: string[] = [];
    const unsubscribe = observeGatewayAccess(signal => signals.push(signal));

    await expect(acquireStreamTicket()).rejects.toBeInstanceOf(GatewayStreamAuthError);
    await vi.waitFor(() => expect(signals).toEqual(["unauthenticated"]));
    unsubscribe();

    expect(gatewayStreamAuthMode()).toBe("remote");
  });

  it("Should fail the connection without ending the session on a transient mint failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(listenerResponse("private"))
        .mockResolvedValueOnce(errorResponse(503))
    );
    const signals: string[] = [];
    const unsubscribe = observeGatewayAccess(signal => signals.push(signal));

    await expect(acquireStreamTicket()).rejects.toBeInstanceOf(GatewayStreamAuthError);
    unsubscribe();

    expect(signals).toEqual([]);
  });

  it("Should reject a successful mint response whose ticket is not a string", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(listenerResponse("private"))
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ ticket: 42, expires_at: "2026-08-07T00:00:00Z" }), {
            status: 201,
            headers: { "Content-Type": "application/json" },
          })
        )
    );

    await expect(acquireStreamTicket()).rejects.toBeInstanceOf(GatewayStreamAuthError);
  });

  it("Should authorize a streaming prompt POST target on a remote session", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(listenerResponse("private"))
        .mockResolvedValueOnce(ticketResponse("tkt_prompt"))
    );

    const target = await authorizeStreamFetchInput("/api/workspaces/ws1/sessions/s1/prompt");

    expect(target).toBe("/api/workspaces/ws1/sessions/s1/prompt?ticket=tkt_prompt");
  });

  it("Should return the prompt POST target unchanged on a local session", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(listenerResponse("local")));

    const target = await authorizeStreamFetchInput("/api/workspaces/ws1/sessions/s1/prompt");

    expect(target).toBe("/api/workspaces/ws1/sessions/s1/prompt");
  });
});

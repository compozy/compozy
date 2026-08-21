// Suite: ticketed WebSocket upgrade
// Invariant: every upgrade attempt carries its own freshly minted single-use ticket on a remote
// gateway session and none on a local one; a failed mint closes so the consumer's own reconnect
// loop takes over. Per-reconnect factory invocation is owned by the window-manager stream suite.
// Owning layer: the shared browser stream transport.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetGatewayStreamAuth } from "@/lib/gateway-stream-auth";
import { createStreamWebSocket } from "@/lib/ticketed-web-socket";

class FakeWebSocket {
  static OPEN = 1;
  static instances: FakeWebSocket[] = [];

  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  closed = false;
  readyState = FakeWebSocket.OPEN;
  send = vi.fn();

  constructor(readonly url: string) {
    FakeWebSocket.instances.push(this);
  }

  close() {
    this.closed = true;
  }
}

function ticketResponse(ticket: string): Response {
  return new Response(JSON.stringify({ ticket, expires_at: "2026-08-07T00:00:00Z" }), {
    status: 201,
    headers: { "Content-Type": "application/json" },
  });
}

function jsonResponse(status: number): Response {
  return new Response(JSON.stringify({ error: "denied" }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function listenerResponse(tier: "local" | "private"): Response {
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
  return new URL(input).pathname;
}

const STREAM_PATH = "/api/workspaces/ws1/window-manager/stream?after_revision=1&client_id=web";

describe("ticketed web socket", () => {
  beforeEach(() => {
    resetGatewayStreamAuth();
    FakeWebSocket.instances = [];
    vi.stubGlobal("WebSocket", FakeWebSocket);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    resetGatewayStreamAuth();
  });

  it("Should upgrade without a ticket on a local session", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(listenerResponse("local")));

    const socket = createStreamWebSocket(STREAM_PATH);
    await vi.waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));

    expect(FakeWebSocket.instances[0].url).toBe(`ws://${window.location.host}${STREAM_PATH}`);
    socket.close();
  });

  it("Should send through the authorized native connection only while it is open", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(listenerResponse("local")));

    const socket = createStreamWebSocket(STREAM_PATH);
    expect(() => socket.send("before-open")).toThrow("The stream WebSocket is not open.");
    await vi.waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
    const native = FakeWebSocket.instances[0];
    native.readyState = 0;
    expect(() => socket.send("connecting")).toThrow("The stream WebSocket is not open.");
    native.readyState = FakeWebSocket.OPEN;

    socket.send("client-result");
    expect(FakeWebSocket.instances[0].send).toHaveBeenCalledWith("client-result");
    socket.close();
    expect(() => socket.send("after-close")).toThrow("The stream WebSocket is not open.");
  });

  it("Should give every upgrade attempt its own freshly minted ticket", async () => {
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

    const first = createStreamWebSocket(STREAM_PATH);
    await vi.waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
    const second = createStreamWebSocket(STREAM_PATH);
    await vi.waitFor(() => expect(FakeWebSocket.instances).toHaveLength(2));

    expect(FakeWebSocket.instances[0].url).toContain("&ticket=tkt_1");
    expect(FakeWebSocket.instances[1].url).toContain("&ticket=tkt_2");

    first.close();
    second.close();
  });

  it("Should close so the consumer reconnect loop takes over when the mint fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(listenerResponse("private"))
        .mockResolvedValueOnce(jsonResponse(503))
    );

    const socket = createStreamWebSocket(STREAM_PATH);
    const closes: CloseEvent[] = [];
    const errors: Event[] = [];
    socket.onclose = event => closes.push(event);
    socket.onerror = event => errors.push(event);

    await vi.waitFor(() => expect(closes).toHaveLength(1));
    expect(errors).toHaveLength(1);
    expect(FakeWebSocket.instances).toHaveLength(0);
  });

  it("Should not open a socket when the consumer closed before authorization resolved", async () => {
    let release: (value: Response) => void = () => {};
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(
        () =>
          new Promise<Response>(resolve => {
            release = resolve;
          })
      )
    );

    const socket = createStreamWebSocket(STREAM_PATH);
    socket.close();
    release(listenerResponse("local"));
    await Promise.resolve();

    expect(FakeWebSocket.instances).toHaveLength(0);
  });
});

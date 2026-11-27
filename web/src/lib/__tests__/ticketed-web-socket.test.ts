// Suite: ticketed WebSocket upgrade
// Invariant: every upgrade attempt carries its own freshly minted single-use ticket on a remote
// browser session and none on a local one; a failed mint closes so the consumer's own reconnect
// loop takes over. Per-reconnect factory invocation is owned by the window-manager stream suite.
// Owning layer: the shared browser stream transport.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createDeferred, createEventRecorder } from "@/test/event-recorder";

import { resetHostedBrowserAuth, setHostedBrowserMode } from "@/lib/hosted-browser-auth";
import { createStreamWebSocket } from "@/lib/ticketed-web-socket";

class FakeWebSocket {
  static OPEN = 1;
  static opened = createEventRecorder<FakeWebSocket>();

  static get instances() {
    return this.opened.events;
  }

  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  closed = false;
  readyState = 0;
  send = vi.fn();

  constructor(readonly url: string) {
    FakeWebSocket.opened.record(this);
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

function requestPath(input: RequestInfo | URL): string {
  if (input instanceof Request) return new URL(input.url).pathname;
  if (input instanceof URL) return input.pathname;
  return new URL(input).pathname;
}

const STREAM_PATH = "/api/workspaces/ws1/window-manager/stream?after_revision=1&client_id=web";

describe("ticketed web socket", () => {
  beforeEach(() => {
    resetHostedBrowserAuth();
    FakeWebSocket.opened = createEventRecorder<FakeWebSocket>();
    vi.stubGlobal("WebSocket", FakeWebSocket);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    resetHostedBrowserAuth();
  });

  it("Should upgrade without a ticket on a local session", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const socket = createStreamWebSocket(STREAM_PATH);
    await FakeWebSocket.opened.waitForCount(1);
    expect(FakeWebSocket.instances).toHaveLength(1);

    expect(FakeWebSocket.instances[0].url).toBe(`ws://${window.location.host}${STREAM_PATH}`);
    expect(fetchMock).not.toHaveBeenCalled();
    socket.close();
  });

  it("Should send through the authorized native connection only while it is open", async () => {
    const socket = createStreamWebSocket(STREAM_PATH);
    expect(() => socket.send("before-open")).toThrow("The stream WebSocket is not open.");
    await FakeWebSocket.opened.waitForCount(1);
    expect(FakeWebSocket.instances).toHaveLength(1);
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
    setHostedBrowserMode(true);
    let issued = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        expect(requestPath(input)).toBe("/remote/v1/stream-tickets");
        issued += 1;
        return Promise.resolve(ticketResponse(`tkt_${issued}`));
      })
    );

    const first = createStreamWebSocket(STREAM_PATH);
    await FakeWebSocket.opened.waitForCount(1);
    expect(FakeWebSocket.instances).toHaveLength(1);
    const second = createStreamWebSocket(STREAM_PATH);
    await FakeWebSocket.opened.waitForCount(2);
    expect(FakeWebSocket.instances).toHaveLength(2);

    expect(FakeWebSocket.instances[0].url).toContain("&remote_ticket=tkt_1");
    expect(FakeWebSocket.instances[1].url).toContain("&remote_ticket=tkt_2");

    first.close();
    second.close();
  });

  it("Should close so the consumer reconnect loop takes over when the mint fails", async () => {
    setHostedBrowserMode(true);
    vi.stubGlobal(
      "fetch",
      vi
        .fn()

        .mockResolvedValueOnce(jsonResponse(503))
    );

    const socket = createStreamWebSocket(STREAM_PATH);
    const closes: CloseEvent[] = [];
    const errors: Event[] = [];
    const closed = createDeferred<void>();
    socket.onclose = event => {
      closes.push(event);
      closed.resolve();
    };
    socket.onerror = event => errors.push(event);

    await closed.promise;
    expect(closes).toHaveLength(1);
    expect(errors).toHaveLength(1);
    expect(FakeWebSocket.instances).toHaveLength(0);
  });

  it("Should not open a socket when the consumer closed before authorization resolved", async () => {
    setHostedBrowserMode(true);
    const mint = createDeferred<Response>();
    const started = createDeferred<void>();
    vi.stubGlobal(
      "fetch",
      vi.fn(() => {
        started.resolve();
        return mint.promise;
      })
    );

    const socket = createStreamWebSocket(STREAM_PATH);
    await started.promise;
    socket.close();
    mint.resolve(ticketResponse("unused"));
    await Promise.resolve();

    expect(FakeWebSocket.instances).toHaveLength(0);
  });
});

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetGatewayStreamAuth } from "@/lib/gateway-stream-auth";
import { createStreamEventSource } from "@/lib/ticketed-event-source";
import { defaultEventSourceFactory } from "@/systems/session/hooks/session-stream-source";

class FakeEventSource {
  static instances: FakeEventSource[] = [];

  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onopen: ((event: Event) => void) | null = null;
  closed = false;

  readonly listeners = new Map<string, Set<EventListener>>();

  constructor(readonly url: string) {
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: EventListener) {
    const existing = this.listeners.get(type);
    if (existing) existing.add(listener);
    else this.listeners.set(type, new Set([listener]));
  }

  removeEventListener(type: string, listener: EventListener) {
    this.listeners.get(type)?.delete(listener);
  }

  close() {
    this.closed = true;
  }

  emit(type: string, event: Event) {
    for (const listener of this.listeners.get(type) ?? []) listener(event);
  }
}

function ticketResponse(ticket: string): Response {
  return new Response(JSON.stringify({ ticket, expires_at: "2026-08-07T00:00:00Z" }), {
    status: 201,
    headers: { "Content-Type": "application/json" },
  });
}

function notFound(): Response {
  return new Response(JSON.stringify({ error: "not found" }), {
    status: 404,
    headers: { "Content-Type": "application/json" },
  });
}

async function settled(): Promise<void> {
  await vi.waitFor(() => expect(FakeEventSource.instances.length).toBeGreaterThan(0));
}

describe("ticketed event source", () => {
  beforeEach(() => {
    resetGatewayStreamAuth();
    FakeEventSource.instances = [];
    vi.stubGlobal("EventSource", FakeEventSource);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    resetGatewayStreamAuth();
  });

  it("Should open a local stream on the bare URL and keep native reconnect", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(notFound()));
    const source = createStreamEventSource("/api/sessions/catalog-stream");
    await settled();

    const native = FakeEventSource.instances[0];
    expect(native.url).toBe("/api/sessions/catalog-stream");

    native.onerror?.(new Event("error"));
    // A local stream is left to the browser's own retry: the socket stays open
    // and no second connection is created.
    expect(native.closed).toBe(false);
    expect(FakeEventSource.instances).toHaveLength(1);

    source.close();
  });

  it("Should open a remote stream with a minted ticket", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(ticketResponse("tkt_1")));
    const source = createStreamEventSource("/api/tasks/t1/stream");
    await settled();

    expect(FakeEventSource.instances[0].url).toBe("/api/tasks/t1/stream?ticket=tkt_1");
    source.close();
  });

  it("Should authenticate the default session stream factory with a fresh ticket", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(ticketResponse("session-ticket")));

    const source = defaultEventSourceFactory("/api/sessions/session-1/stream");
    await settled();

    expect(FakeEventSource.instances[0].url).toBe(
      "/api/sessions/session-1/stream?ticket=session-ticket"
    );
    source.close();
  });

  it("Should reconnect a remote stream with a fresh ticket instead of replaying a spent one", async () => {
    vi.useFakeTimers();
    let issued = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(() => {
        issued += 1;
        return Promise.resolve(ticketResponse(`tkt_${issued}`));
      })
    );

    const source = createStreamEventSource("/api/tasks/t1/stream");
    await vi.waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
    const first = FakeEventSource.instances[0];

    const errors: Event[] = [];
    source.onerror = event => errors.push(event);
    first.onerror?.(new Event("error"));

    // The spent ticket must not be replayed: the socket is torn down.
    expect(errors).toHaveLength(1);
    expect(first.closed).toBe(true);

    await vi.advanceTimersByTimeAsync(600);
    await vi.waitFor(() => expect(FakeEventSource.instances).toHaveLength(2));
    expect(FakeEventSource.instances[1].url).toBe("/api/tasks/t1/stream?ticket=tkt_2");

    source.close();
    vi.useRealTimers();
  });

  it("Should re-attach registered listeners to each reconnected socket", async () => {
    vi.useFakeTimers();
    let issued = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(() => {
        issued += 1;
        return Promise.resolve(ticketResponse(`tkt_${issued}`));
      })
    );

    const source = createStreamEventSource("/api/tasks/t1/stream");
    const received: string[] = [];
    source.addEventListener("task_event", event => {
      received.push((event as MessageEvent<string>).data);
    });
    await vi.waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));

    FakeEventSource.instances[0].emit("task_event", new MessageEvent("task_event", { data: "a" }));
    FakeEventSource.instances[0].onerror?.(new Event("error"));
    await vi.advanceTimersByTimeAsync(600);
    await vi.waitFor(() => expect(FakeEventSource.instances).toHaveLength(2));

    FakeEventSource.instances[1].emit("task_event", new MessageEvent("task_event", { data: "b" }));
    expect(received).toEqual(["a", "b"]);

    source.close();
    vi.useRealTimers();
  });

  it("Should stop reconnecting once the consumer closes the stream", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(ticketResponse("tkt_1")));

    const source = createStreamEventSource("/api/tasks/t1/stream");
    await vi.waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
    FakeEventSource.instances[0].onerror?.(new Event("error"));
    source.close();

    await vi.advanceTimersByTimeAsync(5_000);
    expect(FakeEventSource.instances).toHaveLength(1);
    vi.useRealTimers();
  });

  it("Should surface a failed mint as a stream error and retry on the backoff curve", async () => {
    vi.useFakeTimers();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: "boom" }), {
          status: 503,
          headers: { "Content-Type": "application/json" },
        })
      )
      .mockResolvedValue(ticketResponse("tkt_recovered"));
    vi.stubGlobal("fetch", fetchMock);

    const source = createStreamEventSource("/api/tasks/t1/stream");
    const errors: Event[] = [];
    source.onerror = event => errors.push(event);

    await vi.waitFor(() => expect(errors).toHaveLength(1));
    expect(FakeEventSource.instances).toHaveLength(0);

    await vi.advanceTimersByTimeAsync(600);
    await vi.waitFor(() => expect(FakeEventSource.instances).toHaveLength(1));
    expect(FakeEventSource.instances[0].url).toBe("/api/tasks/t1/stream?ticket=tkt_recovered");

    source.close();
    vi.useRealTimers();
  });

  it("Should forward open and message events to the consumer", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(ticketResponse("tkt_1")));
    const source = createStreamEventSource("/api/tasks/t1/stream");
    await settled();

    const opened: Event[] = [];
    const messages: string[] = [];
    source.onopen = event => opened.push(event);
    source.onmessage = event => messages.push(event.data as string);

    FakeEventSource.instances[0].onopen?.(new Event("open"));
    FakeEventSource.instances[0].onmessage?.(new MessageEvent("message", { data: "frame" }));

    expect(opened).toHaveLength(1);
    expect(messages).toEqual(["frame"]);
    source.close();
  });
});

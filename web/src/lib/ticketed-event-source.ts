import { acquireStreamTicket, appendStreamTicket } from "./gateway-stream-auth";
import { WebSocketEventSource } from "./web-socket-event-source";

/**
 * The union of the `EventSource` surface the app's stream consumers rely on.
 * Every consumer's own source interface is structurally satisfied by this one,
 * so hooks keep their injectable-factory props and their test doubles.
 */
export interface StreamEventSource {
  addEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  removeEventListener: (type: string, listener: EventListenerOrEventListenerObject) => void;
  close: () => void;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
  onopen: ((event: Event) => void) | null;
}

interface StreamEventSourceOptions {
  /** Keep document-wide streams out of the browser's HTTP/1.1 request pool. */
  transport?: "websocket";
  /** Re-seed a freshly ticketed socket from the last durable SSE event id. */
  resumeWithLastEventId?: boolean;
}

const RECONNECT_BASE_MS = 500;
const RECONNECT_MAX_MS = 8_000;
const RECONNECT_MAX_EXPONENT = 4;
const RECONNECT_STABLE_MS = RECONNECT_MAX_MS;

/**
 * Opens a live event stream for the page's own origin.
 *
 * On a local same-origin session this is a native `EventSource` with its native
 * reconnect behaviour left untouched. On a remote gateway session the ticket is
 * single-use, so native reconnect would replay a spent credential and be
 * rejected: the facade closes the socket on error and reopens it with a freshly
 * minted ticket instead. Consumers see the same `open` / `error` sequence
 * either way. Document-wide streams opt into WebSocket framing and use this
 * same ticket, cursor, and backoff lifecycle on local and remote listeners.
 */
export function createStreamEventSource(
  url: string,
  options: StreamEventSourceOptions = {}
): StreamEventSource {
  return new TicketedEventSource(url, options);
}

class TicketedEventSource implements StreamEventSource {
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onopen: ((event: Event) => void) | null = null;

  private readonly listeners = new Map<string, Set<EventListenerOrEventListenerObject>>();
  private readonly forwarders = new Map<string, EventListener>();
  private native: (StreamEventSource & { readonly url: string }) | null = null;
  private attachedTypes = new Set<string>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private stableTimer: ReturnType<typeof setTimeout> | null = null;
  private controller: AbortController | null = null;
  private attempt = 0;
  private closed = false;
  private lastEventId = "";

  constructor(
    private readonly url: string,
    private readonly options: StreamEventSourceOptions
  ) {
    void this.connect();
  }

  addEventListener(type: string, listener: EventListenerOrEventListenerObject): void {
    const existing = this.listeners.get(type);
    if (existing) {
      existing.add(listener);
    } else {
      this.listeners.set(type, new Set([listener]));
    }
    if (this.native && !this.attachedTypes.has(type)) {
      this.attachedTypes.add(type);
      this.native.addEventListener(type, this.forwarderFor(type));
    }
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject): void {
    this.listeners.get(type)?.delete(listener);
  }

  close(): void {
    this.closed = true;
    this.clearReconnect();
    this.controller?.abort();
    this.controller = null;
    this.teardownNative();
    this.listeners.clear();
  }

  private forwarderFor(type: string): EventListener {
    const existing = this.forwarders.get(type);
    if (existing) return existing;
    const forwarder: EventListener = event => {
      this.observeEventId(event);
      this.dispatch(type, event);
    };
    this.forwarders.set(type, forwarder);
    return forwarder;
  }

  private dispatch(type: string, event: Event): void {
    const registered = this.listeners.get(type);
    if (!registered) return;
    for (const listener of Array.from(registered)) {
      if (typeof listener === "function") {
        listener.call(this, event);
      } else {
        listener.handleEvent(event);
      }
    }
  }

  private async connect(): Promise<void> {
    if (this.closed) return;
    if (
      this.options.transport === "websocket"
        ? typeof WebSocket === "undefined" || typeof window === "undefined"
        : typeof EventSource === "undefined"
    )
      return;
    const controller = new AbortController();
    this.controller = controller;
    let authorizedUrl: string;
    try {
      const ticket = await acquireStreamTicket(controller.signal);
      const resumeUrl =
        this.options.resumeWithLastEventId && this.lastEventId !== ""
          ? appendEventCursor(
              this.url,
              this.lastEventId,
              this.options.transport === "websocket" ? "last_event_id" : "after_sequence"
            )
          : this.url;
      authorizedUrl = ticket === null ? resumeUrl : appendStreamTicket(resumeUrl, ticket);
    } catch {
      // A failed mint is indistinguishable from a dropped stream to consumers:
      // report the error and retry on the same backoff curve. An ended session
      // is reported separately by the access signal, not by this path.
      this.handleFailure();
      return;
    }
    if (this.closed || controller.signal.aborted) return;

    const native =
      this.options.transport === "websocket"
        ? new WebSocketEventSource(authorizedUrl)
        : new EventSource(authorizedUrl);
    this.native = native;
    this.attachedTypes = new Set();
    native.onmessage = event => {
      this.observeEventId(event);
      this.onmessage?.(event);
    };
    native.onopen = event => {
      this.scheduleStableReset(native);
      this.onopen?.(event);
    };
    native.onerror = event => {
      this.onerror?.(event);
      this.handleNativeError(native);
    };
    for (const type of this.listeners.keys()) {
      this.attachedTypes.add(type);
      native.addEventListener(type, this.forwarderFor(type));
    }
  }

  /**
   * A local stream keeps the browser's own retry. A remote stream cannot: the
   * ticket in the current URL has already been consumed, so the socket is torn
   * down and reopened with a new one. WebSockets also reconnect here because
   * they have no native retry loop.
   */
  private handleNativeError(native: StreamEventSource & { readonly url: string }): void {
    if (this.closed || this.native !== native) return;
    if (this.options.transport !== "websocket" && !isTicketedUrl(native.url)) return;
    this.teardownNative();
    this.scheduleReconnect();
  }

  private handleFailure(): void {
    if (this.closed) return;
    this.onerror?.(new Event("error"));
    this.dispatch("error", new Event("error"));
    this.scheduleReconnect();
  }

  private scheduleReconnect(): void {
    if (this.closed || this.reconnectTimer !== null) return;
    const delay = Math.min(
      RECONNECT_MAX_MS,
      RECONNECT_BASE_MS * 2 ** Math.min(this.attempt, RECONNECT_MAX_EXPONENT)
    );
    this.attempt += 1;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      void this.connect();
    }, delay);
  }

  private clearReconnect(): void {
    if (this.reconnectTimer === null) return;
    clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
  }

  private scheduleStableReset(native: StreamEventSource): void {
    this.clearStableReset();
    this.stableTimer = setTimeout(() => {
      this.stableTimer = null;
      if (!this.closed && this.native === native) {
        this.attempt = 0;
      }
    }, RECONNECT_STABLE_MS);
  }

  private clearStableReset(): void {
    if (this.stableTimer === null) return;
    clearTimeout(this.stableTimer);
    this.stableTimer = null;
  }

  private teardownNative(): void {
    this.clearStableReset();
    const native = this.native;
    if (!native) return;
    native.onmessage = null;
    native.onerror = null;
    native.onopen = null;
    for (const type of this.attachedTypes) {
      native.removeEventListener(type, this.forwarderFor(type));
    }
    this.attachedTypes = new Set();
    this.native = null;
    native.close();
  }

  private observeEventId(event: Event): void {
    if (!(event instanceof MessageEvent)) return;
    const eventId = event.lastEventId.trim();
    if (eventId !== "") this.lastEventId = eventId;
  }
}

function isTicketedUrl(url: string): boolean {
  return url.includes("ticket=");
}

function appendEventCursor(url: string, cursor: string, parameter: string): string {
  const absolute = /^[a-z][a-z\d+.-]*:/iu.test(url);
  const parsed = new URL(url, "http://compozy.local");
  parsed.searchParams.set(parameter, cursor);
  return absolute ? parsed.toString() : `${parsed.pathname}${parsed.search}${parsed.hash}`;
}

import { acquireStreamTicket, appendStreamTicket } from "./gateway-stream-auth";

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

const RECONNECT_BASE_MS = 500;
const RECONNECT_MAX_MS = 8_000;
const RECONNECT_MAX_EXPONENT = 4;
const RECONNECT_STABLE_MS = RECONNECT_MAX_MS;

/**
 * Opens a live SSE stream for the page's own origin.
 *
 * On a local same-origin session this is a native `EventSource` with its native
 * reconnect behaviour left untouched. On a remote gateway session the ticket is
 * single-use, so native reconnect would replay a spent credential and be
 * rejected: the facade closes the socket on error and reopens it with a freshly
 * minted ticket instead. Consumers see the same `open` / `error` sequence
 * either way.
 */
export function createStreamEventSource(url: string): StreamEventSource {
  return new TicketedEventSource(url);
}

class TicketedEventSource implements StreamEventSource {
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onopen: ((event: Event) => void) | null = null;

  private readonly listeners = new Map<string, Set<EventListenerOrEventListenerObject>>();
  private readonly forwarders = new Map<string, EventListener>();
  private native: EventSource | null = null;
  private attachedTypes = new Set<string>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private stableTimer: ReturnType<typeof setTimeout> | null = null;
  private controller: AbortController | null = null;
  private attempt = 0;
  private closed = false;

  constructor(private readonly url: string) {
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
    const forwarder: EventListener = event => this.dispatch(type, event);
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
    if (this.closed || typeof EventSource === "undefined") return;
    const controller = new AbortController();
    this.controller = controller;
    let authorizedUrl: string;
    try {
      const ticket = await acquireStreamTicket(controller.signal);
      authorizedUrl = ticket === null ? this.url : appendStreamTicket(this.url, ticket);
    } catch {
      // A failed mint is indistinguishable from a dropped stream to consumers:
      // report the error and retry on the same backoff curve. An ended session
      // is reported separately by the access signal, not by this path.
      this.handleFailure();
      return;
    }
    if (this.closed || controller.signal.aborted) return;

    const native = new EventSource(authorizedUrl);
    this.native = native;
    this.attachedTypes = new Set();
    native.onmessage = event => this.onmessage?.(event);
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
   * down and reopened with a new one.
   */
  private handleNativeError(native: EventSource): void {
    if (this.closed || this.native !== native) return;
    if (!isTicketedUrl(native.url)) return;
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

  private scheduleStableReset(native: EventSource): void {
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
}

function isTicketedUrl(url: string): boolean {
  return url.includes("ticket=");
}

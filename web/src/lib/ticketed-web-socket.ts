import { acquireStreamTicket, appendStreamTicket } from "./gateway-stream-auth";

/** The `WebSocket` surface the window-manager stream consumes. */
export interface StreamWebSocket {
  close: () => void;
  send: (data: string) => void;
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

/**
 * Opens a live WebSocket upgrade for the page's own origin, minting a fresh
 * single-use ticket first on a remote gateway session.
 *
 * One instance is one connection attempt: the window-manager hook already
 * reconnects by calling its socket factory again, so every upgrade — the first
 * and every retry — gets its own ticket without a second retry loop here. A
 * failed mint is surfaced as a close so that existing loop takes over.
 */
export function createStreamWebSocket(path: string): StreamWebSocket {
  return new TicketedWebSocket(path);
}

class TicketedWebSocket implements StreamWebSocket {
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  private native: WebSocket | null = null;
  private readonly controller = new AbortController();
  private closed = false;

  constructor(private readonly path: string) {
    void this.connect();
  }

  close(): void {
    this.closed = true;
    this.controller.abort();
    const native = this.native;
    this.native = null;
    if (!native) return;
    native.onopen = null;
    native.onmessage = null;
    native.onclose = null;
    native.onerror = null;
    native.close();
  }

  send(data: string): void {
    if (this.closed || this.native?.readyState !== WebSocket.OPEN) {
      throw new Error("The stream WebSocket is not open.");
    }
    this.native.send(data);
  }

  private async connect(): Promise<void> {
    if (this.closed || typeof WebSocket === "undefined" || typeof window === "undefined") return;
    let authorizedPath: string;
    try {
      const ticket = await acquireStreamTicket(this.controller.signal);
      authorizedPath = ticket === null ? this.path : appendStreamTicket(this.path, ticket);
    } catch {
      this.reportFailure();
      return;
    }
    if (this.closed) return;

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const native = new WebSocket(`${protocol}//${window.location.host}${authorizedPath}`);
    this.native = native;
    native.onopen = event => this.onopen?.(event);
    native.onmessage = event => this.onmessage?.(event);
    native.onerror = event => this.onerror?.(event);
    native.onclose = event => this.onclose?.(event);
  }

  private reportFailure(): void {
    if (this.closed) return;
    this.onerror?.(new Event("error"));
    this.onclose?.(buildCloseEvent());
  }
}

function buildCloseEvent(): CloseEvent {
  if (typeof CloseEvent === "function") return new CloseEvent("close");
  return new Event("close") as CloseEvent;
}

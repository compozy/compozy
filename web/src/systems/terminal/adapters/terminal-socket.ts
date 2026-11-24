/**
 * The terminal stream socket.
 *
 * Local and remote (`/c/{computer_id}/...`) listeners use the terminal's own
 * `?ticket=` attach pass directly. Hosted browser origins additionally mint a
 * single-use `?remote_ticket=` via `POST /remote/v1/stream-tickets`, keeping
 * the two parameters distinct so the hosted listener authenticates the upgrade
 * without colliding with `?ticket=`.
 */

import { acquireHostedStreamTicket, appendHostedStreamTicket } from "@/lib/gateway-stream-auth";
import { isHostedBrowserMode } from "@/lib/hosted-browser-auth";
import { TERMINAL_SUBPROTOCOL, type TerminalFrameBytes } from "../lib/terminal-wire";

export interface TerminalSocket {
  close(code?: number, reason?: string): void;
  send(data: TerminalFrameBytes): void;
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

export type TerminalSocketFactory = (path: string) => TerminalSocket;

class HostedTerminalSocket implements TerminalSocket {
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

  close(code?: number, reason?: string): void {
    this.closed = true;
    this.controller.abort();
    const native = this.native;
    this.native = null;
    if (!native) return;
    native.onopen = null;
    native.onmessage = null;
    native.onclose = null;
    native.onerror = null;
    native.close(code, reason);
  }

  send(data: TerminalFrameBytes): void {
    if (this.closed || this.native?.readyState !== WebSocket.OPEN) return;
    this.native.send(data);
  }

  private async connect(): Promise<void> {
    if (this.closed || typeof WebSocket === "undefined" || typeof window === "undefined") return;
    let authorizedPath: string;
    try {
      const ticket = await acquireHostedStreamTicket(this.path, this.controller.signal);
      authorizedPath = appendHostedStreamTicket(this.path, ticket);
    } catch {
      if (this.closed) return;
      this.onerror?.(new Event("error"));
      const closeEvent =
        typeof CloseEvent === "function"
          ? new CloseEvent("close")
          : (new Event("close") as CloseEvent);
      this.onclose?.(closeEvent);
      return;
    }
    if (this.closed) return;

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const native = new WebSocket(
      `${protocol}//${window.location.host}${authorizedPath}`,
      TERMINAL_SUBPROTOCOL
    );
    native.binaryType = "arraybuffer";
    this.native = native;
    native.onopen = event => this.onopen?.(event);
    native.onmessage = event => this.onmessage?.(event as MessageEvent<unknown>);
    native.onerror = event => this.onerror?.(event);
    native.onclose = event => this.onclose?.(event);
  }
}

export function createTerminalSocket(path: string): TerminalSocket {
  if (isHostedBrowserMode()) {
    return new HostedTerminalSocket(path);
  }
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const socket = new WebSocket(`${protocol}//${window.location.host}${path}`, TERMINAL_SUBPROTOCOL);
  socket.binaryType = "arraybuffer";
  return {
    close: (code, reason) => socket.close(code, reason),
    send: data => socket.send(data),
    get onopen() {
      return socket.onopen;
    },
    set onopen(handler) {
      socket.onopen = handler;
    },
    get onmessage() {
      return socket.onmessage as ((event: MessageEvent<unknown>) => void) | null;
    },
    set onmessage(handler) {
      socket.onmessage = handler as ((event: MessageEvent) => void) | null;
    },
    get onclose() {
      return socket.onclose;
    },
    set onclose(handler) {
      socket.onclose = handler;
    },
    get onerror() {
      return socket.onerror;
    },
    set onerror(handler) {
      socket.onerror = handler;
    },
  };
}

/**
 * The terminal stream socket.
 *
 * Terminals are a local surface in v1 — remote/gateway terminal access is an
 * explicit non-goal — so this deliberately does not go through the gateway
 * stream-ticket path: that helper spends the same `?ticket=` parameter the
 * terminal attach pass already owns, and a second authority on one query
 * parameter is how a stream ends up authorised as the wrong thing.
 */

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

export function createTerminalSocket(path: string): TerminalSocket {
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

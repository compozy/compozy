import { createStreamWebSocket } from "@/lib/ticketed-web-socket";

/** The slice of a WebSocket the window-manager stream drives (tests inject fakes). */
export interface WindowManagerSocket {
  close: () => void;
  send: (data: string) => void;
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent<unknown>) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

export type WindowManagerSocketFactory = (url: string) => WindowManagerSocket;

export function browserWindowManagerSocket(url: string): WindowManagerSocket {
  return createStreamWebSocket(url);
}

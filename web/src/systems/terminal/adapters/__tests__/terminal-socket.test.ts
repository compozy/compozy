import { afterEach, describe, expect, it, vi } from "vitest";

import { createTerminalSocket } from "../terminal-socket";
import { TERMINAL_SUBPROTOCOL } from "../../lib/terminal-wire";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("createTerminalSocket", () => {
  it("Should bind the browser socket URL, protocol, binary mode, and handlers", () => {
    const sends: unknown[] = [];
    const closes: unknown[][] = [];
    const opened: Array<{ url: string; protocol: string }> = [];
    class FakeWebSocket {
      binaryType = "blob";
      onopen: ((event: Event) => void) | null = null;
      onmessage: ((event: MessageEvent) => void) | null = null;
      onclose: ((event: CloseEvent) => void) | null = null;
      onerror: ((event: Event) => void) | null = null;

      constructor(url: string, protocol: string) {
        opened.push({ url, protocol });
      }

      send(data: unknown) {
        sends.push(data);
      }

      close(...args: unknown[]) {
        closes.push(args);
      }
    }
    vi.stubGlobal("WebSocket", FakeWebSocket);

    const socket = createTerminalSocket("/api/terminal/stream?ticket=tkt-1");
    const onOpen = vi.fn();
    socket.onopen = onOpen;
    const frame = new Uint8Array([1, 2, 3]);
    socket.send(frame);
    socket.close(1000, "done");

    expect(opened).toEqual([
      {
        url: `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/api/terminal/stream?ticket=tkt-1`,
        protocol: TERMINAL_SUBPROTOCOL,
      },
    ]);
    expect((socket as unknown as { onopen: typeof onOpen }).onopen).toBe(onOpen);
    expect(sends).toEqual([frame]);
    expect(closes).toEqual([[1000, "done"]]);
  });
});

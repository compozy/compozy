import { vi } from "vitest";

import type { TerminalSocket } from "../../adapters/terminal-socket";
import type { TerminalStreamSink } from "../terminal-protocol-client";
import { TERMINAL_SERVER_OP } from "../terminal-wire";

/**
 * A scripted terminal socket and emulator sink.
 *
 * Only the two real I/O boundaries are replaced — the WebSocket and the
 * emulator. Frame encoding, decoding, credit accounting and the resync sequence
 * all run for real, so the assertions are about the client's behaviour rather
 * than about the harness.
 */

export interface FakeTerminalSocket extends TerminalSocket {
  readonly sent: Uint8Array[];
  readonly path: string;
  closed: boolean;
  open(): void;
  deliver(frame: Uint8Array): void;
  drop(): void;
}

export function createFakeSocketFactory(): {
  factory: (path: string) => TerminalSocket;
  sockets: FakeTerminalSocket[];
  last(): FakeTerminalSocket;
} {
  const sockets: FakeTerminalSocket[] = [];
  return {
    sockets,
    last: () => {
      const socket = sockets.at(-1);
      if (!socket) throw new Error("no socket was opened");
      return socket;
    },
    factory: (path: string) => {
      const socket: FakeTerminalSocket = {
        path,
        sent: [],
        closed: false,
        onopen: null,
        onmessage: null,
        onclose: null,
        onerror: null,
        close: () => {
          socket.closed = true;
        },
        send: data => {
          socket.sent.push(data);
        },
        open: () => socket.onopen?.(new Event("open")),
        deliver: frame => {
          const buffer = frame.buffer.slice(
            frame.byteOffset,
            frame.byteOffset + frame.byteLength
          ) as ArrayBuffer;
          socket.onmessage?.({ data: buffer } as MessageEvent<unknown>);
        },
        drop: () => socket.onclose?.(new CloseEvent("close")),
      };
      sockets.push(socket);
      return socket;
    },
  };
}

export interface FakeSink extends TerminalStreamSink {
  readonly parsed: string[];
  readonly dimensions: Array<{ cols: number; rows: number }>;
  resets: number;
  /** Releases the pending parse for the write at `index`. */
  completeWrite(index: number): void;
  pendingWrites(): number;
}

/** A sink whose parse completion is under the test's control. */
export function createFakeSink(options: { autoParse?: boolean } = {}): FakeSink {
  const pending: Array<() => void> = [];
  const decoder = new TextDecoder();
  const sink: FakeSink = {
    parsed: [],
    dimensions: [],
    resets: 0,
    write: data =>
      new Promise<void>(resolve => {
        const text = typeof data === "string" ? data : decoder.decode(data);
        const settle = () => {
          sink.parsed.push(text);
          resolve();
        };
        if (options.autoParse === false) {
          pending.push(settle);
          return;
        }
        settle();
      }),
    reset: () => {
      sink.resets += 1;
    },
    applyDimensions: dimensions => {
      sink.dimensions.push(dimensions);
    },
    completeWrite: index => pending[index]?.(),
    pendingWrites: () => pending.length,
  };
  return sink;
}

const encoder = new TextEncoder();

export function serverControlFrame(op: number, payload: unknown): Uint8Array {
  const body = encoder.encode(JSON.stringify(payload));
  const frame = new Uint8Array(body.byteLength + 1);
  frame[0] = op;
  frame.set(body, 1);
  return frame;
}

export function serverOutputFrame(seq: number, text: string): Uint8Array {
  const body = encoder.encode(text);
  const frame = new Uint8Array(body.byteLength + 9);
  frame[0] = TERMINAL_SERVER_OP.output;
  new DataView(frame.buffer).setBigUint64(1, BigInt(seq), false);
  frame.set(body, 9);
  return frame;
}

export function attachedFrame(overrides: Partial<Record<string, unknown>> = {}): Uint8Array {
  return serverControlFrame(TERMINAL_SERVER_OP.attached, {
    seq: 0,
    truncated: false,
    cols: 96,
    rows: 28,
    lease: "human_owned",
    mode: "pty",
    ...overrides,
  });
}

export interface TerminalFetchStub {
  calls: string[];
  /** Releases a screen read that was held open, with the given body. */
  resolveScreen(body: { content: string; seq: number }): void;
  /** How many screen reads are waiting to be released. */
  pendingScreenReads(): number;
  restore(): void;
}

/**
 * Stubs the two REST calls the client makes, without touching the adapter.
 *
 * `deferScreen` holds the snapshot read open so a test can deliver live frames
 * into the window between the gap and the snapshot — the exact window where a
 * naive resync loses or interleaves bytes.
 */
export function stubTerminalFetch(handlers: {
  ticket?: () => unknown;
  screen?: () => unknown;
  deferScreen?: boolean;
}): TerminalFetchStub {
  const calls: string[] = [];
  const pendingScreens: Array<(body: unknown) => void> = [];
  const original = globalThis.fetch;
  const defaultScreen = {
    content: "current screen",
    seq: 4096,
    truncated: false,
    busy: false,
    untrusted: true,
  };
  const stub = vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    calls.push(url);
    if (url.includes("/attach-ticket")) {
      const body = handlers.ticket?.() ?? {
        ticket: `tkt-${calls.length}`,
        expires_at: "2026-08-25T12:00:30Z",
      };
      return jsonResponse(body, 201);
    }
    if (url.includes("/read")) {
      if (handlers.deferScreen) {
        const body = await new Promise<unknown>(resolve => pendingScreens.push(resolve));
        return jsonResponse(body, 200);
      }
      return jsonResponse(handlers.screen?.() ?? defaultScreen, 200);
    }
    return jsonResponse({ error: { code: "not_stubbed", message: url } }, 500);
  });
  globalThis.fetch = stub as unknown as typeof globalThis.fetch;
  return {
    calls,
    pendingScreenReads: () => pendingScreens.length,
    resolveScreen: body => {
      const resolve = pendingScreens.shift();
      resolve?.({ ...defaultScreen, ...body });
    },
    restore: () => {
      globalThis.fetch = original;
    },
  };
}

function jsonResponse(body: unknown, status: number): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

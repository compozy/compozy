import { act, render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement } from "react";
import { vi } from "vitest";

import type { TerminalEngine } from "@compozy/ui";

import type { TerminalSocket } from "../../adapters/terminal-socket";
import { TerminalStoreProvider } from "../../contexts/terminal-store-context";
import type { TerminalWindowActions } from "../terminal-window-app";

/**
 * A window rendered without its two live boundaries.
 *
 * The emulator and the socket are the only things replaced; the store, the lease
 * projection and every component below the window run for real.
 */
export function renderTerminalWindow(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(ui, {
    wrapper: ({ children }) => (
      <QueryClientProvider client={queryClient}>
        <TerminalStoreProvider>{children}</TerminalStoreProvider>
      </QueryClientProvider>
    ),
  });
}

/** A socket that never opens or emits frames — the window still renders. */
export const silentSocketFactory = () => {
  const socket: TerminalSocket = {
    close: () => undefined,
    send: () => undefined,
    onopen: null,
    onmessage: null,
    onclose: null,
    onerror: null,
  };
  return socket;
};

/** One frame this client put on the wire, decoded back into its meaning. */
export interface SentTerminalFrame {
  op: number;
  payload: unknown;
}

/**
 * A socket that records what the client sends and lets a test speak back.
 *
 * Control gestures are wire frames, not callbacks, so the only honest way to
 * assert "Take control sent exactly one TAKEOVER" is to read the socket.
 */
export function recordingSocketFactory() {
  const sent: SentTerminalFrame[] = [];
  const sockets: TerminalSocket[] = [];
  const decoder = new TextDecoder();
  const waitForListening = async (minimumConnections?: number) => {
    await waitFor(() => {
      if (
        (minimumConnections !== undefined && sockets.length < minimumConnections) ||
        !sockets.some(socket => socket.onmessage !== null)
      ) {
        throw new Error(
          minimumConnections === undefined
            ? "no socket is listening yet"
            : `waiting for terminal connection ${minimumConnections}`
        );
      }
    });
  };
  const factory = () => {
    const socket: TerminalSocket = {
      close: () => undefined,
      send: data => {
        const bytes = data instanceof Uint8Array ? data : new Uint8Array(data as ArrayBuffer);
        const body = decoder.decode(bytes.subarray(1));
        let payload: unknown = null;
        try {
          payload = body === "" ? null : JSON.parse(body);
        } catch {
          payload = body;
        }
        sent.push({ op: bytes[0], payload });
      },
      onopen: null,
      onmessage: null,
      onclose: null,
      onerror: null,
    };
    sockets.push(socket);
    return socket;
  };
  return {
    factory,
    sent,
    /** Resolves once the client has opened a socket and can be spoken to. */
    ready: async () => {
      await waitForListening();
      await act(async () => {
        for (const socket of sockets) socket.onopen?.({} as Event);
        await Promise.resolve();
      });
    },
    /** Number of passes minted so a reconnect test can wait for a fresh one. */
    connectionCount: () => sockets.length,
    /** Waits for a specific pass count, then opens the newest pass. */
    readyForConnectionCount: async (count: number) => {
      await waitForListening(count);
      await act(async () => {
        sockets.at(-1)?.onopen?.({} as Event);
        await Promise.resolve();
      });
    },
    /** Frames the client sent with this opcode, in order. */
    sentWithOp: (op: number) => sent.filter(frame => frame.op === op),
    /**
     * Delivers a control frame from the daemon.
     *
     * Sent to every socket still listening rather than only the newest: a mode
     * change replaces the connection, and a test should not have to know which
     * attachment happens to be live at the moment it speaks.
     */
    deliver: async (op: number, payload: unknown) => {
      // The attachment is created in an effect after a ticket is minted, so a
      // test speaks as soon as someone is listening rather than guessing when.
      await waitForListening();
      const listening = sockets.filter(socket => socket.onmessage !== null);
      const body = new TextEncoder().encode(JSON.stringify(payload));
      const frame = new Uint8Array(body.byteLength + 1);
      frame[0] = op;
      frame.set(body, 1);
      await act(async () => {
        for (const socket of listening) {
          socket.onmessage?.({ data: frame.buffer } as MessageEvent<unknown>);
        }
        await Promise.resolve();
      });
    },
    /** Drops the newest live connection the way the browser would. */
    drop: () =>
      act(async () => {
        sockets.at(-1)?.onclose?.(new CloseEvent("close"));
        await Promise.resolve();
      }),
    open: () =>
      act(async () => {
        sockets.at(-1)?.onopen?.({} as Event);
        await Promise.resolve();
      }),
  };
}

/** A stand-in emulator: enough surface for the view, no canvas, no GPU. */
export const stubEngineLoader = (): Promise<TerminalEngine> =>
  Promise.resolve({
    createTerminal: options =>
      ({
        options: { ...options },
        rows: 24,
        cols: 80,
        loadAddon: () => undefined,
        open: () => undefined,
        write: (_data: unknown, callback?: () => void) => callback?.(),
        resize: () => undefined,
        refresh: () => undefined,
        reset: () => undefined,
        focus: () => undefined,
        dispose: () => undefined,
        getSelection: () => "",
        getSelectionPosition: () => undefined,
        onData: () => ({ dispose: () => undefined }),
        onSelectionChange: () => ({ dispose: () => undefined }),
      }) as unknown as ReturnType<TerminalEngine["createTerminal"]>,
    createFitAddon: () => ({
      activate: () => undefined,
      dispose: () => undefined,
      proposeDimensions: () => ({ cols: 96, rows: 28 }),
    }),
    createRendererAddon: () => ({
      activate: () => undefined,
      dispose: () => undefined,
      onContextLoss: () => ({ dispose: () => undefined }),
    }),
  });

/** Every action stubbed, so a test can assert only the one it exercises. */
export function stubWindowActions(
  overrides: Partial<TerminalWindowActions> = {}
): TerminalWindowActions {
  return {
    onOpenTerminal: vi.fn(),
    onCloseTerminal: vi.fn(),
    onStop: vi.fn(),
    onStopRecording: vi.fn(),
    onAnswerInputRequest: vi.fn(),
    onRejectInputRequest: vi.fn(),
    onSendSelection: vi.fn(),
    onCopySelection: vi.fn(),
    onChooseSession: vi.fn(),
    onStartSession: vi.fn(),
    hasActiveSession: true,
    ...overrides,
  };
}

/** Keeps the attach path from reaching the network during a render test. */
export function stubTerminalTicketFetch(): () => void {
  const original = globalThis.fetch;
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url =
      typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    if (url.includes("/read")) {
      const signal = init?.signal ?? (input instanceof Request ? input.signal : undefined);
      return new Promise<Response>((_resolve, reject) => {
        signal?.addEventListener(
          "abort",
          () => reject(new DOMException("The operation was aborted.", "AbortError")),
          { once: true }
        );
      });
    }
    return Response.json(
      { ticket: "tkt-test", expires_at: "2026-08-25T12:00:30Z" },
      { status: 201 }
    );
  }) as unknown as typeof globalThis.fetch;
  return () => {
    globalThis.fetch = original;
  };
}

// Suite: Remote terminal REST, stream path, error envelope, and reconnect resume (US-011)
// Invariant: every terminal REST call, attach-ticket mint, and WebSocket stream routes through
// `/c/{computer_id}` when scoped to a remote computer, parses flat remote rejection envelopes into
// fatal `TerminalApiError`s, and resumes with `after_seq` after a disconnect.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  TERMINAL_SERVER_OP,
  encodeTerminalServerControlFrame,
  encodeTerminalServerOutputFrame,
} from "../../lib/terminal-wire";
import { TerminalProtocolClient } from "../../lib/terminal-protocol-client";
import { terminalStreamFatalCode } from "../../lib/terminal-stream-fatal";
import { DEV_SERVER_TERMINAL } from "../../mocks/terminal-fixtures";
import {
  TerminalApiError,
  createTerminal,
  fetchTerminals,
  mintTerminalAttachTicket,
  terminalStreamPath,
} from "../terminal-api";
import type { TerminalSocket } from "../terminal-socket";

const COMPUTER_ID = "cmp_7f3k2q9x";
const WORKSPACE_ID = "ws_remote";
const TERMINAL_ID = "term_remote_1";
const PROFILE = { profile: "default" } as const;

function respond(body: unknown, status = 200) {
  vi.mocked(fetch).mockResolvedValueOnce(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    })
  );
}

function lastRequest() {
  const call = vi.mocked(fetch).mock.calls.at(-1);
  if (!call) throw new Error("expected fetch call");
  return { url: String(call[0]), init: call[1] };
}

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("remote terminal adapter & protocol client", () => {
  it("Should route remote terminal REST and stream URLs through /c/{computer_id}", async () => {
    respond({ terminals: [DEV_SERVER_TERMINAL] });
    await fetchTerminals(WORKSPACE_ID, PROFILE, undefined, COMPUTER_ID);
    expect(lastRequest().url).toContain(
      `/c/${COMPUTER_ID}/api/workspaces/${WORKSPACE_ID}/terminals?profile=default`
    );

    respond({ terminal: DEV_SERVER_TERMINAL });
    await createTerminal(WORKSPACE_ID, { title: "remote" }, PROFILE, null, undefined, COMPUTER_ID);
    expect(lastRequest().url).toContain(
      `/c/${COMPUTER_ID}/api/workspaces/${WORKSPACE_ID}/terminals?profile=default`
    );
    expect(lastRequest().init?.body).toBe(JSON.stringify({ title: "remote" }));

    respond({ ticket: "tkt-remote", expires_at: "2026-11-24T12:00:00Z" });
    await mintTerminalAttachTicket(
      WORKSPACE_ID,
      TERMINAL_ID,
      "write",
      PROFILE,
      null,
      undefined,
      COMPUTER_ID
    );
    expect(lastRequest().url).toContain(
      `/c/${COMPUTER_ID}/api/workspaces/${WORKSPACE_ID}/terminals/${TERMINAL_ID}/attach-ticket?profile=default`
    );

    expect(
      terminalStreamPath(
        WORKSPACE_ID,
        TERMINAL_ID,
        { ticket: "tkt-remote", mode: "write", flow: "drop", afterSeq: 42n },
        COMPUTER_ID
      )
    ).toBe(
      `/c/${COMPUTER_ID}/api/workspaces/${WORKSPACE_ID}/terminals/${TERMINAL_ID}/stream?ticket=tkt-remote&mode=write&flow=drop&after_seq=42`
    );
  });

  it("Should parse flat remote rejection envelopes into fatal TerminalApiErrors", async () => {
    respond(
      {
        error: "missing required scope terminal:operate",
        code: "remote_scope_denied",
        details: { scope: "terminal:operate" },
      },
      403
    );

    const err = await createTerminal(WORKSPACE_ID, {}, PROFILE, null, undefined, COMPUTER_ID).catch(
      cause => cause
    );
    expect(err).toBeInstanceOf(TerminalApiError);
    if (!(err instanceof TerminalApiError)) throw err;
    expect(err).toMatchObject({
      status: 403,
      code: "remote_scope_denied",
      details: { scope: "terminal:operate" },
    });
    expect(terminalStreamFatalCode(err)).toBe("remote_scope_denied");
  });

  it("Should reconnect a remote terminal stream with after_seq after a disconnect", async () => {
    respond({ ticket: "tkt-1", expires_at: "2026-11-24T12:00:00Z" });
    respond({ ticket: "tkt-2", expires_at: "2026-11-24T12:01:00Z" });

    const openedPaths: string[] = [];
    const sockets: TerminalSocket[] = [];
    const firstOpened = Promise.withResolvers<string>();
    const secondOpened = Promise.withResolvers<string>();
    const writeDone = Promise.withResolvers<void>();
    const reconnectScheduled = Promise.withResolvers<() => void>();

    const client = new TerminalProtocolClient({
      workspaceId: WORKSPACE_ID,
      terminalId: TERMINAL_ID,
      computerId: COMPUTER_ID,
      scope: PROFILE,
      mode: "write",
      sink: {
        write: () => {
          writeDone.resolve();
          return Promise.resolve();
        },
        reset: () => {},
        applyDimensions: () => {},
      },
      socketFactory: path => {
        openedPaths.push(path);
        const socket: TerminalSocket = {
          close: () => {},
          send: () => {},
          onopen: null,
          onmessage: null,
          onclose: null,
          onerror: null,
        };
        sockets.push(socket);
        if (openedPaths.length === 1) firstOpened.resolve(path);
        else if (openedPaths.length === 2) secondOpened.resolve(path);
        return socket;
      },
      schedule: run => {
        reconnectScheduled.resolve(run);
        return () => {};
      },
      attachDeadline: () => () => {},
    });

    client.start();
    const firstPath = await firstOpened.promise;
    expect(firstPath).toContain(
      `/c/${COMPUTER_ID}/api/workspaces/${WORKSPACE_ID}/terminals/${TERMINAL_ID}/stream?ticket=tkt-1`
    );

    const firstSocket = sockets[0]!;
    firstSocket.onopen?.(new Event("open"));
    firstSocket.onmessage?.(
      new MessageEvent("message", {
        data: encodeTerminalServerControlFrame(TERMINAL_SERVER_OP.attached, {
          seq: "0",
          mode: "write",
          cols: 80,
          rows: 24,
          truncated: false,
        }),
      })
    );
    // Deliver 6 bytes at seq=1..6 so committed cursor advances to 6.
    firstSocket.onmessage?.(
      new MessageEvent("message", { data: encodeTerminalServerOutputFrame(1n, "devbox") })
    );

    await writeDone.promise;
    await Promise.resolve();

    // Simulate network drop and fire reconnect timer.
    firstSocket.onclose?.(new CloseEvent("close", { code: 1006 }));
    const runReconnect = await reconnectScheduled.promise;
    runReconnect();

    const secondPath = await secondOpened.promise;
    expect(secondPath).toContain(
      `/c/${COMPUTER_ID}/api/workspaces/${WORKSPACE_ID}/terminals/${TERMINAL_ID}/stream?ticket=tkt-2`
    );
    expect(secondPath).toContain("after_seq=6");

    client.stop();
  });
});

// Suite: Remote terminal REST, stream path, error envelope, and reconnect resume (US-011)
// Invariant: every terminal REST call, attach-ticket mint, and WebSocket stream routes through
// `/c/{computer_id}` when scoped to a remote computer, parses flat remote rejection envelopes into
// fatal `TerminalApiError`s, and resumes with `after_seq` after a disconnect.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createDeferred } from "@/test/event-recorder";

import {
  attachedFrame,
  createFakeSink,
  createFakeSocketFactory,
  serverOutputFrame,
} from "../../lib/__tests__/terminal-stream-harness";
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

    const sockets = createFakeSocketFactory();
    const sink = createFakeSink();
    const reconnectScheduled = createDeferred<() => void>();

    const client = new TerminalProtocolClient({
      workspaceId: WORKSPACE_ID,
      terminalId: TERMINAL_ID,
      computerId: COMPUTER_ID,
      scope: PROFILE,
      mode: "write",
      sink,
      socketFactory: sockets.factory,
      schedule: run => {
        reconnectScheduled.resolve(run);
        return () => {};
      },
      attachDeadline: () => () => {},
    });

    client.start();
    await sockets.waitForSocket(1);
    const firstSocket = sockets.last();
    expect(firstSocket.path).toContain(
      `/c/${COMPUTER_ID}/api/workspaces/${WORKSPACE_ID}/terminals/${TERMINAL_ID}/stream?ticket=tkt-1`
    );

    firstSocket.open();
    firstSocket.deliver(attachedFrame({ seq: "0" }));
    // Deliver 6 bytes at seq=0..6 so committed cursor advances to 6.
    firstSocket.deliver(serverOutputFrame("0", "devbox"));

    await sink.waitForParse(1);

    // Simulate network drop and fire reconnect timer.
    firstSocket.drop();
    const runReconnect = await reconnectScheduled.promise;
    runReconnect();

    await sockets.waitForSocket(2);
    const secondSocket = sockets.last();
    expect(secondSocket.path).toContain(
      `/c/${COMPUTER_ID}/api/workspaces/${WORKSPACE_ID}/terminals/${TERMINAL_ID}/stream?ticket=tkt-2`
    );
    expect(secondSocket.path).toContain("after_seq=6");

    client.stop();
  });
});

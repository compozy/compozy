import { afterEach, describe, expect, it, vi } from "vitest";

import { TerminalProtocolClient } from "../terminal-protocol-client";
import { TERMINAL_CLIENT_OP, TERMINAL_MAX_INPUT_BYTES, TERMINAL_SERVER_OP } from "../terminal-wire";
import {
  attachedFrame,
  createFakeSink,
  createFakeSocketFactory,
  serverControlFrame,
  serverOutputFrame,
  stubTerminalFetch,
  type FakeTerminalSocket,
} from "./terminal-stream-harness";

/**
 * Canonical suite for the terminal protocol client (UT-076, UT-077, UT-079).
 *
 * Invariant: a fresh attach pass per attempt, `ATTACHED` then `OUTPUT` rendered
 * in order, input held closed across a `GAP` until the replayed snapshot has
 * been parsed, and ack credit returned only from the parse callback, in 16 KiB
 * grains.
 */

const ACK_GRAIN_BYTES = 16 * 1024;

let restoreFetch: (() => void) | null = null;

afterEach(() => {
  restoreFetch?.();
  restoreFetch = null;
});

function buildClient(
  options: {
    mode?: "read" | "write";
    autoParse?: boolean;
    screen?: () => unknown;
    deferScreen?: boolean;
  } = {}
) {
  const stub = stubTerminalFetch({ screen: options.screen, deferScreen: options.deferScreen });
  restoreFetch = stub.restore;
  const sockets = createFakeSocketFactory();
  const sink = createFakeSink({ autoParse: options.autoParse });
  const statuses: string[] = [];
  const inputEnabled: boolean[] = [];
  const leases: unknown[] = [];
  const exits: unknown[] = [];
  const streamErrors: unknown[] = [];
  const client = new TerminalProtocolClient({
    workspaceId: "ws-atlas",
    terminalId: "term-4f21c9a03b7e",
    scope: { profile: "work" },
    mode: options.mode ?? "write",
    sink,
    socketFactory: sockets.factory,
    random: () => 0,
    schedule: run => {
      const timer = setTimeout(run, 0);
      return () => clearTimeout(timer);
    },
    handlers: {
      onStatus: status => statuses.push(status),
      onInputEnabledChange: enabled => inputEnabled.push(enabled),
      onLease: frame => leases.push(frame),
      onExit: frame => exits.push(frame),
      onStreamError: error => streamErrors.push(error),
    },
  });
  return {
    client,
    sockets,
    sink,
    statuses,
    inputEnabled,
    leases,
    exits,
    streamErrors,
    calls: stub.calls,
    fetchStub: stub,
  };
}

const GAP_FRAME = serverControlFrame(TERMINAL_SERVER_OP.gap, {
  from_seq: 100,
  to_seq: 49_252,
  dropped_bytes: 49_152,
});

function sentOpcodes(socket: FakeTerminalSocket): number[] {
  return socket.sent.map(frame => frame[0]);
}

function ackCredits(socket: FakeTerminalSocket): number[] {
  return socket.sent
    .filter(frame => frame[0] === TERMINAL_CLIENT_OP.ack)
    .map(frame =>
      new DataView(frame.buffer, frame.byteOffset, frame.byteLength).getUint32(1, false)
    );
}

async function settle(): Promise<void> {
  await vi.waitFor(() => Promise.resolve());
  await Promise.resolve();
}

describe("TerminalProtocolClient", () => {
  it("Should mint a pass, attach, and render output", async () => {
    const { client, sockets, sink, calls } = buildClient({});
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));

    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame());
    socket.deliver(serverOutputFrame(0, "bun run dev\r\n"));
    await vi.waitFor(() => expect(sink.parsed).toContain("bun run dev\r\n"));

    expect(calls.filter(url => url.includes("/attach-ticket"))).toHaveLength(1);
    expect(socket.path).toContain("ticket=tkt-1");
    expect(socket.path).toContain("mode=write");
    expect(socket.path).toContain("flow=ack");
    // The daemon's size lands from the first frame; nothing else set it.
    expect(sink.dimensions).toEqual([{ cols: 96, rows: 28 }]);
    client.stop();
  });

  it("Should mint a fresh pass for every reconnect attempt", async () => {
    const { client, sockets, calls } = buildClient({});
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    sockets.last().open();
    sockets.last().deliver(attachedFrame());

    sockets.last().drop();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(2));

    const ticketCalls = calls.filter(url => url.includes("/attach-ticket"));
    expect(ticketCalls).toHaveLength(2);
    // A reused pass is `ticket_invalid`, so the second upgrade must carry a
    // different one.
    expect(sockets.sockets[0].path).toContain("ticket=tkt-1");
    expect(sockets.sockets[1].path).toContain("ticket=tkt-2");
    client.stop();
  });

  it("Should resume from the last sequence it rendered", async () => {
    const { client, sockets, sink } = buildClient({});
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    sockets.last().open();
    sockets.last().deliver(attachedFrame({ seq: 1024 }));
    // The replay ends where the attach said it would, however long its payload
    // is — a truncated one carries a synthetic prefix that belongs to no
    // absolute position.
    sockets.last().deliver(serverOutputFrame(0, "replayed screen"));
    // Then the live tail measures itself.
    sockets.last().deliver(serverOutputFrame(1024, "abcdef"));
    // Both are drawn before the socket dies: the cursor may only claim bytes
    // the emulator actually parsed.
    await vi.waitFor(() => expect(sink.parsed).toHaveLength(2));
    await settle();

    sockets.last().drop();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(2));

    expect(sockets.sockets[1].path).toContain("after_seq=1030");
    client.stop();
  });

  it("Should hold input closed across a gap until the replayed snapshot is parsed", async () => {
    const { client, sockets, sink, inputEnabled, statuses } = buildClient({ autoParse: false });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame());
    await vi.waitFor(() => expect(inputEnabled).toEqual([true]));

    socket.deliver(
      serverControlFrame(TERMINAL_SERVER_OP.gap, {
        from_seq: 100,
        to_seq: 49_252,
        dropped_bytes: 49_152,
      })
    );
    await vi.waitFor(() => expect(inputEnabled).toEqual([true, false]));
    await vi.waitFor(() => expect(sink.resets).toBe(1));
    await vi.waitFor(() => expect(sink.pendingWrites()).toBe(1));

    client.sendInput("whoami\r");
    expect(sentOpcodes(socket)).not.toContain(TERMINAL_CLIENT_OP.input);
    expect(statuses).toContain("resyncing");

    sink.completeWrite(0);

    await vi.waitFor(() => expect(inputEnabled).toEqual([true, false, true]));
    client.sendInput("whoami\r");
    expect(sentOpcodes(socket)).toContain(TERMINAL_CLIENT_OP.input);
    client.stop();
  });

  it("Should hold the live tail that arrives before the snapshot resolves", async () => {
    const { client, sockets, sink, inputEnabled, fetchStub } = buildClient({ deferScreen: true });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame());
    await vi.waitFor(() => expect(inputEnabled).toEqual([true]));

    socket.deliver(GAP_FRAME);
    await vi.waitFor(() => expect(fetchStub.pendingScreenReads()).toBe(1));
    expect(inputEnabled).toEqual([true, false]);

    // The daemon keeps sending while the snapshot is in flight. One frame is
    // already covered by the snapshot, one continues past it.
    socket.deliver(serverOutputFrame(4086, "0123456789"));
    socket.deliver(serverOutputFrame(4096, "live tail\r\n"));
    await settle();

    // Nothing reached the screen yet: writing through here would put bytes on a
    // screen that is about to be reset.
    expect(sink.parsed).toEqual([]);
    expect(sink.resets).toBe(0);

    fetchStub.resolveScreen({ content: "current screen", seq: 4096 });

    await vi.waitFor(() => expect(inputEnabled).toEqual([true, false, true]));
    expect(sink.resets).toBe(1);
    // Reset, then the snapshot, then only the uncovered continuation — the
    // covered frame is dropped rather than replayed on top of it.
    expect(sink.parsed).toEqual(["current screen", "live tail\r\n"]);
    client.stop();
  });

  it("Should keep input closed until the whole reconciled parse lands", async () => {
    const { client, sockets, sink, inputEnabled } = buildClient({ autoParse: false });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame());
    await vi.waitFor(() => expect(inputEnabled).toEqual([true]));

    socket.deliver(GAP_FRAME);
    await vi.waitFor(() => expect(sink.pendingWrites()).toBe(1));
    expect(inputEnabled).toEqual([true, false]);

    // A frame arrives while the snapshot itself is still being parsed.
    socket.deliver(serverOutputFrame(4096, "tail after parse\r\n"));
    await settle();
    expect(sink.parsed).toEqual([]);

    sink.completeWrite(0);
    await vi.waitFor(() => expect(sink.pendingWrites()).toBe(2));
    // The snapshot is on screen, but the tail is not: input stays closed.
    expect(sink.parsed).toEqual(["current screen"]);
    expect(inputEnabled).toEqual([true, false]);

    sink.completeWrite(1);

    await vi.waitFor(() => expect(inputEnabled).toEqual([true, false, true]));
    expect(sink.parsed).toEqual(["current screen", "tail after parse\r\n"]);
    client.stop();
  });

  it("Should reconcile again when more is dropped mid-catch-up", async () => {
    const { client, sockets, sink, inputEnabled, fetchStub } = buildClient({ deferScreen: true });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame());
    await vi.waitFor(() => expect(inputEnabled).toEqual([true]));

    socket.deliver(GAP_FRAME);
    await vi.waitFor(() => expect(fetchStub.pendingScreenReads()).toBe(1));
    socket.deliver(GAP_FRAME);
    fetchStub.resolveScreen({ content: "stale screen", seq: 4096 });

    // The snapshot in flight was already stale, so a second pass runs before
    // input reopens.
    await vi.waitFor(() => expect(fetchStub.pendingScreenReads()).toBe(1));
    expect(inputEnabled).toEqual([true, false]);

    fetchStub.resolveScreen({ content: "fresh screen", seq: 8192 });

    await vi.waitFor(() => expect(inputEnabled).toEqual([true, false, true]));
    expect(sink.parsed).toEqual(["stale screen", "fresh screen"]);
    expect(sink.resets).toBe(2);
    client.stop();
  });

  it("Should return credit only after the parse, in 16 KiB grains", async () => {
    const { client, sockets, sink } = buildClient({ autoParse: false });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame());

    const half = "x".repeat(ACK_GRAIN_BYTES / 2);
    socket.deliver(serverOutputFrame(0, half));
    socket.deliver(serverOutputFrame(ACK_GRAIN_BYTES / 2, half));
    // One write at a time: the emulator is a single screen, and two writes in
    // flight could land in either order.
    await vi.waitFor(() => expect(sink.pendingWrites()).toBe(1));

    // Both halves have arrived, but nothing has been parsed yet.
    expect(ackCredits(socket)).toEqual([]);

    sink.completeWrite(0);
    // Half a grain parsed is still below the threshold: credit is returned in
    // grains, not per frame. Completing the first write lets the second start.
    await vi.waitFor(() => expect(sink.pendingWrites()).toBe(2));
    expect(ackCredits(socket)).toEqual([]);

    sink.completeWrite(1);
    await vi.waitFor(() => expect(ackCredits(socket)).toEqual([ACK_GRAIN_BYTES]));
    client.stop();
  });

  it("Should never return credit while watching", async () => {
    const { client, sockets, sink } = buildClient({ mode: "read" });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame({ lease: "agent_owned" }));
    socket.deliver(serverOutputFrame(0, "y".repeat(ACK_GRAIN_BYTES * 2)));
    await vi.waitFor(() => expect(sink.parsed).toHaveLength(1));

    expect(socket.path).toContain("flow=drop");
    expect(ackCredits(socket)).toEqual([]);
    client.stop();
  });

  it("Should report the daemon's lease frames without inferring one", async () => {
    const { client, sockets, leases } = buildClient({ mode: "read" });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame({ lease: "agent_owned" }));

    expect(leases).toEqual([]);

    socket.deliver(
      serverControlFrame(TERMINAL_SERVER_OP.owner, {
        lease: "human_owned",
        actor_kind: "human",
        actor_id: "pedro",
        reason: "takeover",
      })
    );

    expect(leases).toEqual([
      { lease: "human_owned", actor_kind: "human", actor_id: "pedro", reason: "takeover" },
    ]);
    client.stop();
  });

  it("Should apply only the authoritative size, never its own proposal", async () => {
    const { client, sockets, sink } = buildClient({});
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame({ cols: 96, rows: 28 }));

    client.proposeDimensions(132, 44);

    expect(sink.dimensions).toEqual([{ cols: 96, rows: 28 }]);
    expect(sentOpcodes(socket)).toContain(TERMINAL_CLIENT_OP.resize);

    socket.deliver(serverControlFrame(TERMINAL_SERVER_OP.resized, { cols: 120, rows: 36 }));

    expect(sink.dimensions).toEqual([
      { cols: 96, rows: 28 },
      { cols: 120, rows: 36 },
    ]);
    client.stop();
  });

  it("Should surface a stream refusal instead of retrying it as content", async () => {
    const { client, sockets, streamErrors } = buildClient({ mode: "read" });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(
      serverControlFrame(TERMINAL_SERVER_OP.error, {
        code: "slow_consumer",
        message: "viewer queue was full for 10s",
      })
    );

    expect(streamErrors).toEqual([
      { code: "slow_consumer", message: "viewer queue was full for 10s" },
    ]);
    client.stop();
  });

  it("Should close input and report the exit when the program ends", async () => {
    const { client, sockets, inputEnabled, exits } = buildClient();
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame());
    socket.deliver(
      serverControlFrame(TERMINAL_SERVER_OP.exit, {
        cause: "exited",
        exit_code: 0,
        signal: null,
        seq: 8192,
      })
    );

    expect(exits).toEqual([{ cause: "exited", exit_code: 0, signal: null, seq: 8192 }]);
    expect(inputEnabled.at(-1)).toBe(false);
    client.stop();
  });

  it("Should frame a paste larger than the daemon accepts, byte for byte", async () => {
    const { client, sockets, inputEnabled } = buildClient();
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame());
    // Input has to be genuinely open, or nothing would be sent at all.
    await vi.waitFor(() => expect(inputEnabled.at(-1)).toBe(true));

    // A paste is one event of any size; the daemon refuses anything over 64 KiB.
    const paste = "y".repeat(TERMINAL_MAX_INPUT_BYTES + 500);
    client.sendInput(paste);

    const frames = inputFrames(socket);
    expect(frames.length).toBe(2);
    for (const frame of frames) {
      expect(frame.byteLength).toBeLessThanOrEqual(TERMINAL_MAX_INPUT_BYTES);
    }
    expect(new TextDecoder().decode(concat(frames))).toBe(paste);
    client.stop();
  });

  it("Should never split a character across two input frames", async () => {
    const { client, sockets, inputEnabled } = buildClient();
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame());
    await vi.waitFor(() => expect(inputEnabled.at(-1)).toBe(true));

    // A three-byte character straddling the 64 KiB boundary: cutting it would
    // decode to a different string than the one that was pasted.
    const filler = "z".repeat(TERMINAL_MAX_INPUT_BYTES - 1);
    const paste = `${filler}€${"z".repeat(10)}`;
    client.sendInput(paste);

    const frames = inputFrames(socket);
    expect(frames.length).toBe(2);
    // The boundary frame stops before the character rather than inside it.
    expect(frames[0].byteLength).toBe(TERMINAL_MAX_INPUT_BYTES - 1);
    expect(new TextDecoder().decode(concat(frames))).toBe(paste);
    // Each frame decodes on its own — no replacement characters anywhere.
    for (const frame of frames) {
      expect(new TextDecoder("utf-8", { fatal: true }).decode(frame)).not.toContain("�");
    }
    client.stop();
  });
});

describe("TerminalProtocolClient — teardown and races", () => {
  it("Should wait for a pending parse before deciding where to resume", async () => {
    const { client, sockets, sink, streamErrors } = buildClient({ autoParse: false });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame({ seq: 100 }));
    socket.deliver(serverOutputFrame(0, "the screen so far"));
    await vi.waitFor(() => expect(sink.pendingWrites()).toBe(1));

    // The socket dies while those bytes are still being parsed. They may yet
    // land on the screen, so the resume point cannot be read until they have.
    socket.drop();
    await settle();
    expect(sockets.sockets).toHaveLength(1);

    sink.completeWrite(0);
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(2));

    // Drawn, so the reconnection asks for what comes after them.
    expect(sockets.sockets[1].path).toContain("after_seq=100");
    // Nothing was credited or reported on the socket that died.
    expect(ackCredits(socket)).toEqual([]);
    expect(streamErrors).toEqual([]);
  });

  it("Should hold a truncated attach behind a snapshot before showing anything", async () => {
    const { client, sockets, sink, inputEnabled, calls } = buildClient({
      screen: () => ({
        content: "rebuilt screen",
        seq: 4096,
        truncated: false,
        busy: false,
        untrusted: true,
      }),
    });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    // The daemon says the resume point missed bytes: the suffix it is about to
    // send does not continue the screen.
    socket.deliver(attachedFrame({ seq: 4096, truncated: true }));
    socket.deliver(serverOutputFrame(0, "partial suffix with a synthetic prefix"));

    await vi.waitFor(() => expect(calls.some(url => url.includes("view=screen"))).toBe(true));
    await vi.waitFor(() => expect(sink.parsed).toContain("rebuilt screen"));
    // The truncated replay is never drawn; the snapshot replaced exactly what
    // it would have shown.
    expect(sink.parsed).not.toContain("partial suffix with a synthetic prefix");
    // The keyboard opened once, and only after the rebuilt screen was drawn —
    // never on the strength of the truncated attach alone.
    expect(inputEnabled).toEqual([true]);
    client.stop();
  });

  it("Should keep the resume point behind a snapshot that was still being written", async () => {
    const { client, sockets, sink, fetchStub } = buildClient({
      autoParse: false,
      deferScreen: true,
    });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame({ seq: 100 }));
    socket.deliver(serverOutputFrame(0, "before the gap"));
    await vi.waitFor(() => expect(sink.pendingWrites()).toBe(1));
    sink.completeWrite(0);

    socket.deliver(GAP_FRAME);
    await vi.waitFor(() => expect(fetchStub.pendingScreenReads()).toBe(1));
    // The live tail keeps coming while the snapshot is fetched.
    socket.deliver(serverOutputFrame(49_252, "after the gap"));
    fetchStub.resolveScreen({ content: "rebuilt", seq: 49_252 });
    await vi.waitFor(() => expect(sink.pendingWrites()).toBe(2));

    // The connection dies with the snapshot still being parsed.
    socket.drop();
    await settle();
    expect(sockets.sockets).toHaveLength(1);

    sink.completeWrite(1);
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(2));
    // The snapshot was drawn, so it counts; the held tail behind it was not.
    expect(sockets.sockets[1].path).toContain("after_seq=49252");
    client.stop();
  });

  it("Should not open the next connection while a catch-up still holds the screen", async () => {
    const { client, sockets, fetchStub } = buildClient({ deferScreen: true });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    socket.deliver(attachedFrame());
    socket.deliver(GAP_FRAME);
    await vi.waitFor(() => expect(fetchStub.pendingScreenReads()).toBe(1));

    // The socket dies while the snapshot is still in flight. Reconnecting now
    // would hand the new connection's replay to a buffer the old pass is about
    // to discard, and the screen would never come back.
    socket.drop();
    await settle();
    expect(sockets.sockets).toHaveLength(1);

    fetchStub.resolveScreen({ content: "rebuilt", seq: 49_252 });
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(2));
    client.stop();
  });

  it("Should keep the keyboard shut until the replay it was promised is drawn", async () => {
    const { client, sockets, sink, inputEnabled } = buildClient({ autoParse: false });
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    // The attach lands ahead of where this viewer is, so a replay is coming.
    socket.deliver(attachedFrame({ seq: 512 }));
    await settle();

    expect(inputEnabled).toEqual([]);
    client.sendInput("whoami\r");
    expect(sentOpcodes(socket)).not.toContain(TERMINAL_CLIENT_OP.input);

    socket.deliver(serverOutputFrame(0, "the screen so far"));
    await vi.waitFor(() => expect(sink.pendingWrites()).toBe(1));
    sink.completeWrite(0);

    await vi.waitFor(() => expect(inputEnabled).toEqual([true]));
    client.sendInput("whoami\r");
    expect(sentOpcodes(socket)).toContain(TERMINAL_CLIENT_OP.input);
    client.stop();
  });

  it("Should treat the first frame of a fresh attach as live, not as a replay", async () => {
    const { client, sockets, sink } = buildClient({});
    client.start();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(1));
    const socket = sockets.last();
    socket.open();
    // Nothing had run yet, so there is nothing to catch up on.
    socket.deliver(attachedFrame({ seq: 0 }));
    socket.deliver(serverOutputFrame(0, "abcdef"));
    await vi.waitFor(() => expect(sink.parsed).toHaveLength(1));
    await settle();

    socket.drop();
    await vi.waitFor(() => expect(sockets.sockets).toHaveLength(2));

    // Measured by its own length, because it was live output.
    expect(sockets.sockets[1].path).toContain("after_seq=6");
    client.stop();
  });
});

/** The payloads of every INPUT frame this client sent, in order. */
function inputFrames(socket: { sent: Uint8Array[] }): Uint8Array[] {
  return socket.sent
    .filter(frame => frame[0] === TERMINAL_CLIENT_OP.input)
    .map(frame => frame.subarray(1));
}

function concat(frames: Uint8Array[]): Uint8Array {
  const total = frames.reduce((sum, frame) => sum + frame.byteLength, 0);
  const joined = new Uint8Array(total);
  let offset = 0;
  for (const frame of frames) {
    joined.set(frame, offset);
    offset += frame.byteLength;
  }
  return joined;
}

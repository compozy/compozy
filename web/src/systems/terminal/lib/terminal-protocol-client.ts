/**
 * The live terminal connection.
 *
 * Owns one terminal's byte stream: minting a fresh attach pass per attempt,
 * decoding frames, returning flow-control credit only after the emulator has
 * parsed what it was given, resynchronising from a server snapshot after a gap,
 * and reconnecting with jittered backoff.
 *
 * It owns no rendering and no lease policy. The emulator is reached through a
 * sink, and lease state is reported exactly as the daemon's OWNER frames state
 * it — this client never infers who holds the write lease.
 */

import { readTerminal } from "../adapters/terminal-api";
import type { TerminalSocket } from "../adapters/terminal-socket";
import { TerminalGapBuffer } from "./terminal-gap-buffer";
import { TerminalResync } from "./terminal-resync";
import { detachTerminalStreamHandlers, openTerminalStream } from "./terminal-stream-open";
import type {
  TerminalProtocolClientOptions,
  TerminalStreamStatus,
} from "./terminal-protocol-contract";
import type { TerminalSignal } from "../types";
import { decodeTerminalServerFrame, TERMINAL_SERVER_OP } from "./terminal-wire";
import { TerminalCommandSender } from "./terminal-command-sender";
import { defaultSchedule, terminalBackoffDelay } from "./terminal-backoff";
import { dispatchTerminalControlFrame } from "./terminal-control-frames";
import { TerminalCreditWindow } from "./terminal-credit-window";
import { TerminalEmulatorQueue } from "./terminal-emulator-queue";
import type {
  TerminalAttachedFrame,
  TerminalGapFrame,
  TerminalResizedFrame,
} from "./terminal-wire-schema";

export type {
  TerminalProtocolClientOptions,
  TerminalStreamHandlers,
  TerminalStreamSink,
  TerminalStreamStatus,
} from "./terminal-protocol-contract";

type TerminalLeaseRequest = { kind: "takeover"; force: boolean } | { kind: "release" };

export class TerminalProtocolClient {
  private socket: TerminalSocket | null = null;
  private socketOpen = false;
  private pendingLeaseRequest: TerminalLeaseRequest | null = null;
  /**
   * Which connection is current.
   *
   * Catching up after a gap is asynchronous, and the socket can die while it
   * runs. Work started on one connection must never report its result onto
   * another — a resync that finished after a disconnect would otherwise reopen
   * input with no socket underneath it.
   */
  private connectionEpoch = 0;
  private stopped = false;
  private attempt = 0;
  /**
   * The last byte this viewer has actually *seen*.
   *
   * Resuming from anywhere past this skips output. So it advances only when the
   * emulator has parsed the bytes, when a snapshot has replaced the screen, or
   * when a held tail has been written — never merely because a frame arrived.
   * A connection that dies mid-catch-up asks again from here, and the daemon
   * replays what was never drawn.
   */
  private committedSeq = 0;
  private readonly sender = new TerminalCommandSender({
    socket: () => this.socket,
    reportError: (cause: unknown, fallback: string) => this.reportClientError(cause, fallback),
  });
  private readonly credit = new TerminalCreditWindow(frame =>
    this.sender.send(frame, "The terminal could not report what it has drawn.")
  );
  private inputEnabled = false;
  private readonly gapBuffer = new TerminalGapBuffer({
    write: (bytes: Uint8Array) => this.options.sink.write(bytes),
    returnCredit: (bytes: number) => this.returnCredit(bytes),
    commit: (seqEnd: number) => this.commit(seqEnd),
    // The held tail belongs to the connection that reported the gap. Once that
    // connection is gone, the rest of it is not written — the reconnection asks
    // for it again from the last byte that did reach the screen.
    isCancelled: () => this.stopped || this.resyncEpoch !== this.connectionEpoch,
  });
  /** The connection whose catch-up is running, so a takeover cancels it. */
  private resyncEpoch = 0;
  /** The resume point this attempt asked for, or zero for a first attach. */
  private resumedFrom = 0;
  /** Where the replay announced by `ATTACHED` ends, until it has been drawn. */
  private replayTarget: number | null = null;
  /** Set when the daemon called the attach truncated: the replay is not usable. */
  private discardReplay = false;
  private readonly emulator = new TerminalEmulatorQueue();
  private readonly resync = new TerminalResync({
    readSnapshot: () =>
      readTerminal(
        this.options.workspaceId,
        this.options.terminalId,
        { view: "screen" },
        this.options.scope,
        this.abort.signal
      ),
    reset: () => this.options.sink.reset(),
    write: (content: string) => this.options.sink.write(content),
    enqueue: (run: () => Promise<void>) => this.emulator.run(run),
    gapBuffer: this.gapBuffer,
    commit: (seqEnd: number) => this.commit(seqEnd),
    setStatus: (status: "resyncing" | "connected") => this.setStatus(status),
    setInputEnabled: (enabled: boolean) => this.setInputEnabled(enabled),
    onRecovered: () => this.options.handlers?.onGapCleared?.(),
    reportError: (cause: unknown, fallback: string) => this.reportClientError(cause, fallback),
    reconnectFromCommitted: () => this.reconnectFromCommitted(),
    isStopped: () => this.stopped,
    currentEpoch: () => this.connectionEpoch,
    mayWrite: () => this.options.mode === "write",
  });
  private cancelReconnect: (() => void) | null = null;
  private readonly abort = new AbortController();

  constructor(private readonly options: TerminalProtocolClientOptions) {}

  /** The flow class follows the attach mode: watchers drop, writers ack. */
  private get flow(): "drop" | "ack" {
    return this.options.mode === "write" ? "ack" : "drop";
  }

  start(): void {
    if (this.stopped) return;
    void this.connect();
  }

  /** Detaches politely, then stops. The terminal itself keeps running. */
  stop(): void {
    if (this.stopped) return;
    this.stopped = true;
    // Anything still in flight belongs to a connection nobody is watching.
    this.connectionEpoch += 1;
    this.cancelReconnect?.();
    this.cancelReconnect = null;
    this.abort.abort();
    const socket = this.socket;
    this.socket = null;
    this.socketOpen = false;
    this.pendingLeaseRequest = null;
    if (socket) {
      // Exactly one DETACH per attachment. Releasing control already sent it,
      // and a second would detach a lease this client no longer holds.
      this.sender.detachOnClose(socket);
      detachTerminalStreamHandlers(socket);
      socket.close(1000, "detached");
    }
    this.setStatus("closed");
  }

  /** Sends a keystroke or a paste. Refused locally while input is gated. */
  sendInput(data: string): void {
    if (!this.inputEnabled) return;
    this.sender.input(data);
  }

  proposeDimensions(cols: number, rows: number): void {
    this.sender.proposeDimensions(cols, rows);
  }

  sendSignal(signal: TerminalSignal): void {
    this.sender.signal(signal);
  }

  /** Claims the write lease. `force` skips the human-vs-human confirmation. */
  requestTakeover(force: boolean): void {
    if (this.stopped) return;
    this.pendingLeaseRequest = { kind: "takeover", force };
    this.flushLeaseRequest();
  }

  /** Gives the write lease back and waits for the daemon's `OWNER` frame. */
  releaseControl(): void {
    if (this.stopped) return;
    this.pendingLeaseRequest = { kind: "release" };
    this.flushLeaseRequest();
  }

  private flushLeaseRequest(): void {
    const request = this.pendingLeaseRequest;
    if (!request || !this.socketOpen) return;
    const sent =
      request.kind === "takeover"
        ? this.sender.takeover(request.force)
        : this.sender.release("The terminal could not release control.");
    if (sent && this.pendingLeaseRequest === request) this.pendingLeaseRequest = null;
  }

  private setStatus(status: TerminalStreamStatus): void {
    this.options.handlers?.onStatus?.(status);
  }

  private setInputEnabled(enabled: boolean): void {
    if (this.inputEnabled === enabled) return;
    this.inputEnabled = enabled;
    this.options.handlers?.onInputEnabledChange?.(enabled);
  }

  private async connect(): Promise<void> {
    if (this.stopped) return;
    this.setStatus(this.attempt === 0 ? "connecting" : "reconnecting");
    // Let the previous connection finish with the screen before asking where to
    // resume. Two reasons, both about the same thing:
    //
    // A write still being parsed will commit when it lands, and a resume point
    // read before that would ask the daemon to replay bytes about to appear —
    // drawing them twice.
    //
    // A catch-up still waiting on its snapshot owns the gap buffer. Opening the
    // next socket first would push that connection's replay into a buffer the
    // old pass is about to throw away, and the screen would never come back.
    await this.resync.idle();
    await this.emulator.drain();
    if (this.stopped) return;
    let socket: TerminalSocket;
    // Remembered so the attach frame can be read correctly: whether a replay is
    // coming at all depends on whether this attempt asked to resume.
    this.resumedFrom = this.committedSeq;
    try {
      socket = await openTerminalStream({
        workspaceId: this.options.workspaceId,
        terminalId: this.options.terminalId,
        scope: this.options.scope,
        mode: this.options.mode,
        viewer: this.options.viewer,
        flow: this.flow,
        afterSeq: this.committedSeq > 0 ? this.committedSeq : undefined,
        proposed: this.sender.proposed,
        ...(this.options.socketFactory ? { socketFactory: this.options.socketFactory } : {}),
        signal: this.abort.signal,
      });
    } catch (cause) {
      if (this.stopped) return;
      this.reportClientError(cause, "Failed to open a connection pass.");
      this.scheduleReconnect();
      return;
    }
    if (this.stopped) {
      socket.close(1000, "detached");
      return;
    }
    this.socket = socket;
    this.socketOpen = false;
    this.connectionEpoch += 1;
    socket.onopen = () => {
      if (this.socket !== socket) return;
      this.socketOpen = true;
      this.setStatus("connected");
      this.flushLeaseRequest();
    };
    socket.onmessage = event => this.handleMessage(event);
    socket.onerror = () => this.setStatus("reconnecting");
    socket.onclose = () => {
      if (this.stopped || this.socket !== socket) return;
      this.socket = null;
      this.socketOpen = false;
      // Anything still in flight belongs to a connection that no longer exists.
      this.connectionEpoch += 1;
      this.setInputEnabled(false);
      // A detached client asked to be closed. Reconnecting here would reclaim
      // the write lease it just handed back, before the watcher attachment that
      // replaces it has even been created.
      if (this.sender.detachSent) return;
      this.scheduleReconnect();
    };
  }

  private handleMessage(event: MessageEvent<unknown>): void {
    const data = event.data;
    if (!(data instanceof ArrayBuffer) && !(data instanceof Uint8Array)) {
      this.reportClientError(
        new Error("The terminal stream sent a non-binary frame."),
        "The terminal stream sent a non-binary frame."
      );
      this.reconnectFromCommitted();
      return;
    }
    let frame;
    try {
      frame = decodeTerminalServerFrame(data);
    } catch (cause) {
      this.reportClientError(cause, "The terminal stream sent a frame this client cannot read.");
      this.reconnectFromCommitted();
      return;
    }
    if (frame.op === TERMINAL_SERVER_OP.output) {
      this.consumeOutput(frame.seq, frame.bytes);
      return;
    }
    this.consumeControl(frame.op, frame.payload);
  }

  /**
   * Writes bytes and returns credit only once the emulator reports the parse.
   * Acking on receipt would let a viewer that cannot keep up pull more work
   * than it can render, which is exactly what the credit window exists to stop.
   *
   * While a gap is being reconciled the frame is held instead. The daemon keeps
   * sending the live tail across a gap, and writing it straight through would
   * put bytes on a screen that is about to be reset — the snapshot would then
   * erase them, or land interleaved with them.
   */
  private consumeOutput(seq: number, bytes: Uint8Array): void {
    // The first frame after `ATTACHED` is the replay, and it ends exactly where
    // the daemon said it would. Every later frame is live and measures itself.
    const replayTarget = this.replayTarget;
    const discarding = this.discardReplay && replayTarget !== null;
    this.replayTarget = null;
    this.discardReplay = false;
    const size = bytes.byteLength;
    const end = replayTarget ?? seq + size;
    if (discarding) {
      // Never drawn, but the daemon counted it against this viewer's window.
      this.returnCredit(size);
      return;
    }
    if (this.resync.isRunning) {
      // A replay held across a gap is covered up to the target the daemon named
      // — its synthetic prefix would otherwise read as a continuation past the
      // snapshot and be written a second time.
      this.gapBuffer.hold(replayTarget !== null ? end - size : seq, bytes);
      return;
    }
    const epoch = this.connectionEpoch;
    void this.emulator
      .run(async () => {
        // A write whose connection died before it started never reaches the
        // screen, so the cursor stays put and the reconnection asks for it again.
        if (epoch !== this.connectionEpoch) return;
        await this.options.sink.write(bytes);
        // Past this point the bytes *are* drawn, whatever happened to the socket
        // meanwhile. Not committing them would make the reconnection replay bytes
        // already on screen and paint them twice.
        this.commit(end);
        this.returnCredit(size);
        // The replay is on screen, so the keyboard can open — if this is still
        // the connection that owns it.
        if (replayTarget !== null && epoch === this.connectionEpoch) {
          this.setInputEnabled(this.options.mode === "write");
        }
      })
      .catch(cause => {
        // A view that went away took its screen with it. That is a teardown, not
        // a failure: nothing was drawn, the cursor never moved, and whoever
        // mounts next attaches from where this viewer actually got to.
        if (this.stopped || epoch !== this.connectionEpoch) return;
        this.reportClientError(cause, "The terminal could not render its output.");
        // The screen no longer matches what the cursor claims was drawn, so the
        // connection is abandoned and the missing bytes asked for again.
        this.reconnectFromCommitted();
      });
  }

  /** Marks everything up to `seqEnd` as drawn, so a resume starts after it. */
  private commit(seqEnd: number): void {
    this.committedSeq = Math.max(this.committedSeq, seqEnd);
  }

  /**
   * Abandons this connection and asks again from the last byte on screen.
   *
   * Used when the screen can no longer be trusted to be continuous. The cursor
   * is not moved, so the daemon replays whatever was missed.
   */
  private reconnectFromCommitted(): void {
    const socket = this.socket;
    this.socket = null;
    this.socketOpen = false;
    this.connectionEpoch += 1;
    this.setInputEnabled(false);
    if (socket) {
      detachTerminalStreamHandlers(socket);
      socket.close(1000, "resync failed");
    }
    this.scheduleReconnect();
  }

  /** Watchers drop what they cannot keep up with, so only writers ack. */
  private returnCredit(bytes: number): void {
    if (this.flow !== "ack" || !this.socket) return;
    this.credit.record(bytes);
  }

  private consumeControl(op: number, payload: unknown): void {
    const handlers = this.options.handlers;
    try {
      dispatchTerminalControlFrame(op, payload, {
        onAttached: frame => this.applyAttached(frame),
        onOwner: frame => handlers?.onLease?.(frame),
        onPresence: frame => handlers?.onPresence?.(frame),
        onTitle: title => handlers?.onTitle?.(title),
        onResized: frame => this.applyResized(frame),
        onGap: frame => void this.resynchronize(frame),
        onExit: frame => {
          this.setInputEnabled(false);
          handlers?.onExit?.(frame);
        },
        onError: frame => handlers?.onStreamError?.(frame),
      });
    } catch (cause) {
      this.reportClientError(cause, "The terminal stream sent an unreadable frame.");
      this.reconnectFromCommitted();
    }
  }

  private applyAttached(frame: TerminalAttachedFrame): void {
    this.attempt = 0;
    // The cursor deliberately does not move here. `ATTACHED` announces where the
    // replay *will end*, and the replay itself arrives afterwards as `OUTPUT`.
    // Trusting the announcement would mean a disconnect in between resumes past
    // a replay that was never drawn.
    this.credit.reset();
    // The replay that follows is where the cursor lands once it is drawn. It is
    // also the one frame whose length says nothing about sequence: a truncated
    // replay is prefixed with a synthetic reset and preamble that belong to no
    // absolute position, so its end is this target rather than start + length.
    // A replay is coming whenever the daemon attaches ahead of where this
    // viewer left off — including a first attach to a terminal that has already
    // produced output. Only an attach that lands exactly on the resume point
    // has nothing to catch up on.
    this.replayTarget = frame.seq > this.resumedFrom ? frame.seq : null;
    this.discardReplay = false;
    // A client that asked for the whole history cannot vouch for what is
    // already on the screen: the emulator's buffer outlives connections by
    // design, so a fresh pass over a retained buffer would paint the replay on
    // top of the very bytes it repeats. The daemon prefixes its own reset only
    // on the truncated branch; the from-zero branch is this client's to clear.
    // Queued on the emulator so it lands ahead of the replay's writes.
    if (this.resumedFrom === 0 && this.replayTarget !== null) {
      const epoch = this.connectionEpoch;
      void this.emulator.run(async () => {
        if (epoch === this.connectionEpoch && !this.stopped) this.options.sink.reset();
      });
    }
    // The daemon's size is authoritative from the first frame; the client's own
    // proposal is only ever a vote.
    this.options.sink.applyDimensions({ cols: frame.cols, rows: frame.rows });
    this.setStatus("connected");
    this.options.handlers?.onAttached?.(frame);
    if (frame.truncated) {
      // The daemon has already said the resume point missed bytes. Showing the
      // suffix as if it continued the screen would be a lie about what ran, so
      // the screen is rebuilt from a snapshot and the keyboard stays shut until
      // it is on screen.
      //
      // The replay itself is thrown away rather than held: it is a partial
      // screen with a synthetic prefix, and the snapshot replaces exactly what
      // it would have drawn. Marked as state, not inferred from arrival order —
      // it can land before or after the snapshot request resolves.
      this.discardReplay = this.replayTarget !== null;
      void this.resynchronize({
        dropped_bytes: 0,
        from_seq: this.committedSeq,
        to_seq: frame.seq,
      });
      return;
    }
    // The keyboard waits for the replay. Typing into a screen the emulator has
    // not drawn yet answers a prompt that is not on it.
    if (this.replayTarget === null) this.setInputEnabled(this.options.mode === "write");
  }

  private applyResized(frame: TerminalResizedFrame): void {
    this.options.sink.applyDimensions({ cols: frame.cols, rows: frame.rows });
    this.options.handlers?.onResized?.(frame);
  }

  /** Reports the gap, then hands the catch-up to the resync pass. */
  private async resynchronize(frame: TerminalGapFrame): Promise<void> {
    this.options.handlers?.onGap?.(frame);
    this.resyncEpoch = this.connectionEpoch;
    await this.resync.run();
  }

  private scheduleReconnect(): void {
    if (this.stopped || this.cancelReconnect) return;
    this.setStatus("reconnecting");
    const delay = terminalBackoffDelay(this.attempt, this.options.random);
    this.attempt += 1;
    const schedule = this.options.schedule ?? defaultSchedule;
    this.cancelReconnect = schedule(() => {
      this.cancelReconnect = null;
      void this.connect();
    }, delay);
  }

  private reportClientError(cause: unknown, fallback: string): void {
    const error = cause instanceof Error ? cause : new Error(fallback);
    this.options.handlers?.onClientError?.(error);
  }
}

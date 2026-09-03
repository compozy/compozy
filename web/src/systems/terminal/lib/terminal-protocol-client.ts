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
import { detachTerminalStreamHandlers, openTerminalStream } from "./terminal-stream-open";
import type {
  TerminalProtocolClientOptions,
  TerminalStreamStatus,
} from "./terminal-protocol-contract";
import type { TerminalSignal } from "../types";
import { decodeTerminalServerFrame, TERMINAL_SERVER_OP } from "./terminal-wire";
import { TerminalCommandSender } from "./terminal-command-sender";
import { terminalStreamFatalCode } from "./terminal-stream-fatal";
import { defaultSchedule, terminalBackoffDelay } from "./terminal-backoff";
import { dispatchTerminalControlFrame } from "./terminal-control-frames";
import { TerminalCreditWindow } from "./terminal-credit-window";
import {
  createTerminalFrameConsumers,
  type TerminalFrameConsumers,
} from "./terminal-frame-consumers";
import { TerminalLeaseRequests } from "./terminal-lease-requests";
import { TerminalReplayState } from "./terminal-replay-state";
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

export class TerminalProtocolClient {
  private socket: TerminalSocket | null = null;
  private socketOpen = false;
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
  /**
   * The terminal itself is over — an EXIT frame arrived, or the daemon refused
   * a connection pass because the terminal is gone. Unlike a dropped socket,
   * this is not recoverable by reconnecting: the stream settles as `closed`
   * and the exit surface owns the story from here.
   */
  private ended = false;
  private attempt = 0;
  private readonly leaseRequests = new TerminalLeaseRequests();
  private readonly replay = new TerminalReplayState();
  private readonly sender = new TerminalCommandSender({
    // A socket that has not opened yet cannot carry a frame; handing it out
    // would record sends that never happened.
    socket: () => (this.socketOpen ? this.socket : null),
    reportError: (cause: unknown, fallback: string) => this.reportClientError(cause, fallback),
  });
  private readonly credit = new TerminalCreditWindow(frame =>
    this.sender.send(frame, "The terminal could not report what it has drawn.")
  );
  private inputEnabled = false;
  private readonly consumers: TerminalFrameConsumers;
  private cancelReconnect: (() => void) | null = null;
  private readonly abort = new AbortController();

  constructor(private readonly options: TerminalProtocolClientOptions) {
    this.consumers = createTerminalFrameConsumers({
      write: data => this.options.sink.write(data),
      reset: () => this.options.sink.reset(),
      readSnapshot: () =>
        readTerminal(
          this.options.workspaceId,
          this.options.terminalId,
          { view: "screen" },
          this.options.scope,
          this.abort.signal
        ),
      commit: seqEnd => this.replay.commit(seqEnd),
      returnCredit: bytes => this.returnCredit(bytes),
      currentEpoch: () => this.connectionEpoch,
      isStopped: () => this.stopped,
      isGapCancelled: () => this.replay.isResyncCancelled(this.stopped, this.connectionEpoch),
      replayTarget: () => this.replay.replayTarget,
      discardReplay: () => this.replay.discardReplay,
      finishReplay: () => this.replay.finishReplay(),
      onReplayComplete: () => this.setInputEnabled(this.options.mode === "write"),
      onTrustedFrame: frame => this.options.handlers?.onRedactedInput?.(frame),
      setStatus: status => this.setStatus(status),
      setInputEnabled: enabled => this.setInputEnabled(enabled),
      onRecovered: () => this.options.handlers?.onGapCleared?.(),
      mayWrite: () => this.options.mode === "write",
      reportError: (cause, fallback) => this.reportClientError(cause, fallback),
      reconnectFromCommitted: () => this.reconnectFromCommitted(),
    });
  }

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
    this.leaseRequests.clear();
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
    this.sender.recordProposal(cols, rows);
    // Watchers never vote on the wire: the daemon sizes by its writers, and an
    // observer's RESIZE would only be refused (ADR-009/ADR-015).
    if (this.options.mode === "write") this.sender.flushProposal();
  }

  sendSignal(signal: TerminalSignal): void {
    this.sender.signal(signal);
  }

  /** Claims the write lease. `force` skips the human-vs-human confirmation. */
  requestTakeover(force: boolean): void {
    if (this.stopped) return;
    this.leaseRequests.requestTakeover(force);
    this.flushLeaseRequest();
  }

  /** Gives the write lease back and waits for the daemon's `OWNER` frame. */
  releaseControl(): void {
    if (this.stopped) return;
    this.leaseRequests.requestRelease();
    this.flushLeaseRequest();
  }

  private flushLeaseRequest(): void {
    this.leaseRequests.flush(this.socketOpen, this.sender);
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
    await this.consumers.resync.idle();
    await this.consumers.emulator.drain();
    if (this.stopped) return;
    let socket: TerminalSocket;
    const resumedFrom = this.replay.beginConnectionAttempt();
    // Read once: a vote recorded while the mint is in flight is not in this
    // query, so it must not be marked as carried below.
    const carriedProposal = this.sender.proposed;
    try {
      socket = await openTerminalStream({
        workspaceId: this.options.workspaceId,
        terminalId: this.options.terminalId,
        scope: this.options.scope,
        mode: this.options.mode,
        viewer: this.options.viewer,
        flow: this.flow,
        afterSeq: resumedFrom > 0n ? resumedFrom : undefined,
        proposed: carriedProposal,
        ...(this.options.socketFactory ? { socketFactory: this.options.socketFactory } : {}),
        signal: this.abort.signal,
      });
    } catch (cause) {
      if (this.stopped) return;
      const fatalCode = terminalStreamFatalCode(cause);
      if (fatalCode !== null) {
        this.ended = true;
        // An exited terminal already has its exit surface; the other refusals
        // have no catalog row to speak for them, so the daemon's sentence does.
        if (fatalCode !== "terminal_exited" && cause instanceof Error) {
          this.options.handlers?.onStreamError?.({
            error: { code: fatalCode, message: cause.message },
          });
        }
        this.setStatus("closed");
        return;
      }
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
    this.sender.markProposalCarried(carriedProposal);
    socket.onopen = () => {
      if (this.socket !== socket) return;
      this.socketOpen = true;
      this.flushLeaseRequest();
      // Any size measured while this connection was opening goes out now.
      if (this.options.mode === "write") this.sender.flushProposal();
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
      if (this.ended) {
        this.setStatus("closed");
        return;
      }
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
      this.consumers.output.consume(frame.seq, frame.bytes);
      return;
    }
    this.consumeControl(frame.op, frame.payload);
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
        onRedactedInput: frame => this.consumers.redactedInput.consume(frame),
        onTitle: title => handlers?.onTitle?.(title),
        onResized: frame => this.applyResized(frame),
        onGap: frame => void this.resynchronize(frame),
        onExit: frame => {
          this.ended = true;
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
    this.credit.reset();
    const replayPlan = this.replay.applyAttached(frame);
    // A client that asked for the whole history cannot vouch for what is
    // already on the screen: the emulator's buffer outlives connections by
    // design, so a fresh pass over a retained buffer would paint the replay on
    // top of the very bytes it repeats. The daemon prefixes its own reset only
    // on the truncated branch; the from-zero branch is this client's to clear.
    // Queued on the emulator so it lands ahead of the replay's writes.
    if (replayPlan.resetRetainedScreen) {
      const epoch = this.connectionEpoch;
      void this.consumers.emulator.run(async () => {
        if (epoch === this.connectionEpoch && !this.stopped) this.options.sink.reset();
      });
    }
    // The daemon's size is authoritative from the first frame; the client's own
    // proposal is only ever a vote.
    this.options.sink.applyDimensions({ cols: frame.cols, rows: frame.rows });
    this.setStatus("connected");
    this.options.handlers?.onAttached?.(frame);
    if (replayPlan.startResync) {
      // The daemon has already said the resume point missed bytes. Showing the
      // suffix as if it continued the screen would be a lie about what ran, so
      // the screen is rebuilt from a snapshot and the keyboard stays shut until
      // it is on screen.
      //
      // The replay itself is thrown away rather than held: it is a partial
      // screen with a synthetic prefix, and the snapshot replaces exactly what
      // it would have drawn. Marked as state, not inferred from arrival order —
      // it can land before or after the snapshot request resolves.
      void this.startResync();
      return;
    }
    // The keyboard waits for the replay. Typing into a screen the emulator has
    // not drawn yet answers a prompt that is not on it.
    if (replayPlan.inputReady) this.setInputEnabled(this.options.mode === "write");
  }

  private applyResized(frame: TerminalResizedFrame): void {
    this.options.sink.applyDimensions({ cols: frame.cols, rows: frame.rows });
    this.options.handlers?.onResized?.(frame);
  }

  /** Reports the gap, then hands the catch-up to the resync pass. */
  private async resynchronize(frame: TerminalGapFrame): Promise<void> {
    this.options.handlers?.onGap?.(frame);
    await this.startResync();
  }

  private async startResync(): Promise<void> {
    this.replay.beginResync(this.connectionEpoch);
    await this.consumers.resync.run();
  }

  private scheduleReconnect(): void {
    if (this.ended) {
      this.setStatus("closed");
      return;
    }
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

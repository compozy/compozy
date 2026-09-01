/**
 * Assembly of the frame-consumer pipeline behind one live terminal connection.
 *
 * The protocol client owns connection state (epochs, commit cursor, replay
 * bookkeeping) and exposes it here as callbacks; this module owns how the
 * consumers connect to each other: output and redacted-input share one gap
 * buffer and one emulator queue, and the resync pass drives both.
 */

import { TerminalEmulatorQueue } from "./terminal-emulator-queue";
import { TerminalGapBuffer } from "./terminal-gap-buffer";
import { TerminalOutputConsumer } from "./terminal-output-consumer";
import { TerminalRedactedInputConsumer } from "./terminal-redacted-input-consumer";
import { TerminalResync, type TerminalResyncPort } from "./terminal-resync";
import type { TerminalRedactedInputFrame } from "./terminal-wire-schema";

/** Connection truths the consumers read from the protocol client. */
export interface TerminalConsumerHost {
  write: (data: Uint8Array | string) => Promise<void>;
  reset: () => void;
  readSnapshot: TerminalResyncPort["readSnapshot"];
  commit: (seqEnd: bigint) => void;
  returnCredit: (bytes: number) => void;
  currentEpoch: () => number;
  isStopped: () => boolean;
  /** The held tail belongs to the connection that reported the gap. */
  isGapCancelled: () => boolean;
  replayTarget: () => bigint | null;
  discardReplay: () => boolean;
  finishReplay: () => void;
  onReplayComplete: () => void;
  onTrustedFrame: (frame: TerminalRedactedInputFrame) => void;
  setStatus: (status: "resyncing" | "connected") => void;
  setInputEnabled: (enabled: boolean) => void;
  onRecovered: () => void;
  mayWrite: () => boolean;
  reportError: (cause: unknown, fallback: string) => void;
  reconnectFromCommitted: () => void;
}

export interface TerminalFrameConsumers {
  emulator: TerminalEmulatorQueue;
  output: TerminalOutputConsumer;
  redactedInput: TerminalRedactedInputConsumer;
  resync: TerminalResync;
}

export function createTerminalFrameConsumers(host: TerminalConsumerHost): TerminalFrameConsumers {
  const emulator = new TerminalEmulatorQueue();
  const gapBuffer = new TerminalGapBuffer({
    write: bytes => host.write(bytes),
    returnCredit: bytes => host.returnCredit(bytes),
    commit: seqEnd => host.commit(seqEnd),
    isCancelled: () => host.isGapCancelled(),
  });
  const resync = new TerminalResync({
    readSnapshot: () => host.readSnapshot(),
    reset: () => host.reset(),
    write: content => host.write(content),
    enqueue: run => emulator.run(run),
    gapBuffer,
    commit: seqEnd => host.commit(seqEnd),
    setStatus: status => host.setStatus(status),
    setInputEnabled: enabled => host.setInputEnabled(enabled),
    onRecovered: () => host.onRecovered(),
    reportError: (cause, fallback) => host.reportError(cause, fallback),
    reconnectFromCommitted: () => host.reconnectFromCommitted(),
    isStopped: () => host.isStopped(),
    currentEpoch: () => host.currentEpoch(),
    mayWrite: () => host.mayWrite(),
  });
  const output = new TerminalOutputConsumer({
    emulator,
    gapBuffer,
    write: bytes => host.write(bytes),
    replayTarget: () => host.replayTarget(),
    discardReplay: () => host.discardReplay(),
    finishReplay: () => host.finishReplay(),
    isResyncing: () => resync.isRunning,
    currentEpoch: () => host.currentEpoch(),
    isStopped: () => host.isStopped(),
    commit: seqEnd => host.commit(seqEnd),
    returnCredit: bytes => host.returnCredit(bytes),
    onReplayComplete: () => host.onReplayComplete(),
    reportError: cause => host.reportError(cause, "The terminal could not render its output."),
    reconnect: () => host.reconnectFromCommitted(),
  });
  const redactedInput = new TerminalRedactedInputConsumer({
    emulator,
    gapBuffer,
    write: bytes => host.write(bytes),
    onTrustedFrame: frame => host.onTrustedFrame(frame),
    replayTarget: () => host.replayTarget(),
    discardReplay: () => host.discardReplay(),
    finishReplay: () => host.finishReplay(),
    isResyncing: () => resync.isRunning,
    currentEpoch: () => host.currentEpoch(),
    isStopped: () => host.isStopped(),
    commit: seqEnd => host.commit(seqEnd),
    onReplayComplete: () => host.onReplayComplete(),
    reportError: cause =>
      host.reportError(cause, "The terminal could not render a redacted input marker."),
    reconnect: () => host.reconnectFromCommitted(),
  });
  return { emulator, output, redactedInput, resync };
}

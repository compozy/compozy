import { TerminalEmulatorQueue } from "./terminal-emulator-queue";
import { TerminalGapBuffer } from "./terminal-gap-buffer";

type TerminalOutputConsumerOptions = {
  emulator: TerminalEmulatorQueue;
  gapBuffer: TerminalGapBuffer;
  write: (bytes: Uint8Array) => void | Promise<void>;
  replayTarget: () => bigint | null;
  discardReplay: () => boolean;
  finishReplay: () => void;
  isResyncing: () => boolean;
  currentEpoch: () => number;
  isStopped: () => boolean;
  commit: (seqEnd: bigint) => void;
  returnCredit: (bytes: number) => void;
  onReplayComplete: () => void;
  reportError: (cause: unknown) => void;
  reconnect: () => void;
};

/** Draws PTY bytes and advances flow credit only after the emulator parses them. */
export class TerminalOutputConsumer {
  constructor(private readonly options: TerminalOutputConsumerOptions) {}

  consume(seq: bigint, bytes: Uint8Array): void {
    const replayTarget = this.options.replayTarget();
    const discarding = this.options.discardReplay() && replayTarget !== null;
    const size = bytes.byteLength;
    const end = seq + BigInt(size);
    const replayComplete = replayTarget !== null && end >= replayTarget;
    if (replayComplete) this.options.finishReplay();
    if (discarding) {
      this.options.returnCredit(size);
      return;
    }
    if (this.options.isResyncing()) {
      this.options.gapBuffer.hold(seq, bytes);
      return;
    }
    const epoch = this.options.currentEpoch();
    void this.options.emulator
      .run(async () => {
        if (epoch !== this.options.currentEpoch()) return;
        await this.options.write(bytes);
        this.options.commit(end);
        this.options.returnCredit(size);
        if (replayComplete && epoch === this.options.currentEpoch()) {
          this.options.onReplayComplete();
        }
      })
      .catch(cause => {
        if (this.options.isStopped() || epoch !== this.options.currentEpoch()) return;
        this.options.reportError(cause);
        this.options.reconnect();
      });
  }
}

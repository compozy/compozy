import { TerminalEmulatorQueue } from "./terminal-emulator-queue";
import { TerminalGapBuffer } from "./terminal-gap-buffer";
import { renderTerminalRedactedInput } from "./terminal-wire";
import type { TerminalRedactedInputFrame } from "./terminal-wire-schema";

type TerminalRedactedInputConsumerOptions = {
  emulator: TerminalEmulatorQueue;
  gapBuffer: TerminalGapBuffer;
  write: (bytes: Uint8Array) => void | Promise<void>;
  onTrustedFrame: (frame: TerminalRedactedInputFrame) => void;
  replayTarget: () => bigint | null;
  discardReplay: () => boolean;
  finishReplay: () => void;
  isResyncing: () => boolean;
  currentEpoch: () => number;
  isStopped: () => boolean;
  commit: (seqEnd: bigint) => void;
  onReplayComplete: () => void;
  reportError: (cause: unknown) => void;
  reconnect: () => void;
};

const terminalTextEncoder = new TextEncoder();

/** Renders only daemon-authored redaction metadata into the terminal stream. */
export class TerminalRedactedInputConsumer {
  constructor(private readonly options: TerminalRedactedInputConsumerOptions) {}

  consume(frame: TerminalRedactedInputFrame): void {
    this.options.onTrustedFrame(frame);
    const rendered = terminalTextEncoder.encode(renderTerminalRedactedInput(frame.characters));
    const end = frame.seq + 1n;
    const replayTarget = this.options.replayTarget();
    const discarding = this.options.discardReplay() && replayTarget !== null;
    const replayComplete = replayTarget !== null && end >= replayTarget;
    if (replayComplete) this.options.finishReplay();
    if (discarding) return;
    if (this.options.isResyncing()) {
      this.options.gapBuffer.holdMarker(frame.seq, rendered);
      return;
    }
    const epoch = this.options.currentEpoch();
    void this.options.emulator
      .run(async () => {
        if (epoch !== this.options.currentEpoch()) return;
        await this.options.write(rendered);
        this.options.commit(end);
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

import type { TerminalAttachedFrame } from "./terminal-wire-schema";

export interface TerminalReplayPlan {
  inputReady: boolean;
  resetRetainedScreen: boolean;
  startResync: boolean;
}

/** Owns the resume cursor and replay/gap state across connection passes. */
export class TerminalReplayState {
  private committed = 0n;
  private resumedFrom = 0n;
  private target: bigint | null = null;
  private discard = false;
  private resyncEpoch = 0;

  get committedSeq(): bigint {
    return this.committed;
  }

  get replayTarget(): bigint | null {
    return this.target;
  }

  get discardReplay(): boolean {
    return this.discard;
  }

  beginConnectionAttempt(): bigint {
    this.resumedFrom = this.committed;
    return this.resumedFrom;
  }

  applyAttached(frame: TerminalAttachedFrame): TerminalReplayPlan {
    this.target = frame.seq > this.resumedFrom ? frame.seq : null;
    this.discard = frame.truncated && this.target !== null;
    return {
      inputReady: !frame.truncated && this.target === null,
      resetRetainedScreen: this.resumedFrom === 0n && this.target !== null,
      startResync: frame.truncated,
    };
  }

  commit(seqEnd: bigint): void {
    if (seqEnd > this.committed) this.committed = seqEnd;
  }

  finishReplay(): void {
    this.target = null;
    this.discard = false;
  }

  beginResync(connectionEpoch: number): void {
    this.resyncEpoch = connectionEpoch;
  }

  isResyncCancelled(stopped: boolean, connectionEpoch: number): boolean {
    return stopped || this.resyncEpoch !== connectionEpoch;
  }
}

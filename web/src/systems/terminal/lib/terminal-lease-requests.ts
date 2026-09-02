interface TerminalLeaseSender {
  release(fallback: string): boolean;
  takeover(force: boolean): boolean;
}

type TerminalLeaseRequest = { kind: "takeover"; force: boolean } | { kind: "release" };

/** Retains the latest lease intent until an open socket confirms it was sent. */
export class TerminalLeaseRequests {
  private pending: TerminalLeaseRequest | null = null;

  requestTakeover(force: boolean): void {
    this.pending = { kind: "takeover", force };
  }

  requestRelease(): void {
    this.pending = { kind: "release" };
  }

  clear(): void {
    this.pending = null;
  }

  flush(socketOpen: boolean, sender: TerminalLeaseSender): void {
    const request = this.pending;
    if (!request || !socketOpen) return;
    const sent =
      request.kind === "takeover"
        ? sender.takeover(request.force)
        : sender.release("The terminal could not release control.");
    if (sent && this.pending === request) this.pending = null;
  }
}

/** One client operation resolved through the command-palette dispatch seam. */
export type ClientCommandRunner = (op: string, payload: unknown) => Promise<unknown>;

/**
 * Bridges daemon-pushed operations to the current shell without making React
 * state the transport. The channel owns its mutable runner and guards stale
 * effect cleanup from disconnecting a newer shell.
 */
export class ClientCommandChannel {
  private runner: ClientCommandRunner | null = null;

  connect(runner: ClientCommandRunner): () => void {
    this.runner = runner;
    return () => {
      if (this.runner === runner) this.runner = null;
    };
  }

  execute(op: string, payload: unknown): Promise<unknown> {
    if (this.runner === null) {
      return Promise.reject(new Error(`Unsupported client operation: ${op}`));
    }
    return this.runner(op, payload);
  }
}

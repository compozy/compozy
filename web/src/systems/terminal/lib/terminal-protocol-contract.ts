/**
 * What a terminal stream client is, from the outside.
 *
 * The sink is where decoded bytes go; the handlers are everything the client
 * reports back. Both are named here rather than beside the implementation so a
 * consumer can depend on the contract without depending on the machinery that
 * satisfies it.
 */

import type { TerminalSocketFactory } from "../adapters/terminal-socket";
import type {
  TerminalAttachedFrame,
  TerminalErrorFrame,
  TerminalExitFrame,
  TerminalGapFrame,
  TerminalOwnerFrame,
  TerminalResizedFrame,
} from "./terminal-wire-schema";
import type { TerminalAttachMode, TerminalProfileScopeParams } from "../types";

export type TerminalStreamStatus =
  | "idle"
  | "connecting"
  | "connected"
  | "reconnecting"
  | "resyncing"
  | "closed";

/** Where decoded bytes go. Implemented by the pane over `TerminalViewHandle`. */
export interface TerminalStreamSink {
  /** Resolves once the emulator has parsed the bytes, never on receipt. */
  write(data: Uint8Array | string): Promise<void>;
  reset(): void;
  applyDimensions(dimensions: { cols: number; rows: number }): void;
}

export interface TerminalStreamHandlers {
  onStatus?(status: TerminalStreamStatus): void;
  onAttached?(frame: TerminalAttachedFrame): void;
  /** The daemon's word on who holds the lease. The only source there is. */
  onLease?(frame: TerminalOwnerFrame): void;
  onTitle?(title: string): void;
  onResized?(frame: TerminalResizedFrame): void;
  onGap?(frame: TerminalGapFrame): void;
  /** The screen and held tail are continuous again after a reported gap. */
  onGapCleared?(): void;
  onExit?(frame: TerminalExitFrame): void;
  onStreamError?(error: TerminalErrorFrame): void;
  onClientError?(error: Error): void;
  /**
   * The local keyboard gate. It opens only after an attached replay or a gap
   * snapshot has fully parsed, and closes on gaps, exits, disconnects, release,
   * protocol errors, and teardown.
   */
  onInputEnabledChange?(enabled: boolean): void;
}

export interface TerminalProtocolClientOptions {
  workspaceId: string;
  terminalId: string;
  scope: TerminalProfileScopeParams;
  mode: TerminalAttachMode;
  sink: TerminalStreamSink;
  handlers?: TerminalStreamHandlers;
  socketFactory?: TerminalSocketFactory;
  /** Seam for deterministic backoff in tests. */
  random?: () => number;
  schedule?: (run: () => void, delayMs: number) => () => void;
}

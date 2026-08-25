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
import type { TerminalAttachMode, TerminalScopeParams } from "../types";

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
  onExit?(frame: TerminalExitFrame): void;
  onStreamError?(error: TerminalErrorFrame): void;
  onClientError?(error: Error): void;
  /** False while a gap is being replayed; true again after the parse lands. */
  onInputEnabledChange?(enabled: boolean): void;
}

export interface TerminalProtocolClientOptions {
  workspaceId: string;
  terminalId: string;
  scope: TerminalScopeParams;
  mode: TerminalAttachMode;
  sink: TerminalStreamSink;
  handlers?: TerminalStreamHandlers;
  socketFactory?: TerminalSocketFactory;
  /** Seam for deterministic backoff in tests. */
  random?: () => number;
  schedule?: (run: () => void, delayMs: number) => () => void;
}

/**
 * Opening one attachment to a terminal's byte stream.
 *
 * Two steps that always go together: mint a single-use pass, then upgrade to
 * the socket that pass authorizes. They live here so the client is left with
 * what to *do* about a connection rather than how to obtain one.
 */

import { mintTerminalAttachTicket, terminalStreamPath } from "../adapters/terminal-api";
import { createTerminalSocket, type TerminalSocketFactory } from "../adapters/terminal-socket";
import type { TerminalSocket } from "../adapters/terminal-socket";
import type { TerminalAttachMode, TerminalProfileScopeParams } from "../types";

export interface OpenTerminalStreamOptions {
  workspaceId: string;
  terminalId: string;
  scope: TerminalProfileScopeParams;
  mode: TerminalAttachMode;
  /** Watchers drop what they cannot keep up with; writers return credit. */
  flow: "drop" | "ack";
  /** Resume point: the last byte this viewer actually saw, or none. */
  afterSeq: number | undefined;
  /** The size this viewer would like, carried into the upgrade. */
  proposed: { cols: number; rows: number } | null;
  socketFactory?: TerminalSocketFactory;
  signal: AbortSignal;
}

/**
 * Mints a pass and opens the socket it authorizes.
 *
 * Every attempt mints its own: passes are single-use, so a cached one would be
 * refused as `ticket_invalid` on the first reconnect.
 */
export async function openTerminalStream(
  options: OpenTerminalStreamOptions
): Promise<TerminalSocket> {
  const minted = await mintTerminalAttachTicket(
    options.workspaceId,
    options.terminalId,
    options.mode,
    options.scope,
    options.signal
  );
  const path = terminalStreamPath(options.workspaceId, options.terminalId, {
    ticket: minted.ticket,
    mode: options.mode,
    flow: options.flow,
    afterSeq: options.afterSeq,
    cols: options.proposed?.cols,
    rows: options.proposed?.rows,
  });
  return (options.socketFactory ?? createTerminalSocket)(path);
}

/**
 * Stops listening to a socket before closing it.
 *
 * A close fired by the client's own teardown must not run the reconnect path
 * meant for a connection that dropped on its own.
 */
export function detachTerminalStreamHandlers(socket: TerminalSocket): void {
  socket.onopen = null;
  socket.onmessage = null;
  socket.onclose = null;
  socket.onerror = null;
}

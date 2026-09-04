"use client";

import type { TerminalViewHandle } from "@compozy/ui";
import { useEffect, useEffectEvent, useRef } from "react";

import { TerminalProtocolClient, type TerminalStreamSink } from "../lib/terminal-protocol-client";
import { terminalScopeKey } from "../lib/terminal-scope-key";
import type { TerminalSocketFactory } from "../adapters/terminal-socket";
import type { TerminalAttachMode, TerminalSignal, TerminalViewerIdentity } from "../types";
import { useTerminalStore } from "./use-terminal-store";

/** Socket injection owned by the attachment boundary, for tests and scripted stories. */
export type TerminalAttachmentSocketFactory = TerminalSocketFactory;

export interface UseTerminalAttachmentOptions {
  workspaceId: string;
  terminalId: string;
  scope: { profile: string };
  mode: TerminalAttachMode;
  viewer?: TerminalViewerIdentity | null;
  /** The emulator this connection paints into. */
  handleRef: React.RefObject<TerminalViewHandle | null>;
  /** Test seam; the browser socket is the default. */
  socketFactory?: TerminalAttachmentSocketFactory;
  enabled?: boolean;
  /**
   * Bumped to start the connection over.
   *
   * The client already reconnects on its own with backoff; this is how a person
   * says "now" instead of waiting it out.
   */
  restartKey?: number;
}

export interface TerminalAttachment {
  sendInput: (data: string) => void;
  proposeDimensions: (cols: number, rows: number) => void;
  sendSignal: (signal: TerminalSignal) => void;
}

/**
 * Binds one terminal's live connection to one emulator.
 *
 * The connection is keyed by `(workspace, terminal, mode)`. Every frame lands
 * in the store, which is the only place the UI reads connection state from.
 */
export function useTerminalAttachment(options: UseTerminalAttachmentOptions): TerminalAttachment {
  const store = useTerminalStore();
  const clientRef = useRef<TerminalProtocolClient | null>(null);
  const {
    workspaceId,
    terminalId,
    mode,
    socketFactory,
    enabled = true,
    restartKey = 0,
    scope,
  } = options;
  const profile = scope.profile ?? "";
  const viewerId = options.viewer?.id ?? null;
  const viewerAttachmentToken = options.viewer?.attachmentToken ?? null;

  const buildSink = useEffectEvent(
    (): TerminalStreamSink => ({
      write: data => options.handleRef.current?.write(data) ?? Promise.resolve(),
      reset: () => options.handleRef.current?.reset(),
      applyDimensions: dimensions => options.handleRef.current?.applyDimensions(dimensions),
    })
  );

  useEffect(() => {
    if (!enabled || workspaceId === "" || terminalId === "") return undefined;
    // Bind the scope before opening the pane, never after. The reducer drops
    // every pane from the previous `(workspace, profile)`, so a terminal id
    // reused across profiles would otherwise inherit the old scope's input gate
    // and error — and the drop would land on the pane just opened.
    store.trigger.scopeBound({ scopeKey: terminalScopeKey(workspaceId, profile) });
    store.trigger.paneOpened({ terminalId });
    const client = new TerminalProtocolClient({
      workspaceId,
      terminalId,
      scope: { profile },
      mode,
      viewer:
        viewerId !== null && viewerAttachmentToken !== null
          ? { id: viewerId, attachmentToken: viewerAttachmentToken }
          : null,
      sink: buildSink(),
      socketFactory,
      handlers: {
        onStatus: status => store.trigger.statusChanged({ terminalId, status }),
        onAttached: frame =>
          store.trigger.attached({
            terminalId,
            mode: frame.mode,
            cols: frame.cols,
            rows: frame.rows,
          }),
        onPresence: frame => store.trigger.presenceObserved({ terminalId, viewers: frame.viewers }),
        onResized: frame =>
          store.trigger.resized({ terminalId, cols: frame.cols, rows: frame.rows }),
        onGap: frame =>
          store.trigger.gapObserved({
            terminalId,
            gap: {
              droppedBytes: frame.dropped_bytes,
              fromSeq: frame.from_seq,
              toSeq: frame.to_seq,
            },
          }),
        onGapCleared: () => store.trigger.gapCleared({ terminalId }),
        onInputEnabledChange: value =>
          store.trigger.inputEnabledChanged({ terminalId, enabled: value }),
        onExit: frame =>
          store.trigger.exited({
            terminalId,
            exit: { cause: frame.cause, code: frame.exit_code, signal: frame.signal },
          }),
        onStreamError: ({ error }) =>
          store.trigger.streamErrored({
            terminalId,
            code: error.code,
            // Kept verbatim: for a code this client has no copy for, the
            // daemon's own sentence is the only thing a reader can act on.
            message: error.message ?? null,
          }),
        // Client-side failures (mint refused, undecodable frame) get the same
        // truthful surface as daemon refusals instead of an unexplained spinner.
        onClientError: error =>
          store.trigger.streamErrored({ terminalId, code: "client_error", message: error.message }),
      },
    });
    clientRef.current = client;
    client.start();
    return () => {
      client.stop();
      if (clientRef.current === client) clientRef.current = null;
    };
    // Effect events are deliberately absent: they are stable by contract, and
    // listing one would tie the connection's lifetime to a render.
  }, [
    enabled,
    mode,
    profile,
    restartKey,
    socketFactory,
    store,
    terminalId,
    viewerAttachmentToken,
    viewerId,
    workspaceId,
  ]);

  return {
    sendInput: data => clientRef.current?.sendInput(data),
    proposeDimensions: (cols, rows) => clientRef.current?.proposeDimensions(cols, rows),
    sendSignal: signal => clientRef.current?.sendSignal(signal),
  };
}

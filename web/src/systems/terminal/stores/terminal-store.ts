import { createStoreLogic } from "@xstate/store";

import type { TerminalStreamStatus } from "../lib/terminal-protocol-client";
import type { TerminalExitNotice, TerminalMode } from "../types";

/**
 * Client-only terminal state.
 *
 * Server truth — the catalog, the journal, pending questions — lives in the
 * query cache. What lives here is what only this browser knows: which pane is
 * connected and whether its input is open right now.
 */

export interface TerminalGapNotice {
  droppedBytes: number;
  fromSeq: bigint;
  toSeq: bigint;
}

export interface TerminalPaneState {
  terminalId: string;
  status: TerminalStreamStatus;
  mode: TerminalMode | null;
  cols: number | null;
  rows: number | null;
  viewers: number | null;
  /** Local input gate. Closed while a gap replay is in flight. */
  inputEnabled: boolean;
  gap: TerminalGapNotice | null;
  exit: TerminalExitNotice | null;
  /** A refusal the stream reported, by machine code. */
  errorCode: string | null;
  /**
   * The daemon's own sentence for that refusal.
   *
   * Kept because the surface promises to show it: a code the client has no copy
   * for would otherwise render as the bare code, which tells a reader nothing.
   */
  errorMessage: string | null;
}

export interface TerminalStoreState {
  /** `(workspace, profile)` identity these panes belong to. */
  scopeKey: string | null;
  panes: Record<string, TerminalPaneState>;
}

type TerminalStoreEvents = {
  /** Rebinds after any workspace, profile, or aggregate-scope change. */
  scopeBound: { scopeKey: string | null };
  paneOpened: { terminalId: string };
  statusChanged: { terminalId: string; status: TerminalStreamStatus };
  attached: {
    terminalId: string;
    mode: TerminalMode;
    cols: number;
    rows: number;
  };
  presenceObserved: { terminalId: string; viewers: number };
  inputEnabledChanged: { terminalId: string; enabled: boolean };
  resized: { terminalId: string; cols: number; rows: number };
  gapObserved: { terminalId: string; gap: TerminalGapNotice };
  gapCleared: { terminalId: string };
  exited: { terminalId: string; exit: TerminalExitNotice };
  streamErrored: { terminalId: string; code: string; message: string | null };
};

function emptyPane(terminalId: string): TerminalPaneState {
  return {
    terminalId,
    status: "idle",
    mode: null,
    cols: null,
    rows: null,
    viewers: null,
    inputEnabled: false,
    gap: null,
    exit: null,
    errorCode: null,
    errorMessage: null,
  };
}

function updatePane(
  context: TerminalStoreState,
  terminalId: string,
  update: (pane: TerminalPaneState) => TerminalPaneState
): TerminalStoreState | undefined {
  const pane = context.panes[terminalId];
  if (!pane) return undefined;
  return { ...context, panes: { ...context.panes, [terminalId]: update(pane) } };
}

export const terminalStoreLogic = createStoreLogic<TerminalStoreState, TerminalStoreEvents>({
  context: {
    scopeKey: null,
    panes: {},
  },
  on: {
    // A scope switch is not a filter: the previous scope's panes leave the store
    // entirely, so nothing stale can render while the new scope loads.
    scopeBound: (context, event) =>
      context.scopeKey === event.scopeKey ? undefined : { scopeKey: event.scopeKey, panes: {} },
    paneOpened: (context, event) =>
      context.panes[event.terminalId]
        ? undefined
        : {
            ...context,
            panes: { ...context.panes, [event.terminalId]: emptyPane(event.terminalId) },
          },
    statusChanged: (context, event) =>
      updatePane(context, event.terminalId, pane => ({ ...pane, status: event.status })),
    attached: (context, event) =>
      updatePane(context, event.terminalId, pane => ({
        ...pane,
        status: "connected",
        mode: event.mode,
        cols: event.cols,
        rows: event.rows,
        errorCode: null,
        errorMessage: null,
      })),
    presenceObserved: (context, event) =>
      updatePane(context, event.terminalId, pane => ({ ...pane, viewers: event.viewers })),
    inputEnabledChanged: (context, event) =>
      updatePane(context, event.terminalId, pane => ({ ...pane, inputEnabled: event.enabled })),
    resized: (context, event) =>
      updatePane(context, event.terminalId, pane => ({
        ...pane,
        cols: event.cols,
        rows: event.rows,
      })),
    gapObserved: (context, event) =>
      updatePane(context, event.terminalId, pane => ({ ...pane, gap: event.gap })),
    gapCleared: (context, event) =>
      updatePane(context, event.terminalId, pane => (pane.gap ? { ...pane, gap: null } : pane)),
    exited: (context, event) =>
      updatePane(context, event.terminalId, pane => ({
        ...pane,
        exit: event.exit,
        inputEnabled: false,
      })),
    streamErrored: (context, event) =>
      updatePane(context, event.terminalId, pane => ({
        ...pane,
        errorCode: event.code,
        errorMessage: event.message,
      })),
  },
});

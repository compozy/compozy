import { createStoreLogic } from "@xstate/store";

import type { TerminalStreamStatus } from "../lib/terminal-protocol-client";
import type {
  TerminalActor,
  TerminalExitNotice,
  TerminalLeaseState,
  TerminalMode,
} from "../types";

/**
 * Client-only terminal state.
 *
 * Server truth — the catalog, the journal, pending questions — lives in the
 * query cache. What lives here is what only this browser knows: which pane is
 * connected, whether its input is open right now, and what the daemon last said
 * about the lease. Lease state is never derived: it is written from `OWNER`
 * frames and from the `ATTACHED` frame's opening statement, and from nothing
 * else.
 */

export interface TerminalGapNotice {
  droppedBytes: number;
  fromSeq: number;
  toSeq: number;
}

export interface TerminalPaneState {
  terminalId: string;
  status: TerminalStreamStatus;
  lease: TerminalLeaseState;
  /** Who the daemon named. Absent until a frame names someone. */
  controller: TerminalActor | null;
  /**
   * Whether an `OWNER` frame has named the controller on this pane.
   *
   * `ATTACHED` states the lease but not the actor, so `controller: null` before
   * the first `OWNER` means "not said yet" rather than "nobody". Without this
   * flag the two are indistinguishable, and a terminal someone else is holding
   * reads as free for the moment between attaching and the first owner frame.
   */
  ownerObserved: boolean;
  /**
   * Whether the daemon has stated the lease on this pane at all.
   *
   * Once it has, its word stands — including while the connection is being
   * replaced. Falling back to the catalog whenever the socket is momentarily
   * away would undo a takeover the daemon just granted, because swapping to a
   * write attachment is itself what closes the previous connection.
   */
  leaseKnown: boolean;
  /** The daemon's reason for the last lease change, verbatim. */
  leaseReason: string | null;
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
    lease: TerminalLeaseState;
    mode: TerminalMode;
    cols: number;
    rows: number;
  };
  leaseObserved: {
    terminalId: string;
    lease: TerminalLeaseState;
    controller: TerminalActor | null;
    reason: string | null;
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
    // Nobody is claimed to hold the lease until the daemon says who does.
    lease: "available",
    controller: null,
    ownerObserved: false,
    leaseKnown: false,
    leaseReason: null,
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
        lease: event.lease,
        // An attachment begins a new stream pass. The previous pass's OWNER
        // actor is no longer evidence about this one, even when the terminal id
        // is the same; the next OWNER frame will name the current controller.
        controller: null,
        // `available` names nobody by definition. Other leases still need an
        // OWNER frame before this pass knows which person or agent holds them.
        ownerObserved: event.lease === "available",
        mode: event.mode,
        cols: event.cols,
        rows: event.rows,
        leaseKnown: true,
        errorCode: null,
        errorMessage: null,
      })),
    // `OWNER` is the only frame that names the actor, so it is the only one that
    // may replace the catalog's answer — including replacing it with nobody.
    leaseObserved: (context, event) =>
      updatePane(context, event.terminalId, pane => ({
        ...pane,
        lease: event.lease,
        controller: event.controller,
        ownerObserved: true,
        leaseKnown: true,
        leaseReason: event.reason,
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

"use client";

import type { TerminalViewHandle } from "@compozy/ui";
import { useSelector } from "@xstate/store-react";
import { useRef, useState } from "react";

import { terminalLeaseView, type TerminalLeaseView } from "../lib/terminal-lease";
import { terminalScopeKey } from "../lib/terminal-scope-key";
import type { TerminalPaneState } from "../stores/terminal-store";
import type { TerminalInfo } from "../types";
import {
  useTerminalAttachment,
  type TerminalAttachment,
  type TerminalAttachmentSocketFactory,
} from "./use-terminal-attachment";
import { useTerminalStore } from "./use-terminal-store";

export interface UseTerminalWindowConnectionOptions {
  terminal: TerminalInfo;
  workspaceId: string;
  profile: string;
  viewerId: string | null;
  readOnly?: boolean;
  socketFactory?: TerminalAttachmentSocketFactory;
}

export interface TerminalWindowConnection {
  pane: TerminalPaneState | undefined;
  lease: TerminalLeaseView;
  attachment: TerminalAttachment;
  handleRef: React.RefObject<TerminalViewHandle | null>;
  /** Asks the daemon for the write lease, forced only after a confirmation. */
  takeControl: (force: boolean) => void;
  /** Hands the write lease back and comes back as a watcher. */
  releaseControl: () => void;
  /** Starts the connection over now, rather than waiting out the backoff. */
  reconnect: () => void;
}

/**
 * Who the daemon says holds this terminal.
 *
 * The catalog is a starting point, not an authority: it is whatever the list
 * said when it was last read. `ATTACHED` states the lease; only `OWNER` names
 * the actor. A fresh attachment clears any actor named by the previous stream
 * pass; the next `OWNER` frame then names the current controller.
 */
function leaseFrom(
  terminal: TerminalInfo,
  pane: TerminalPaneState | undefined,
  viewerId: string | null
): TerminalLeaseView {
  return terminalLeaseView({
    // Once the daemon has stated the lease it stands, even while the connection
    // is being replaced — swapping to a write attachment is itself what closes
    // the previous one, and falling back to the catalog there would undo the
    // takeover that caused the swap.
    lease: pane?.leaseKnown ? pane.lease : terminal.lease,
    controller: pane?.ownerObserved ? pane.controller : terminal.controller,
    viewerId,
    mode: terminal.mode,
    capabilities: terminal.capabilities,
  });
}

/**
 * One terminal's live connection, and the two gestures that change who owns it.
 *
 * Taking control and giving it back are frames on this socket rather than REST
 * calls, which is why they live with the connection. Neither changes anything
 * on screen: the daemon answers with `OWNER`, and that frame is what the
 * surface reads.
 */
export function useTerminalWindowConnection({
  terminal,
  workspaceId,
  profile,
  viewerId,
  socketFactory,
  readOnly = false,
}: UseTerminalWindowConnectionOptions): TerminalWindowConnection {
  const store = useTerminalStore();
  const handleRef = useRef<TerminalViewHandle>(null);
  const scopeKey = terminalScopeKey(workspaceId, profile);
  // Pane state is only readable once the store agrees which scope it holds. A
  // terminal id can repeat across profiles, so reading a pane before the rebind
  // lands would show the previous profile's lease and keyboard for a frame.
  const pane = useSelector(store, snapshot =>
    snapshot.context.scopeKey === scopeKey ? snapshot.context.panes[terminal.id] : undefined
  );
  // Giving control back is a request, not a claim: `DETACH` goes out on the
  // write connection, the daemon closes it, and this viewer comes back as a
  // watcher. Without remembering the request, the reconnect would ask for the
  // write lease again and undo the release it just performed.
  const [releaseAsked, setReleaseAsked] = useState(false);
  // The client reconnects on its own with backoff; this is how a person asks
  // for it now rather than waiting the delay out.
  const [restartKey, setRestartKey] = useState(0);
  const observedLease = leaseFrom(terminal, pane, viewerId);
  const lease = readOnly
    ? {
        ...observedLease,
        canType: false,
        canTakeControl: false,
        requiresConfirmation: false,
        canRelease: false,
      }
    : observedLease;
  // The request is over the moment the daemon's own frames agree the lease has
  // moved — derived here rather than reset by an effect, so there is never a
  // render where the two disagree.
  const releasing = releaseAsked && lease.canType;
  const attachment = useTerminalAttachment({
    workspaceId,
    terminalId: terminal.id,
    scope: { profile },
    mode: !readOnly && lease.canType && !releasing ? "write" : "read",
    handleRef,
    socketFactory,
    restartKey,
  });

  return {
    pane,
    lease,
    attachment,
    handleRef,
    takeControl: force => {
      setReleaseAsked(false);
      attachment.requestTakeover(force);
    },
    releaseControl: () => {
      attachment.releaseControl();
      setReleaseAsked(true);
    },
    reconnect: () => setRestartKey(key => key + 1),
  };
}

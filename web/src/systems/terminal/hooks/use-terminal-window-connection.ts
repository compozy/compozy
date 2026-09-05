"use client";

import type { TerminalViewHandle } from "@compozy/ui";
import { useSelector } from "@xstate/store-react";
import { useRef, useState } from "react";

import { terminalScopeKey } from "../lib/terminal-scope-key";
import type { TerminalPaneState } from "../stores/terminal-store";
import type { TerminalInfo } from "../types";
import type { TerminalViewerIdentity } from "../types";
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
  viewer?: TerminalViewerIdentity | null;
  readOnly?: boolean;
  socketFactory?: TerminalAttachmentSocketFactory;
}

export interface TerminalWindowConnection {
  pane: TerminalPaneState | undefined;
  attachment: TerminalAttachment;
  handleRef: React.RefObject<TerminalViewHandle | null>;
  /** Starts the connection over now, rather than waiting out the backoff. */
  reconnect: () => void;
}

/**
 * One terminal's live connection. Interactive windows attach writable from the
 * start; only presentation surfaces use the read-only mode.
 */
export function useTerminalWindowConnection({
  terminal,
  workspaceId,
  profile,
  viewer,
  socketFactory,
  readOnly = false,
}: UseTerminalWindowConnectionOptions): TerminalWindowConnection {
  const store = useTerminalStore();
  const handleRef = useRef<TerminalViewHandle>(null);
  const scopeKey = terminalScopeKey(workspaceId, profile);
  // Pane state is only readable once the store agrees which scope it holds. A
  // terminal id can repeat across profiles, so reading a pane before the rebind
  // lands would show the previous profile's keyboard state for a frame.
  const pane = useSelector(store, snapshot =>
    snapshot.context.scopeKey === scopeKey ? snapshot.context.panes[terminal.id] : undefined
  );
  // The client reconnects on its own with backoff; this is how a person asks
  // for it now rather than waiting the delay out.
  const [restartKey, setRestartKey] = useState(0);
  const attachment = useTerminalAttachment({
    workspaceId,
    terminalId: terminal.id,
    scope: { profile },
    mode: readOnly ? "read" : "write",
    viewer,
    handleRef,
    socketFactory,
    restartKey,
  });

  return {
    pane,
    attachment,
    handleRef,
    reconnect: () => setRestartKey(key => key + 1),
  };
}

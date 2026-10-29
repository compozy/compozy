"use client";

import type { TerminalViewHandle } from "@compozy/ui";
import { useSelector } from "@xstate/store-react";
import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { terminalRetainedOutputQuery, terminalScope } from "../lib/query-options";

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
  // This body is keyed by terminal identity. Once started, the live stream owns
  // its end: the catalog can announce exit before the socket's final bytes arrive.
  const [startedLive] = useState(() => terminal.state === "running");
  // Match transcript previews: a pane that streamed retains its live scrollback.
  const [streamed, setStreamed] = useState(false);
  const needsRetainedOutput =
    terminal.state === "exited" && !streamed && (!startedLive || pane?.status === "closed");
  const output = useQuery({
    ...terminalRetainedOutputQuery(terminalScope(workspaceId, profile), terminal.id),
    enabled: needsRetainedOutput,
    staleTime: Infinity,
  });
  useEffect(() => {
    if (!needsRetainedOutput || !output.data) return;
    const handle = handleRef.current;
    if (!handle) return;
    handle.reset();
    void handle.write(output.data.content.replace(/\r?\n/g, "\r\n"));
  }, [needsRetainedOutput, output.data]);
  const attachment = useTerminalAttachment({
    workspaceId,
    terminalId: terminal.id,
    scope: { profile },
    mode: readOnly ? "read" : "write",
    viewer,
    handleRef,
    socketFactory,
    restartKey,
    enabled: startedLive,
    onOutputPainted: () => setStreamed(true),
  });

  return {
    pane:
      output.error && needsRetainedOutput
        ? {
            ...pane,
            terminalId: terminal.id,
            status: "closed",
            mode: terminal.mode,
            cols: null,
            rows: null,
            viewers: terminal.viewers,
            inputEnabled: false,
            gap: null,
            exit: pane?.exit ?? null,
            errorCode: "client_error",
            errorMessage: output.error.message,
          }
        : pane,
    attachment,
    handleRef,
    reconnect: () => {
      if (terminal.state === "exited") {
        if (needsRetainedOutput) void output.refetch();
      } else setRestartKey(key => key + 1);
    },
  };
}

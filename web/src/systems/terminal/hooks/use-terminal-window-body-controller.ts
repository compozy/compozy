"use client";

import { useState } from "react";

import type { TerminalInfo, TerminalViewerIdentity } from "../types";
import type { TerminalWindowActions } from "../lib/terminal-window-actions";
import type { TerminalAttachmentSocketFactory } from "./use-terminal-attachment";
import {
  useTerminalWindowConnection,
  type TerminalWindowConnection,
} from "./use-terminal-window-connection";

export interface UseTerminalWindowBodyControllerOptions {
  terminal: TerminalInfo;
  workspaceId: string;
  profile: string;
  viewerId: string | null;
  viewer?: TerminalViewerIdentity | null;
  readOnly: boolean;
  socketFactory?: TerminalAttachmentSocketFactory;
  actions: TerminalWindowActions;
}

export interface TerminalWindowBodyController {
  connection: TerminalWindowConnection;
  pendingTakeover: boolean;
  takeControl?: () => void;
  releaseControl?: () => void;
  stop?: () => void;
  stopRecording?: () => void;
  cancelTakeover: () => void;
  confirmTakeover: () => void;
}

/** Owns terminal gestures whose result is authoritative only after a stream frame. */
export function useTerminalWindowBodyController({
  terminal,
  workspaceId,
  profile,
  viewerId,
  viewer,
  readOnly,
  socketFactory,
  actions,
}: UseTerminalWindowBodyControllerOptions): TerminalWindowBodyController {
  const [pendingTakeover, setPendingTakeover] = useState(false);
  const stopRecording = actions.onStopRecording;
  const connection = useTerminalWindowConnection({
    terminal,
    workspaceId,
    profile,
    viewerId,
    viewer,
    socketFactory,
    readOnly,
  });

  return {
    connection,
    pendingTakeover,
    takeControl: readOnly
      ? undefined
      : () => {
          // Displacing a person asks first; displacing an agent never does.
          if (connection.lease.requiresConfirmation) {
            setPendingTakeover(true);
            return;
          }
          connection.takeControl(false);
        },
    releaseControl: readOnly ? undefined : connection.releaseControl,
    stop: readOnly ? undefined : () => actions.onStop(terminal.id),
    stopRecording: !readOnly && stopRecording ? () => stopRecording(terminal.id) : undefined,
    cancelTakeover: () => setPendingTakeover(false),
    confirmTakeover: () => {
      // Confirmed displacement is the only forced takeover there is.
      connection.takeControl(true);
      setPendingTakeover(false);
    },
  };
}

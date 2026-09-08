"use client";

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
  viewer?: TerminalViewerIdentity | null;
  readOnly: boolean;
  socketFactory?: TerminalAttachmentSocketFactory;
  actions: TerminalWindowActions;
}

export interface TerminalWindowBodyController {
  connection: TerminalWindowConnection;
  stop?: () => void;
  stopRecording?: () => void;
}

/** Owns the live connection and terminal actions bound to its id. */
export function useTerminalWindowBodyController({
  terminal,
  workspaceId,
  profile,
  viewer,
  readOnly,
  socketFactory,
  actions,
}: UseTerminalWindowBodyControllerOptions): TerminalWindowBodyController {
  const stopRecording = actions.onStopRecording;
  const connection = useTerminalWindowConnection({
    terminal,
    workspaceId,
    profile,
    viewer,
    socketFactory,
    readOnly,
  });

  return {
    connection,
    stop: readOnly ? undefined : () => actions.onStop(terminal.id),
    stopRecording: !readOnly && stopRecording ? () => stopRecording(terminal.id) : undefined,
  };
}

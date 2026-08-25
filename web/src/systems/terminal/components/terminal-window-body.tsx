"use client";

import type { TerminalEngineLoader } from "@compozy/ui";
import { useState } from "react";

import type { TerminalSocketFactory } from "../adapters/terminal-socket";
import { useTerminalWindowConnection } from "../hooks/use-terminal-window-connection";
import { exitNoticeFromTerminal } from "../lib/terminal-exit";
import { terminalInstanceKey } from "../lib/terminal-scope-key";
import type { TerminalInfo, TerminalInputRequest } from "../types";
import { TerminalHeader, type TerminalRecordingState } from "./terminal-header";
import { TerminalInputRequestCard } from "./terminal-input-request";
import { TerminalPane } from "./terminal-pane";
import { TerminalPipeLogPane } from "./terminal-pipe-log-pane";
import { TerminalTakeoverDialog } from "./terminal-takeover-dialog";
import type { TerminalWindowActions } from "./terminal-window-actions";

export interface TerminalWindowBodyProps {
  terminal: TerminalInfo;
  viewerId: string | null;
  workspaceId: string;
  profile: string;
  inputRequests: readonly TerminalInputRequest[];
  auditBlocked: boolean;
  exitRetentionMs?: number;
  recording: TerminalRecordingState | null;
  pipeOutput?: { lines: readonly string[]; firstLineNumber: number };
  actions: TerminalWindowActions;
  /** Opens the pinned journal tab, for a terminal that is no longer there. */
  onViewJournal: () => void;
  socketFactory?: TerminalSocketFactory;
  engineLoader?: TerminalEngineLoader;
}

/**
 * One terminal, live.
 *
 * The head, the grid and the questions pinned under it all read from the same
 * connection, because taking control and giving it back are frames on that
 * socket rather than calls to a parent. Nothing in the window claims either
 * happened before the daemon says so.
 */
export function TerminalWindowBody({
  terminal,
  viewerId,
  workspaceId,
  profile,
  inputRequests,
  auditBlocked,
  exitRetentionMs,
  recording,
  pipeOutput,
  actions,
  onViewJournal,
  socketFactory,
  engineLoader,
}: TerminalWindowBodyProps) {
  const [pendingTakeover, setPendingTakeover] = useState(false);
  const connection = useTerminalWindowConnection({
    terminal,
    workspaceId,
    profile,
    viewerId,
    socketFactory,
  });
  const { lease, pane, attachment, handleRef } = connection;

  return (
    <>
      <TerminalHeader
        grid={pane?.cols && pane?.rows ? { cols: pane.cols, rows: pane.rows } : null}
        lease={lease}
        onClose={() => actions.onCloseTerminal(terminal.id)}
        onReleaseControl={connection.releaseControl}
        onStop={() => actions.onStop(terminal.id)}
        onStopRecording={
          actions.onStopRecording ? () => actions.onStopRecording?.(terminal.id) : undefined
        }
        onTakeControl={() => {
          // Displacing a person asks first; displacing an agent never does.
          if (lease.requiresConfirmation) {
            setPendingTakeover(true);
            return;
          }
          connection.takeControl(false);
        }}
        recording={recording}
        terminal={terminal}
      />
      {terminal.mode === "pipe" ? (
        <TerminalPipeLogPane
          exit={exitNoticeFromTerminal(terminal)}
          firstLineNumber={pipeOutput?.firstLineNumber ?? 1}
          lines={pipeOutput?.lines ?? []}
          terminal={terminal}
        />
      ) : (
        <TerminalPane
          attachment={attachment}
          auditBlocked={auditBlocked}
          engineLoader={engineLoader}
          exitRetentionMs={exitRetentionMs}
          handleRef={handleRef}
          instanceId={terminalInstanceKey(workspaceId, profile, terminal.id)}
          lease={lease}
          onReconnect={connection.reconnect}
          onViewJournal={onViewJournal}
          pane={pane}
          selectionActions={{
            hasActiveSession: actions.hasActiveSession,
            onChooseSession: actions.onChooseSession,
            onCopy: selection => actions.onCopySelection(terminal.id, selection),
            onSendToConversation: selection => actions.onSendSelection(terminal.id, selection),
            onStartSession: selection => actions.onStartSession(terminal.id, selection),
          }}
          terminal={terminal}
        />
      )}
      {inputRequests.map(request => (
        <TerminalInputRequestCard
          canAnswerDirectly={lease.canType}
          key={request.id}
          onAnswer={input => actions.onAnswerInputRequest(request, input)}
          onReject={() => actions.onRejectInputRequest(request)}
          request={request}
        />
      ))}
      {pendingTakeover ? (
        <TerminalTakeoverDialog
          controllerName={lease.controllerName ?? "the current controller"}
          onCancel={() => setPendingTakeover(false)}
          onConfirm={() => {
            // Confirmed displacement is the only forced takeover there is.
            connection.takeControl(true);
            setPendingTakeover(false);
          }}
          open
          terminalId={terminal.id}
          terminalTitle={terminal.title}
        />
      ) : null}
    </>
  );
}

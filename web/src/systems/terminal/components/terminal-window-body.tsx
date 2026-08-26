"use client";

import type { TerminalEngineLoader } from "@compozy/ui";
import { useQuery } from "@tanstack/react-query";

import type { TerminalAttachmentSocketFactory } from "../hooks/use-terminal-attachment";
import { useTerminalWindowBodyController } from "../hooks/use-terminal-window-body-controller";
import { exitNoticeFromTerminal } from "../lib/terminal-exit";
import { terminalLeaseView } from "../lib/terminal-lease";
import { terminalPipeOutputQuery, terminalScope } from "../lib/query-options";
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
  readOnly?: boolean;
  inputRequests: readonly TerminalInputRequest[];
  auditBlocked: boolean;
  exitRetentionMs?: number;
  recording: TerminalRecordingState | null;
  pipeOutput?: { lines: readonly string[]; firstLineNumber: number };
  actions: TerminalWindowActions;
  /** Opens the pinned journal tab, for a terminal that is no longer there. */
  onViewJournal: () => void;
  socketFactory?: TerminalAttachmentSocketFactory;
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
export function TerminalWindowBody(props: TerminalWindowBodyProps) {
  return props.terminal.mode === "pipe" ? (
    <TerminalPipeWindowBody {...props} />
  ) : (
    <TerminalInteractiveWindowBody {...props} />
  );
}

function TerminalInteractiveWindowBody({
  terminal,
  viewerId,
  workspaceId,
  profile,
  readOnly = false,
  inputRequests,
  auditBlocked,
  exitRetentionMs,
  recording,
  actions,
  onViewJournal,
  socketFactory,
  engineLoader,
}: TerminalWindowBodyProps) {
  const controller = useTerminalWindowBodyController({
    terminal,
    workspaceId,
    profile,
    viewerId,
    socketFactory,
    readOnly,
    actions,
  });
  const { connection } = controller;
  const { lease, pane, attachment, handleRef } = connection;

  return (
    <>
      <TerminalHeader
        grid={pane?.cols && pane?.rows ? { cols: pane.cols, rows: pane.rows } : null}
        lease={lease}
        onReleaseControl={controller.releaseControl}
        onStop={controller.stop}
        onStopRecording={controller.stopRecording}
        onTakeControl={controller.takeControl}
        recording={recording}
        terminal={terminal}
      />
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
      {(readOnly ? [] : inputRequests).map(request => (
        <TerminalInputRequestCard
          canAnswerDirectly={lease.canType}
          key={request.id}
          onAnswer={input => actions.onAnswerInputRequest(request, input)}
          onReject={() => actions.onRejectInputRequest(request)}
          request={request}
        />
      ))}
      {controller.pendingTakeover ? (
        <TerminalTakeoverDialog
          controllerName={lease.controllerName ?? "the current controller"}
          onCancel={controller.cancelTakeover}
          onConfirm={controller.confirmTakeover}
          open
          terminalId={terminal.id}
          terminalTitle={terminal.title}
        />
      ) : null}
    </>
  );
}

function TerminalPipeWindowBody({
  terminal,
  viewerId,
  workspaceId,
  profile,
  readOnly = false,
  pipeOutput,
  actions,
}: TerminalWindowBodyProps) {
  const scope = terminalScope(workspaceId, profile);
  const output = useQuery({
    ...terminalPipeOutputQuery(scope, terminal.id),
    enabled: pipeOutput === undefined,
  });
  const captured = pipeOutput ?? {
    lines: splitTerminalOutput(output.data?.content ?? ""),
    firstLineNumber: 1,
  };
  const lease = terminalLeaseView({
    lease: terminal.lease,
    controller: terminal.controller,
    viewerId,
    mode: terminal.mode,
    capabilities: terminal.capabilities,
  });

  return (
    <>
      <TerminalHeader
        lease={lease}
        onClose={readOnly ? undefined : () => actions.onCloseTerminal(terminal.id)}
        terminal={terminal}
      />
      <TerminalPipeLogPane
        exit={exitNoticeFromTerminal(terminal)}
        firstLineNumber={captured.firstLineNumber}
        lines={captured.lines}
        terminal={terminal}
      />
    </>
  );
}

function splitTerminalOutput(content: string): string[] {
  const lines = content.split(/\r?\n/);
  if (lines.at(-1) === "") lines.pop();
  return lines;
}

"use client";

import { AlertCircle } from "lucide-react";
import { BlockLoading, Button, Empty, type TerminalEngineLoader } from "@compozy/ui";
import { useQuery } from "@tanstack/react-query";

import type { TerminalAttachmentSocketFactory } from "../hooks/use-terminal-attachment";
import { useTerminalWindowBodyController } from "../hooks/use-terminal-window-body-controller";
import { exitNoticeFromTerminal } from "../lib/terminal-exit";
import {
  terminalPipeOutputQuery,
  terminalScope,
  type TerminalProfileQueryScope,
} from "../lib/query-options";
import { terminalInstanceKey } from "../lib/terminal-scope-key";
import type { TerminalInfo, TerminalInputRequest, TerminalResolvedInputRequest } from "../types";
import { TerminalExpiredState, TerminalNotFoundState } from "./terminal-empty-states";
import { TerminalHeader, type TerminalRecordingState } from "./terminal-header";
import { TerminalInputRequestStack } from "./terminal-input-request-stack";
import { TerminalPane, type TerminalPaneSelectionActions } from "./terminal-pane";
import { TerminalPipeLogPane } from "./terminal-pipe-log-pane";
import type { TerminalWindowActions } from "../lib/terminal-window-actions";

const EMPTY_RESOLVED: readonly TerminalResolvedInputRequest[] = [];

export interface TerminalWindowBodyProps {
  terminal: TerminalInfo;
  viewerId: string | null;
  viewerToken?: string | null;
  workspaceId: string;
  profile: string;
  readOnly?: boolean;
  inputRequests: readonly TerminalInputRequest[];
  resolvedInputRequests?: readonly TerminalResolvedInputRequest[];
  inputRequestTitles?: ReadonlyMap<string, string>;
  exitRetentionMs?: number;
  recording: TerminalRecordingState | null;
  pipeOutput?: { lines: readonly string[]; firstLineNumber: number };
  actions: TerminalWindowActions;
  /** Opens the pinned journal tab, for a terminal that is no longer there. */
  onViewJournal: () => void;
  socketFactory?: TerminalAttachmentSocketFactory;
  engineLoader?: TerminalEngineLoader;
  hostChrome?: boolean;
  compact?: boolean;
  /** `[terminal].detached_ttl`, already phrased when known. */
  detachedTtl?: string;
  terminalCount?: number;
  limit?: number;
}

/**
 * One terminal, live.
 *
 * The head, grid, and questions pinned under it all share one connection.
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
  viewerToken,
  workspaceId,
  profile,
  readOnly = false,
  inputRequests,
  resolvedInputRequests = EMPTY_RESOLVED,
  inputRequestTitles,
  exitRetentionMs,
  recording,
  actions,
  onViewJournal,
  socketFactory,
  engineLoader,
  hostChrome = false,
  compact = false,
  detachedTtl,
  terminalCount,
  limit,
}: TerminalWindowBodyProps) {
  const controller = useTerminalWindowBodyController({
    terminal,
    workspaceId,
    profile,
    viewer: viewerId && viewerToken ? { id: viewerId, attachmentToken: viewerToken } : null,
    socketFactory,
    readOnly,
    actions,
  });
  const { connection } = controller;
  const { pane, attachment, handleRef } = connection;
  const newTerminal = readOnly ? undefined : (actions.onOpenTerminalTab ?? actions.onOpenTerminal);
  const goneCode = pane?.errorCode;
  if (goneCode === "terminal_expired" || goneCode === "terminal_not_found") {
    return (
      <>
        <TerminalHeader
          hostChrome={hostChrome}
          limit={limit}
          onNewTerminal={newTerminal}
          onViewJournal={onViewJournal}
          recording={recording}
          terminal={{ ...terminal, viewers: pane?.viewers ?? terminal.viewers }}
          terminalCount={terminalCount}
        />
        {goneCode === "terminal_expired" ? (
          <TerminalExpiredState
            idleFor={detachedTtl}
            onOpenTerminal={newTerminal}
            onViewJournal={onViewJournal}
          />
        ) : (
          <TerminalNotFoundState onOpenTerminal={newTerminal} onViewJournal={onViewJournal} />
        )}
      </>
    );
  }

  return (
    <>
      <TerminalHeader
        closePending={actions.closePending}
        hostChrome={hostChrome}
        limit={limit}
        onClose={readOnly ? undefined : () => actions.onCloseTerminal(terminal.id)}
        onNewTerminal={newTerminal}
        onStop={controller.stop}
        onStopRecording={controller.stopRecording}
        onViewJournal={onViewJournal}
        recording={recording}
        terminal={{ ...terminal, viewers: pane?.viewers ?? terminal.viewers }}
        terminalCount={terminalCount}
      />
      <TerminalPane
        attachment={attachment}
        compact={compact}
        engineLoader={engineLoader}
        exitRetentionMs={exitRetentionMs}
        handleRef={handleRef}
        instanceId={terminalInstanceKey(workspaceId, profile, terminal.id)}
        readOnly={readOnly}
        onReconnect={connection.reconnect}
        onViewJournal={onViewJournal}
        pane={pane}
        requestRegion={
          <div
            aria-live="polite"
            className="relative z-10 max-h-1/2 flex-none overflow-y-auto"
            role="status"
          >
            <TerminalInputRequestStack
              canAnswer={!readOnly}
              onAnswer={(request, input) => actions.onAnswerInputRequest(request, input)}
              onReject={request => actions.onRejectInputRequest(request)}
              pending={inputRequests}
              resolved={resolvedInputRequests}
              titles={inputRequestTitles}
            />
          </div>
        }
        selectionActions={terminalSelectionActions(actions, terminal.id)}
        terminal={terminal}
      />
    </>
  );
}

function TerminalPipeWindowBody({
  terminal,
  workspaceId,
  profile,
  readOnly = false,
  pipeOutput,
  actions,
  onViewJournal,
  hostChrome = false,
  terminalCount,
  limit,
}: TerminalWindowBodyProps) {
  const scope = terminalScope(workspaceId, profile);
  const newTerminal = readOnly ? undefined : (actions.onOpenTerminalTab ?? actions.onOpenTerminal);
  return (
    <>
      <TerminalHeader
        closePending={actions.closePending}
        hostChrome={hostChrome}
        limit={limit}
        onClose={readOnly ? undefined : () => actions.onCloseTerminal(terminal.id)}
        onNewTerminal={newTerminal}
        onSignal={
          readOnly || terminal.state !== "running" ? undefined : () => actions.onStop(terminal.id)
        }
        onViewJournal={onViewJournal}
        onWait={readOnly ? undefined : () => actions.onWait(terminal.id)}
        terminal={terminal}
        terminalCount={terminalCount}
      />
      {pipeOutput ? (
        <TerminalPipeLogPane
          exit={exitNoticeFromTerminal(terminal)}
          firstLineNumber={pipeOutput.firstLineNumber}
          lines={pipeOutput.lines}
          selectionActions={terminalSelectionActions(actions, terminal.id)}
          terminal={terminal}
        />
      ) : (
        <TerminalPipeFetchedOutput actions={actions} scope={scope} terminal={terminal} />
      )}
    </>
  );
}

function TerminalPipeFetchedOutput({
  actions,
  scope,
  terminal,
}: {
  actions: TerminalWindowActions;
  scope: TerminalProfileQueryScope;
  terminal: TerminalInfo;
}) {
  const output = useQuery(terminalPipeOutputQuery(scope, terminal.id));

  // Waiting and failing must never read as an empty log — an empty log is a
  // real state that means the command printed nothing.
  if (output.isPending) {
    return <BlockLoading className="flex-1" label="Loading the captured output" surface="bare" />;
  }
  if (output.error) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
        <Empty
          action={
            <Button onClick={() => void output.refetch()} size="sm" type="button" variant="outline">
              Retry
            </Button>
          }
          className="max-w-md"
          description={output.error instanceof Error ? output.error.message : undefined}
          icon={AlertCircle}
          title="Couldn't load the captured output"
        />
      </div>
    );
  }
  return (
    <TerminalPipeLogPane
      exit={exitNoticeFromTerminal(terminal)}
      firstLineNumber={1}
      lines={splitTerminalOutput(output.data.content)}
      selectionActions={terminalSelectionActions(actions, terminal.id)}
      terminal={terminal}
    />
  );
}

function terminalSelectionActions(
  actions: TerminalWindowActions,
  terminalId: string
): TerminalPaneSelectionActions {
  return {
    hasActiveSession: actions.hasActiveSession,
    onChooseSession: selection => actions.onChooseSession(terminalId, selection),
    onCopy: selection => actions.onCopySelection(terminalId, selection),
    onSendToConversation: selection => actions.onSendSelection(terminalId, selection),
    onStartSession: selection => actions.onStartSession(terminalId, selection),
  };
}

function splitTerminalOutput(content: string): string[] {
  const lines = content.split(/\r?\n/);
  if (lines.at(-1) === "") lines.pop();
  return lines;
}

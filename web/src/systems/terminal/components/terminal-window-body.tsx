"use client";

import { AlertCircle } from "lucide-react";
import { BlockLoading, Button, Empty, type TerminalEngineLoader } from "@compozy/ui";
import { useQuery } from "@tanstack/react-query";

import type { TerminalAttachmentSocketFactory } from "../hooks/use-terminal-attachment";
import { useTerminalWindowBodyController } from "../hooks/use-terminal-window-body-controller";
import { exitNoticeFromTerminal } from "../lib/terminal-exit";
import { terminalLeaseView } from "../lib/terminal-lease";
import { terminalPipeOutputQuery, terminalScope } from "../lib/query-options";
import { terminalInstanceKey } from "../lib/terminal-scope-key";
import type { TerminalInfo, TerminalInputRequest, TerminalResolvedInputRequest } from "../types";
import { TerminalExpiredState, TerminalNotFoundState } from "./terminal-empty-states";
import { TerminalHeader, type TerminalRecordingState } from "./terminal-header";
import { TerminalInputRequestStack } from "./terminal-input-request-stack";
import { TerminalPane } from "./terminal-pane";
import { TerminalPipeLogPane } from "./terminal-pipe-log-pane";
import { TerminalTakeoverDialog } from "./terminal-takeover-dialog";
import type { TerminalWindowActions } from "./terminal-window-actions";

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
    viewerId,
    viewer: viewerId && viewerToken ? { id: viewerId, attachmentToken: viewerToken } : null,
    socketFactory,
    readOnly,
    actions,
  });
  const { connection } = controller;
  const { lease, pane, attachment, handleRef } = connection;
  const newTerminal = readOnly ? undefined : (actions.onOpenTerminalTab ?? actions.onOpenTerminal);
  const goneCode = pane?.errorCode;
  if (goneCode === "terminal_expired" || goneCode === "terminal_not_found") {
    return (
      <>
        <TerminalHeader
          hostChrome={hostChrome}
          lease={lease}
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
            onOpenTerminal={actions.onOpenTerminal}
            onViewJournal={onViewJournal}
          />
        ) : (
          <TerminalNotFoundState
            onOpenTerminal={actions.onOpenTerminal}
            onViewJournal={onViewJournal}
          />
        )}
      </>
    );
  }

  return (
    <>
      <TerminalHeader
        closePending={actions.closePending}
        hostChrome={hostChrome}
        lease={lease}
        limit={limit}
        onClose={readOnly ? undefined : () => actions.onCloseTerminal(terminal.id)}
        onNewTerminal={newTerminal}
        onReleaseControl={controller.releaseControl}
        onStop={controller.stop}
        onStopRecording={controller.stopRecording}
        onTakeControl={controller.takeControl}
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
        lease={lease}
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
              canAnswerDirectly={lease.canType && !readOnly}
              onAnswer={(request, input) => actions.onAnswerInputRequest(request, input)}
              onReject={request => actions.onRejectInputRequest(request)}
              pending={inputRequests}
              resolved={resolvedInputRequests}
              titles={inputRequestTitles}
            />
          </div>
        }
        selectionActions={{
          hasActiveSession: actions.hasActiveSession,
          onChooseSession: selection => actions.onChooseSession(terminal.id, selection),
          onCopy: selection => actions.onCopySelection(terminal.id, selection),
          onSendToConversation: selection => actions.onSendSelection(terminal.id, selection),
          onStartSession: selection => actions.onStartSession(terminal.id, selection),
        }}
        terminal={terminal}
      />
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
  onViewJournal,
  hostChrome = false,
  terminalCount,
  limit,
}: TerminalWindowBodyProps) {
  const scope = terminalScope(workspaceId, profile);
  const output = useQuery({
    ...terminalPipeOutputQuery(scope, terminal.id),
    enabled: pipeOutput === undefined,
  });
  const newTerminal = readOnly ? undefined : (actions.onOpenTerminalTab ?? actions.onOpenTerminal);
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
        closePending={actions.closePending}
        hostChrome={hostChrome}
        lease={lease}
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
      {/* Waiting and failing must never read as an empty log — an empty log is
          a real state that means the command printed nothing. */}
      {pipeOutput === undefined && output.isPending ? (
        <BlockLoading className="flex-1" label="Loading the captured output" surface="bare" />
      ) : pipeOutput === undefined && output.error ? (
        <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
          <Empty
            action={
              <Button
                onClick={() => void output.refetch()}
                size="sm"
                type="button"
                variant="outline"
              >
                Retry
              </Button>
            }
            className="max-w-md"
            description={output.error instanceof Error ? output.error.message : undefined}
            icon={AlertCircle}
            title="Couldn't load the captured output"
          />
        </div>
      ) : (
        <TerminalPipeLogPane
          exit={exitNoticeFromTerminal(terminal)}
          firstLineNumber={pipeOutput?.firstLineNumber ?? 1}
          lines={pipeOutput?.lines ?? splitTerminalOutput(output.data?.content ?? "")}
          selectionActions={{
            hasActiveSession: actions.hasActiveSession,
            onChooseSession: selection => actions.onChooseSession(terminal.id, selection),
            onCopy: selection => actions.onCopySelection(terminal.id, selection),
            onSendToConversation: selection => actions.onSendSelection(terminal.id, selection),
            onStartSession: selection => actions.onStartSession(terminal.id, selection),
          }}
          terminal={terminal}
        />
      )}
    </>
  );
}

function splitTerminalOutput(content: string): string[] {
  const lines = content.split(/\r?\n/);
  if (lines.at(-1) === "") lines.pop();
  return lines;
}

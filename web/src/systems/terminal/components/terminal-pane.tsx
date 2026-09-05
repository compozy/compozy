"use client";

import {
  TerminalView,
  type TerminalEngineLoader,
  type TerminalSelectionRange,
  type TerminalViewHandle,
} from "@compozy/ui";
import type { ReactNode } from "react";

import type { TerminalAttachment } from "../hooks/use-terminal-attachment";
import { useTerminalPaneStatus } from "../hooks/use-terminal-pane-status";
import { useTerminalSelection } from "../hooks/use-terminal-selection";
import { terminalRetentionNote } from "../lib/terminal-exit";
import { terminalQuoteFromSelection } from "../lib/terminal-quote";
import { TERMINAL_MIN_COLS, TERMINAL_MIN_ROWS } from "../lib/terminal-wire";
import type { TerminalPaneState } from "../stores/terminal-store";
import type { TerminalInfo } from "../types";
import { TerminalConnectingLine } from "./terminal-connecting-line";
import { TerminalSelectionActions } from "./terminal-quote-block";
import { TerminalExitBar, TerminalSizeVoteBar } from "./terminal-exit-bar";
import { TerminalGapSeam, TerminalStreamNotice } from "./terminal-notices";

export interface TerminalPaneProps {
  terminal: TerminalInfo;
  /** Presentation attachments remain non-interactive. */
  readOnly?: boolean;
  /** The connection's identity key, so a switch cannot share a buffer. */
  instanceId: string;
  /** What the stream has said about this terminal so far. */
  pane: TerminalPaneState | undefined;
  /** The live connection, owned by the window body. */
  attachment: TerminalAttachment;
  handleRef: React.RefObject<TerminalViewHandle | null>;
  /** `[terminal].exit_retention`, in milliseconds. Omit when unknown. */
  exitRetentionMs?: number;
  /** Replaces the emulator. Tests and playback harnesses only. */
  engineLoader?: TerminalEngineLoader;
  /** The gesture a non-empty selection offers. Omit to render no actions. */
  selectionActions?: TerminalPaneSelectionActions;
  onSelectionChange?: (selection: TerminalSelectionRange | null) => void;
  onReconnect?: () => void;
  /** Opens the journal when the terminal itself is gone. */
  onViewJournal?: () => void;
  /** Input-request cards, pinned on the same surface as the grid. */
  requestRegion?: ReactNode;
  /**
   * The window has dropped below the anatomy that can host a full grid.
   * Proposals clamp to the protocol minimum rather than a degenerate fit.
   */
  compact?: boolean;
}

/**
 * What this pane's selection can become.
 *
 * Every callback that acts on the selection is handed the range the emulator
 * reported — the terminal, the first and last line, and the text — because a
 * quote that cannot name its lines is not the same artifact `compozy terminal
 * quote --lines A-B` produces.
 */
export interface TerminalPaneSelectionActions {
  hasActiveSession: boolean;
  onSendToConversation: (selection: TerminalSelectionRange) => void;
  onChooseSession: (selection: TerminalSelectionRange) => void;
  onStartSession: (selection: TerminalSelectionRange) => void;
  onCopy: (selection: TerminalSelectionRange) => void;
}

/**
 * One terminal on screen.
 *
 * The emulator paints; the connection decides what it paints. Local input is
 * offered after replay completes unless this is an explicit presentation view.
 */
export function TerminalPane({
  terminal,
  readOnly: presentationOnly = false,
  instanceId,
  pane,
  attachment,
  handleRef,
  exitRetentionMs,
  engineLoader,
  selectionActions,
  onSelectionChange,
  onReconnect,
  onViewJournal,
  requestRegion,
  compact = false,
}: TerminalPaneProps) {
  // A selection is only worth acting on while it exists, so the actions appear
  // with it and leave with it — the range comes from the emulator rather than
  // from a count of newlines, so the numbers match what `--lines` would take.
  const { readSelection, selection } = useTerminalSelection(handleRef, onSelectionChange);
  const readOnly = presentationOnly || !(pane?.inputEnabled ?? false);
  const display = useTerminalPaneStatus(terminal, pane);

  return (
    <div
      className="flex min-h-0 min-w-0 flex-1 flex-col bg-terminal-bg"
      data-compact={compact ? "true" : undefined}
      data-testid={`terminal-pane-${terminal.id}`}
    >
      <div className="relative min-h-0 min-w-0 flex-1">
        {pane?.gap ? <TerminalGapSeam gap={pane.gap} /> : null}
        <TerminalView
          aria-label={terminal.title}
          className={
            display.awaitingFirstFrame
              ? "invisible px-3.5 pt-2.5 pb-3 font-mono text-code-block tracking-mono"
              : "px-3.5 pt-2.5 pb-3 font-mono text-code-block tracking-mono"
          }
          {...(engineLoader ? { engineLoader } : {})}
          handleRef={handleRef}
          instanceId={instanceId}
          onData={attachment.sendInput}
          onProposeDimensions={dimensions => {
            const cols = compact ? TERMINAL_MIN_COLS : dimensions.cols;
            const rows = compact ? TERMINAL_MIN_ROWS : dimensions.rows;
            attachment.proposeDimensions(cols, rows);
          }}
          onSelectionChange={readSelection}
          readOnly={readOnly}
          screenReaderMode
        />
        <TerminalPaneConnectionOverlay
          awaitingFirstFrame={display.awaitingFirstFrame}
          show={display.showConnecting}
          status={display.status}
        />
      </div>
      <TerminalPaneSelectionBar
        actions={selectionActions}
        selection={selection}
        terminalId={terminal.id}
      />
      {requestRegion}
      <TerminalPaneStreamStatus
        onReconnect={onReconnect}
        onViewJournal={onViewJournal}
        pane={pane}
      />
      {display.exit ? (
        <TerminalExitBar
          exit={display.exit}
          retentionNote={terminalRetentionNote(terminal, exitRetentionMs, display.retentionNow)}
          terminal={terminal}
        />
      ) : display.showSizeVote && display.settledCols !== null && display.settledRows !== null ? (
        <TerminalSizeVoteBar cols={display.settledCols} rows={display.settledRows} />
      ) : null}
    </div>
  );
}

function TerminalPaneConnectionOverlay({
  awaitingFirstFrame,
  show,
  status,
}: {
  awaitingFirstFrame: boolean;
  show: boolean;
  status: TerminalPaneState["status"];
}) {
  if (!show) return null;
  return (
    // The emulator canvas forms its own stacking context, so the line needs an
    // explicit layer to actually paint above the cells.
    <div
      className={
        awaitingFirstFrame
          ? "absolute inset-0 z-10 bg-terminal-bg"
          : "pointer-events-none absolute inset-x-0 top-0 z-10"
      }
    >
      <TerminalConnectingLine status={status} />
    </div>
  );
}

function TerminalPaneSelectionBar({
  actions,
  selection,
  terminalId,
}: {
  actions?: TerminalPaneSelectionActions;
  selection: TerminalSelectionRange | null;
  terminalId: string;
}) {
  if (!selection || !actions) return null;
  return (
    <TerminalSelectionActions
      hasActiveSession={actions.hasActiveSession}
      onChooseSession={() => actions.onChooseSession(selection)}
      onCopy={() => actions.onCopy(selection)}
      onSendToConversation={() => actions.onSendToConversation(selection)}
      onStartSession={() => actions.onStartSession(selection)}
      quote={terminalQuoteFromSelection(terminalId, selection)}
    />
  );
}

function TerminalPaneStreamStatus({
  onReconnect,
  onViewJournal,
  pane,
}: Pick<TerminalPaneProps, "onReconnect" | "onViewJournal" | "pane">) {
  if (!pane?.errorCode) return null;
  return (
    <TerminalStreamNotice
      code={pane.errorCode}
      message={pane.errorMessage ?? pane.errorCode}
      onReconnect={onReconnect}
      onViewJournal={onViewJournal}
    />
  );
}

"use client";

import {
  TerminalView,
  type TerminalEngineLoader,
  type TerminalSelectionRange,
  type TerminalViewHandle,
} from "@compozy/ui";
import { useEffect, useState, type ReactNode } from "react";

import type { TerminalAttachment } from "../hooks/use-terminal-attachment";
import { exitNoticeFromTerminal, terminalRetentionNote } from "../lib/terminal-exit";
import type { TerminalLeaseView } from "../lib/terminal-lease";
import { terminalQuoteFromSelection } from "../lib/terminal-quote";
import { TERMINAL_MIN_COLS, TERMINAL_MIN_ROWS } from "../lib/terminal-wire";
import type { TerminalPaneState } from "../stores/terminal-store";
import type { TerminalInfo } from "../types";
import { TerminalConnectingLine } from "./terminal-connecting-line";
import { TerminalSelectionActions } from "./terminal-quote-block";
import { TerminalExitBar, TerminalSizeVoteBar } from "./terminal-exit-bar";
import { TerminalAuditBlockedBar, TerminalGapSeam, TerminalStreamNotice } from "./terminal-notices";

const RETENTION_REFRESH_MS = 30_000;

export interface TerminalPaneProps {
  terminal: TerminalInfo;
  lease: TerminalLeaseView;
  /** The connection's identity key, so a switch cannot share a buffer. */
  instanceId: string;
  /** What the stream has said about this terminal so far. */
  pane: TerminalPaneState | undefined;
  /** The live connection, owned by the window body. */
  attachment: TerminalAttachment;
  handleRef: React.RefObject<TerminalViewHandle | null>;
  auditBlocked?: boolean;
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
 * offered only while the daemon's lease says this viewer may write and the
 * stream has not gated it — the pane never opens the keyboard on its own
 * reading of the situation.
 */
export function TerminalPane({
  terminal,
  lease,
  instanceId,
  pane,
  attachment,
  handleRef,
  auditBlocked = false,
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
  const [selection, setSelection] = useState<TerminalSelectionRange | null>(null);
  const readSelection = () => {
    const range = handleRef.current?.getSelectionRange() ?? null;
    const next = range && range.text.trim() !== "" ? range : null;
    setSelection(next);
    onSelectionChange?.(next);
  };
  const readOnly = !lease.canType || !(pane?.inputEnabled ?? false);
  // A terminal that exited before this pane attached has no live EXIT frame to
  // observe. Its outcome is still the daemon's, recorded on the terminal, so the
  // bar reads from there when the stream has nothing newer to say.
  const exit = pane?.exit ?? exitNoticeFromTerminal(terminal);
  const hasExit = exit !== null;
  // The retention countdown is wall-clock truth: render stays pure by reading a
  // ticking state, and the tick exists only while an exit bar shows it.
  const [retentionNow, setRetentionNow] = useState(() => Date.now());
  useEffect(() => {
    if (!hasExit) return undefined;
    const timer = window.setInterval(() => setRetentionNow(Date.now()), RETENTION_REFRESH_MS);
    return () => window.clearInterval(timer);
  }, [hasExit]);

  const status = pane?.status ?? "connecting";
  const awaitingFirstFrame = status === "connecting" || status === "idle";
  const showConnecting =
    status === "connecting" || status === "reconnecting" || status === "resyncing";
  const viewers = pane?.viewers ?? terminal.viewers;
  const settledCols = pane?.status === "connected" ? pane.cols : null;
  const settledRows = pane?.status === "connected" ? pane.rows : null;
  const sizeVoteVisible = settledCols !== null && settledRows !== null && viewers > 1;

  return (
    <div
      className="flex min-h-0 min-w-0 flex-1 flex-col bg-terminal-bg"
      data-audit-blocked={auditBlocked ? "true" : undefined}
      data-compact={compact ? "true" : undefined}
      data-testid={`terminal-pane-${terminal.id}`}
    >
      <div className="relative min-h-0 min-w-0 flex-1">
        {pane?.gap ? <TerminalGapSeam gap={pane.gap} /> : null}
        <TerminalView
          aria-label={terminal.title}
          className={
            awaitingFirstFrame
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
        {showConnecting ? (
          <div
            className={
              awaitingFirstFrame ? "absolute inset-0 bg-terminal-bg" : "absolute inset-x-0 top-0"
            }
          >
            <TerminalConnectingLine status={status} />
          </div>
        ) : null}
      </div>
      {selection && selectionActions ? (
        <TerminalSelectionActions
          hasActiveSession={selectionActions.hasActiveSession}
          onChooseSession={() => selectionActions.onChooseSession(selection)}
          onCopy={() => selectionActions.onCopy(selection)}
          onSendToConversation={() => selectionActions.onSendToConversation(selection)}
          onStartSession={() => selectionActions.onStartSession(selection)}
          quote={terminalQuoteFromSelection(terminal.id, selection)}
        />
      ) : null}
      {requestRegion}
      {pane?.errorCode ? (
        <TerminalStreamNotice
          code={pane.errorCode}
          // The daemon's own sentence when there is one; the code is the last
          // resort, not the default.
          message={pane.errorMessage ?? pane.errorCode}
          onReconnect={onReconnect}
          onViewJournal={onViewJournal}
        />
      ) : null}
      {auditBlocked ? <TerminalAuditBlockedBar /> : null}
      {exit ? (
        <TerminalExitBar
          exit={exit}
          retentionNote={terminalRetentionNote(terminal, exitRetentionMs, retentionNow)}
          terminal={terminal}
        />
      ) : sizeVoteVisible && settledCols !== null && settledRows !== null ? (
        <TerminalSizeVoteBar cols={settledCols} rows={settledRows} />
      ) : null}
    </div>
  );
}

import { useState } from "react";
import type { TerminalSelectionRange } from "@compozy/ui";

import { terminalQuoteFromSelection } from "../lib/terminal-quote";
import type { TerminalExitNotice, TerminalInfo } from "../types";
import { TerminalExitBar } from "./terminal-exit-bar";
import { TerminalSelectionActions } from "./terminal-quote-block";
import type { TerminalPaneSelectionActions } from "./terminal-pane";

export interface TerminalPipeLogPaneProps {
  terminal: TerminalInfo;
  /** The captured output, already fetched as a tail. */
  lines: readonly string[];
  /** Line number of the first line shown, so the gutter stays truthful. */
  firstLineNumber: number;
  exit?: TerminalExitNotice | null;
  selectionActions?: TerminalPaneSelectionActions;
}

/**
 * A command's output, not a prompt.
 *
 * A pipe terminal was born from a command and can never be typed into, so it
 * renders as a numbered log: no cursor, no input line, no control affordances.
 * Typing here is not a disabled option — it does not exist. Selection uses the
 * same quote actions as the live grid, with `firstLineNumber` as the origin.
 */
export function TerminalPipeLogPane({
  terminal,
  lines,
  firstLineNumber,
  exit,
  selectionActions,
}: TerminalPipeLogPaneProps) {
  const [selection, setSelection] = useState<TerminalSelectionRange | null>(null);
  return (
    <div
      className="flex min-h-0 min-w-0 flex-1 flex-col bg-terminal-bg"
      data-testid={`terminal-pipe-pane-${terminal.id}`}
    >
      <div
        aria-label={`${terminal.title} — command output, read-only`}
        className="min-h-0 flex-1 overflow-auto px-3.5 pt-2.5 pb-3 font-mono text-code-block tracking-mono text-terminal-ansi-7"
        onMouseUp={event =>
          setSelection(pipeSelectionFromEvent(event.currentTarget, firstLineNumber))
        }
        role="log"
      >
        {lines.map((line, index) => (
          <div
            className="grid grid-cols-[2.125rem_minmax(0,1fr)] gap-2.5"
            data-line={firstLineNumber + index}
            key={`${firstLineNumber + index}`}
          >
            <span className="text-right text-terminal-ansi-8 select-none">
              {firstLineNumber + index}
            </span>
            <span className="break-all whitespace-pre-wrap">{line}</span>
          </div>
        ))}
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
      {exit ? <TerminalExitBar exit={exit} terminal={terminal} /> : null}
    </div>
  );
}

function pipeSelectionFromEvent(
  root: HTMLElement,
  firstLineNumber: number
): TerminalSelectionRange | null {
  const native = window.getSelection();
  if (!native || native.isCollapsed || !root.contains(native.anchorNode)) return null;
  const text = native.toString();
  if (text.trim() === "") return null;
  const start = lineNumberFromNode(native.anchorNode, firstLineNumber);
  const end = lineNumberFromNode(native.focusNode, firstLineNumber);
  return {
    startLine: Math.min(start, end),
    endLine: Math.max(start, end),
    text,
  };
}

function lineNumberFromNode(node: Node | null, fallback: number): number {
  const element = node instanceof Element ? node : node?.parentElement;
  const line = element?.closest("[data-line]")?.getAttribute("data-line");
  const parsed = line === null || line === undefined ? Number.NaN : Number(line);
  return Number.isFinite(parsed) ? parsed : fallback;
}

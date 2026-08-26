import { TerminalExitBar } from "./terminal-exit-bar";
import type { TerminalExitNotice, TerminalInfo } from "../types";

export interface TerminalPipeLogPaneProps {
  terminal: TerminalInfo;
  /** The captured output, already fetched as a tail. */
  lines: readonly string[];
  /** Line number of the first line shown, so the gutter stays truthful. */
  firstLineNumber: number;
  exit?: TerminalExitNotice | null;
}

/**
 * A command's output, not a prompt.
 *
 * A pipe terminal was born from a command and can never be typed into, so it
 * renders as a numbered log: no cursor, no input line, no control affordances.
 * Typing here is not a disabled option — it does not exist.
 */
export function TerminalPipeLogPane({
  terminal,
  lines,
  firstLineNumber,
  exit,
}: TerminalPipeLogPaneProps) {
  return (
    <div
      className="flex min-h-0 min-w-0 flex-1 flex-col bg-terminal-bg"
      data-testid={`terminal-pipe-pane-${terminal.id}`}
    >
      <div
        aria-label={`${terminal.title} — command output, read-only`}
        className="min-h-0 flex-1 overflow-auto px-3.5 pt-2.5 pb-3 font-mono text-[12.5px] leading-[1.5] text-terminal-ansi-7"
        role="log"
      >
        {lines.map((line, index) => (
          <div
            className="grid grid-cols-[2.125rem_minmax(0,1fr)] gap-2.5"
            key={`${firstLineNumber + index}`}
          >
            <span className="text-right text-terminal-ansi-8 select-none">
              {firstLineNumber + index}
            </span>
            <span className="break-all whitespace-pre-wrap">{line}</span>
          </div>
        ))}
      </div>
      {exit ? <TerminalExitBar exit={exit} terminal={terminal} /> : null}
    </div>
  );
}

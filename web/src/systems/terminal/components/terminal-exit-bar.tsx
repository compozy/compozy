import { Scaling } from "lucide-react";

import { MonoId, Pill, Time } from "@compozy/ui";

import { terminalExitCopy } from "../lib/terminal-copy";
import type { TerminalExitNotice, TerminalInfo } from "../types";

export interface TerminalExitBarProps {
  exit: TerminalExitNotice;
  terminal: TerminalInfo;
  /** How long the screen stays readable, already phrased. */
  retentionNote?: string;
}

/**
 * How the run ended, pinned under the grid.
 *
 * Only zero earns colour: a non-zero code is information, not an emergency, and
 * a cause the daemon could not verify renders as unknown rather than as an
 * invented code.
 */
export function TerminalExitBar({ exit, terminal, retentionNote }: TerminalExitBarProps) {
  const copy = terminalExitCopy(exit);
  return (
    <div
      className="flex min-h-8 flex-none items-center gap-2.5 border-line border-t bg-canvas px-3.5 text-eyebrow text-subtle"
      data-testid="terminal-exit-bar"
      role="status"
    >
      <Pill size="xs" tone={copy.tone === "success" ? "success" : "neutral"}>
        {copy.label}
      </Pill>
      <MonoId size="sm" value={copy.code} />
      {copy.note ? <span>{copy.note}</span> : null}
      {retentionNote ? <span>{retentionNote}</span> : null}
      {terminal.exit?.at ? (
        <Time className="ml-auto font-mono text-micro text-faint" iso={terminal.exit.at} />
      ) : null}
    </div>
  );
}

export interface TerminalSizeVoteBarProps {
  cols: number;
  rows: number;
}

/**
 * Why the grid is the size it is, said only while it can surprise.
 *
 * The daemon sizes a shared terminal to the smallest window among people who
 * can type, so with several viewers the number on screen is frequently not the
 * one this window proposed. The bar renders machine truth demoted to micro
 * mono — the daemon never names whose window won, so neither does this line.
 */
export function TerminalSizeVoteBar({ cols, rows }: TerminalSizeVoteBarProps) {
  return (
    <div
      className="flex min-h-8 flex-none items-center gap-1.25 border-line border-t bg-canvas px-3.5 font-mono text-micro text-faint"
      data-testid="terminal-size-vote"
    >
      <Scaling aria-hidden="true" className="size-3" />
      <span>
        sized to the smallest window that can type · {cols}×{rows}
      </span>
    </div>
  );
}

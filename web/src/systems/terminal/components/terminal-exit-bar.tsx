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
      className="flex min-h-8 flex-none items-center gap-2.5 border-line border-t bg-canvas px-3.5 text-form-input text-subtle"
      data-testid="terminal-exit-bar"
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

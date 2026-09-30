import { Eye, FileText, TerminalSquare } from "lucide-react";

import { Pill, StateGlyph, type StateGlyphState, useTopbarSlot } from "@compozy/ui";

import { terminalDisplayTitle } from "../lib/terminal-copy";
import type { TerminalInfo } from "../types";
import { TerminalHeaderActions, TerminalWindowVerbs } from "./terminal-header-actions";

export interface TerminalRecordingState {
  /** Elapsed capture time, already formatted as `m:ss`. */
  elapsed: string;
}

export interface TerminalHeaderProps {
  terminal: TerminalInfo;
  recording?: TerminalRecordingState | null;
  /** How many terminals this project has, for the cap trail. */
  terminalCount?: number;
  /** The per-project cap, from `[terminal].max_per_workspace`. */
  limit?: number;
  onStop?: () => void;
  onSignal?: () => void;
  onWait?: () => void;
  onStopRecording?: () => void;
  /** Opens another terminal. */
  onNewTerminal?: () => void;
  /** Reveals the journal overlay. */
  onViewJournal?: () => void;
  /**
   * When true, identity and ≤2 actions publish into the OS window head
   * instead of drawing a second identity row under the deck.
   */
  hostChrome?: boolean;
}

function terminalCapCount(terminalCount: number | undefined, limit: number | undefined) {
  if (terminalCount === undefined || limit === undefined || terminalCount < limit || limit <= 0) {
    return null;
  }
  return (
    <span data-testid="terminal-cap-count">
      {terminalCount} of {limit}
    </span>
  );
}

/**
 * The head's leading state mark. Only a clean exit earns the done check; a
 * failing run stays neutral (information, not an emergency — see
 * `terminalExitCopy`), and a signal reads as stopped.
 */
function terminalStateGlyph(terminal: TerminalInfo): StateGlyphState {
  if (terminal.state === "running") return "running";
  if (terminal.exit?.cause === "signaled") return "stopped";
  if (terminal.exit?.cause === "exited" && terminal.exit.code === 0) return "done";
  return "idle";
}

/**
 * The terminal's identity row.
 *
 * The name is stated once and at most two actions trail it — the head is where
 * a person orients, not where every verb lives.
 */
export function TerminalHeader({
  terminal,
  recording,
  terminalCount,
  limit,
  onStop,
  onSignal,
  onWait,
  onStopRecording,
  onNewTerminal,
  onViewJournal,
  hostChrome = false,
}: TerminalHeaderProps) {
  const isPipe = terminal.mode === "pipe";
  const capCount = terminalCapCount(terminalCount, limit);
  // The raw terminal id lives in the journal detail; the head only shows the cap count.
  const identityCount = capCount ?? undefined;
  const actions = (
    <>
      <TerminalHeaderActions
        isPipe={isPipe}
        onSignal={onSignal}
        onStop={onStop}
        onStopRecording={onStopRecording}
        onWait={onWait}
        recording={recording}
      />
      <TerminalWindowVerbs onNewTerminal={onNewTerminal} onViewJournal={onViewJournal} />
    </>
  );
  const status = (
    <>
      {recording ? (
        <Pill data-testid="terminal-recording-chip" size="sm" tone="neutral">
          <Pill.Dot pulse tone="danger" />
          Recording {recording.elapsed}
        </Pill>
      ) : null}
      {isPipe ? (
        <Pill data-testid="terminal-pipe-chip" size="sm" tone="neutral">
          read-only log
        </Pill>
      ) : null}
      {isPipe || terminal.viewers <= 1 ? null : (
        <Pill
          aria-label={`${terminal.viewers} ${terminal.viewers === 1 ? "viewer" : "viewers"}`}
          data-testid="terminal-viewers"
          mono
          size="sm"
          tone="neutral"
        >
          <Eye aria-hidden="true" className="size-3" />
          {terminal.viewers}
        </Pill>
      )}
    </>
  );
  useTopbarSlot(
    hostChrome
      ? {
          glyph: <StateGlyph size="sm" state={terminalStateGlyph(terminal)} />,
          glyphPresentation: "state",
          crumb: terminalDisplayTitle(terminal),
          count: identityCount,
          status,
          actions,
        }
      : null
  );
  if (hostChrome) return null;
  return (
    <header
      className="flex min-h-window-head flex-none items-center gap-2.5 border-line border-b bg-canvas px-4"
      data-testid="terminal-header"
    >
      <span className="flex min-w-0 items-center gap-2">
        {isPipe ? (
          <FileText aria-hidden="true" className="size-3.5 text-muted" />
        ) : (
          <TerminalSquare aria-hidden="true" className="size-3.5 text-muted" />
        )}
        <span className="truncate font-medium text-fg text-ws-name tracking-tight">
          {terminalDisplayTitle(terminal)}
        </span>
        {identityCount}
      </span>
      <span aria-hidden="true" className="min-w-2 flex-1" />
      <div className="flex flex-none items-center gap-2">
        {status}
        {actions}
      </div>
    </header>
  );
}

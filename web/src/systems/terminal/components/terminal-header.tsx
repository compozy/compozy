import { FileText, TerminalSquare } from "lucide-react";

import { MonoId, Pill, useTopbarSlot } from "@compozy/ui";

import type { TerminalLeaseView } from "../lib/terminal-lease";
import type { TerminalInfo } from "../types";
import { TerminalHeaderActions, TerminalWindowVerbs } from "./terminal-header-actions";
import { TerminalLeaseBadge } from "./terminal-lease-badge";

export interface TerminalRecordingState {
  /** Elapsed capture time, already formatted as `m:ss`. */
  elapsed: string;
}

export interface TerminalHeaderProps {
  terminal: TerminalInfo;
  lease: TerminalLeaseView;
  recording?: TerminalRecordingState | null;
  /** How many terminals this project has, for the cap trail. */
  terminalCount?: number;
  /** The per-project cap, from `[terminal].max_per_workspace`. */
  limit?: number;
  onTakeControl?: () => void;
  onReleaseControl?: () => void;
  onStop?: () => void;
  onClose?: () => void;
  onSignal?: () => void;
  onWait?: () => void;
  onStopRecording?: () => void;
  /** Opens another terminal. A window-level verb, not a lease action. */
  onNewTerminal?: () => void;
  /** Reveals the journal overlay. */
  onViewJournal?: () => void;
  /** True while a close is already on its way to the daemon. */
  closePending?: boolean;
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
 * The terminal's identity row.
 *
 * The name is stated once, the chip says who is in control, and at most two
 * actions trail it — the head is where a person orients, not where every verb
 * lives. Take control is the single accent affordance in terminal chrome.
 */
export function TerminalHeader({
  terminal,
  lease,
  recording,
  terminalCount,
  limit,
  onTakeControl,
  onReleaseControl,
  onStop,
  onClose,
  onSignal,
  onWait,
  onStopRecording,
  onNewTerminal,
  onViewJournal,
  closePending = false,
  hostChrome = false,
}: TerminalHeaderProps) {
  const isPipe = terminal.mode === "pipe";
  const capCount = terminalCapCount(terminalCount, limit);
  const identityCount = capCount ?? <MonoId size="sm" value={terminal.id} />;
  const actions = (
    <>
      <TerminalHeaderActions
        closePending={closePending}
        isPipe={isPipe}
        lease={lease}
        onClose={onClose}
        onReleaseControl={onReleaseControl}
        onSignal={onSignal}
        onStop={onStop}
        onStopRecording={onStopRecording}
        onWait={onWait}
        recording={recording}
        onTakeControl={onTakeControl}
      />
      <TerminalWindowVerbs onNewTerminal={onNewTerminal} onViewJournal={onViewJournal} />
    </>
  );
  const status = (
    <>
      {recording ? (
        <Pill data-testid="terminal-recording-chip" mono size="sm" tone="neutral">
          <Pill.Dot pulse tone="danger" />
          rec {recording.elapsed}
        </Pill>
      ) : null}
      {isPipe ? (
        <Pill data-testid="terminal-pipe-chip" size="sm" tone="neutral">
          read-only log
        </Pill>
      ) : null}
      <TerminalLeaseBadge view={lease} viewers={isPipe ? undefined : terminal.viewers} />
    </>
  );
  useTopbarSlot(
    hostChrome
      ? {
          glyph: isPipe ? <FileText /> : <TerminalSquare />,
          crumb: terminal.title,
          count: identityCount,
          status,
          actions,
        }
      : null
  );
  if (hostChrome) return null;
  return (
    <header
      className="flex min-h-11 flex-none items-center gap-2.5 border-line border-b bg-canvas px-3"
      data-testid="terminal-header"
    >
      <span className="flex min-w-0 items-center gap-2">
        {isPipe ? (
          <FileText aria-hidden="true" className="size-3.5 text-muted" />
        ) : (
          <TerminalSquare aria-hidden="true" className="size-3.5 text-muted" />
        )}
        <span className="truncate font-semibold text-fg-strong text-ws-name tracking-tight">
          {terminal.title}
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

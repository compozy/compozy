import { CircleStop, Ellipsis, FileText, Plus, ScrollText, TerminalSquare } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  MonoId,
  Pill,
  Separator,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  useTopbarSlot,
} from "@compozy/ui";

import type { TerminalLeaseView } from "../lib/terminal-lease";
import type { TerminalInfo } from "../types";
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

/** Publishes the journal's identity into the OS window head. */
export function TerminalJournalHostChrome({
  hostChrome,
  projectLabel,
  onBack,
}: {
  hostChrome: boolean;
  projectLabel?: string;
  /** Returns to the terminal underneath the overlay. */
  onBack?: () => void;
}) {
  useTopbarSlot(
    hostChrome
      ? {
          glyph: <ScrollText />,
          crumb: "Journal",
          status: projectLabel ? (
            <span className="truncate text-badge text-subtle">{projectLabel}</span>
          ) : undefined,
          ...(onBack ? { onBack } : {}),
        }
      : null
  );
  return null;
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

/**
 * At most two trailing actions, set off from the chips by a hairline.
 *
 * On a pipe terminal the interactive verbs are absent rather than disabled: a
 * greyed-out Take control would still claim the feature exists here. Wait and
 * Close stay on the head; Signal moves to overflow so the head never grows a
 * third verb.
 */
function TerminalHeaderActions({
  isPipe,
  lease,
  recording,
  closePending,
  onTakeControl,
  onReleaseControl,
  onStop,
  onClose,
  onSignal,
  onWait,
  onStopRecording,
}: TerminalHeaderActionsProps) {
  if (isPipe) {
    return (
      <TerminalPipeHeaderActions
        closePending={closePending}
        onClose={onClose}
        onSignal={onSignal}
        onWait={onWait}
      />
    );
  }
  const takeControl =
    lease.canTakeControl && onTakeControl ? (
      <Button data-testid="terminal-take-control" onClick={onTakeControl} size="sm" type="button">
        Take control
      </Button>
    ) : null;
  const releaseControl =
    lease.canRelease && onReleaseControl ? (
      <Button
        data-testid="terminal-release-control"
        onClick={onReleaseControl}
        size="sm"
        type="button"
        variant="ghost"
      >
        Release control
      </Button>
    ) : null;
  // Recording does not touch the lease, so it never takes the lease action's
  // place. Stopping the recording is ghost text; danger stays on the rec dot.
  const quietAction =
    recording && onStopRecording ? (
      <Button
        data-testid="terminal-stop-recording"
        onClick={onStopRecording}
        size="sm"
        type="button"
        variant="ghost"
      >
        Stop recording
      </Button>
    ) : onStop && !lease.canTakeControl ? (
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              aria-label="Stop"
              data-testid="terminal-stop"
              onClick={onStop}
              size="icon-sm"
              type="button"
              variant="ghost"
            />
          }
        >
          <CircleStop aria-hidden="true" className="size-3.5 text-danger" />
        </TooltipTrigger>
        <TooltipContent side="bottom">Stop</TooltipContent>
      </Tooltip>
    ) : null;
  // Ending the session is deliberate, so it lives one step away — never as a
  // third verb beside the lease pair.
  const overflow = onClose ? (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            aria-label="More actions"
            data-testid="terminal-overflow"
            size="icon-sm"
            type="button"
            variant="ghost"
          />
        }
      >
        <Ellipsis aria-hidden="true" className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem
          data-testid="terminal-close"
          disabled={closePending}
          onClick={onClose}
          variant="destructive"
        >
          Close terminal
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  ) : null;
  if (!takeControl && !releaseControl && !quietAction && !overflow) return null;
  return (
    <>
      <TerminalHeaderRule />
      {takeControl}
      {releaseControl}
      {quietAction}
      {overflow}
    </>
  );
}

/**
 * Window-level verbs: another terminal, and the journal. They belong to the
 * window rather than to this terminal's lease, so they trail everything else.
 */
function TerminalWindowVerbs({
  onNewTerminal,
  onViewJournal,
}: Pick<TerminalHeaderProps, "onNewTerminal" | "onViewJournal">) {
  if (!onNewTerminal && !onViewJournal) return null;
  return (
    <>
      <TerminalHeaderRule />
      {onNewTerminal ? (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                aria-label="New terminal"
                data-testid="terminal-new"
                onClick={onNewTerminal}
                size="icon-sm"
                type="button"
                variant="ghost"
              />
            }
          >
            <Plus aria-hidden="true" className="size-3.5" />
          </TooltipTrigger>
          <TooltipContent side="bottom">New terminal</TooltipContent>
        </Tooltip>
      ) : null}
      {onViewJournal ? (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                aria-label="Journal"
                data-testid="terminal-journal-toggle"
                onClick={onViewJournal}
                size="icon-sm"
                type="button"
                variant="ghost"
              />
            }
          >
            <ScrollText aria-hidden="true" className="size-3.5" />
          </TooltipTrigger>
          <TooltipContent side="bottom">Journal</TooltipContent>
        </Tooltip>
      ) : null}
    </>
  );
}

function TerminalPipeHeaderActions({
  onClose,
  onSignal,
  onWait,
  closePending,
}: Pick<TerminalHeaderActionsProps, "onClose" | "onSignal" | "onWait" | "closePending">) {
  const wait = onWait ? (
    <Button data-testid="terminal-wait" onClick={onWait} size="sm" type="button" variant="ghost">
      Wait
    </Button>
  ) : null;
  const close = onClose ? (
    <Button
      data-testid="terminal-close"
      disabled={closePending}
      onClick={onClose}
      size="sm"
      type="button"
      variant="ghost"
    >
      Close
    </Button>
  ) : null;
  const overflow = onSignal ? (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            aria-label="More actions"
            data-testid="terminal-pipe-overflow"
            size="icon-sm"
            type="button"
            variant="ghost"
          />
        }
      >
        <Ellipsis aria-hidden="true" className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem data-testid="terminal-signal" onClick={onSignal}>
          Signal
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  ) : null;
  if (!wait && !close && !overflow) return null;
  return (
    <>
      <TerminalHeaderRule />
      {wait}
      {close}
      {overflow}
    </>
  );
}

function TerminalHeaderRule() {
  return <Separator className="h-3.5 self-center" orientation="vertical" />;
}

type TerminalHeaderActionsProps = Pick<
  TerminalHeaderProps,
  | "lease"
  | "recording"
  | "closePending"
  | "onTakeControl"
  | "onReleaseControl"
  | "onStop"
  | "onClose"
  | "onSignal"
  | "onWait"
  | "onStopRecording"
> & { isPipe: boolean };

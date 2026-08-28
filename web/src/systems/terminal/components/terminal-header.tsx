import {
  CircleStop,
  Disc,
  FileText,
  ScrollText,
  TerminalSquare,
  type LucideIcon,
} from "lucide-react";

import {
  Button,
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
  onTakeControl?: () => void;
  onReleaseControl?: () => void;
  onStop?: () => void;
  onClose?: () => void;
  onSignal?: () => void;
  onStopRecording?: () => void;
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
}: {
  hostChrome: boolean;
  projectLabel?: string;
}) {
  useTopbarSlot(
    hostChrome
      ? {
          glyph: <TerminalIdentityGlyph icon={ScrollText} />,
          crumb: "Journal",
          status: projectLabel ? (
            <span className="truncate text-badge text-subtle">{projectLabel}</span>
          ) : undefined,
        }
      : null
  );
  return null;
}

/** The framed window glyph — the same recipe the OS window head uses. */
export function TerminalIdentityGlyph({ icon: Icon }: { icon: LucideIcon }) {
  return (
    <span
      aria-hidden="true"
      className="inline-flex size-topbar-glyph shrink-0 items-center justify-center rounded border border-line bg-badge-fill text-muted"
    >
      <Icon className="size-3.5" />
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
  onTakeControl,
  onReleaseControl,
  onStop,
  onClose,
  onSignal,
  onStopRecording,
  hostChrome = false,
}: TerminalHeaderProps) {
  const isPipe = terminal.mode === "pipe";
  const actions = (
    <TerminalHeaderActions
      isPipe={isPipe}
      lease={lease}
      onClose={onClose}
      onReleaseControl={onReleaseControl}
      onSignal={onSignal}
      onStop={onStop}
      onStopRecording={onStopRecording}
      recording={recording}
      onTakeControl={onTakeControl}
    />
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
          glyph: <TerminalIdentityGlyph icon={isPipe ? FileText : TerminalSquare} />,
          crumb: terminal.title,
          count: <MonoId size="sm" value={terminal.id} />,
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
        <TerminalIdentityGlyph icon={isPipe ? FileText : TerminalSquare} />
        <span className="truncate font-semibold text-fg-strong text-ws-name tracking-tight">
          {terminal.title}
        </span>
        <MonoId size="sm" value={terminal.id} />
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
 * greyed-out Take control would still claim the feature exists here.
 */
function TerminalHeaderActions({
  isPipe,
  lease,
  recording,
  onTakeControl,
  onReleaseControl,
  onStop,
  onClose,
  onSignal,
  onStopRecording,
}: TerminalHeaderActionsProps) {
  if (isPipe) {
    const signal = onSignal ? (
      <Button
        data-testid="terminal-signal"
        onClick={onSignal}
        size="sm"
        type="button"
        variant="ghost"
      >
        Signal
      </Button>
    ) : null;
    const close = onClose ? (
      <Button
        data-testid="terminal-close"
        onClick={onClose}
        size="sm"
        type="button"
        variant="ghost"
      >
        Close
      </Button>
    ) : null;
    if (!signal && !close) return null;
    return (
      <>
        <TerminalHeaderRule />
        {signal}
        {close}
      </>
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
  // place: a watcher can still take control of a terminal that is recording,
  // and whoever holds it can still give it back. Stopping the recording is the
  // quieter of the two and stands beside it as an icon.
  const quietAction =
    recording && onStopRecording ? (
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              aria-label="Stop recording"
              data-testid="terminal-stop-recording"
              onClick={onStopRecording}
              size="icon-sm"
              type="button"
              variant="ghost"
            />
          }
        >
          <Disc aria-hidden="true" className="size-3.5 text-danger" />
        </TooltipTrigger>
        <TooltipContent side="bottom">Stop recording</TooltipContent>
      </Tooltip>
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
  if (!takeControl && !releaseControl && !quietAction) return null;
  return (
    <>
      <TerminalHeaderRule />
      {takeControl}
      {releaseControl}
      {quietAction}
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
  | "onTakeControl"
  | "onReleaseControl"
  | "onStop"
  | "onClose"
  | "onSignal"
  | "onStopRecording"
> & { isPipe: boolean };

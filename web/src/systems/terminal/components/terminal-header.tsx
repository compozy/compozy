import { CircleStop, Disc, FileText, TerminalSquare } from "lucide-react";

import { Button, MonoId, Pill } from "@compozy/ui";

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
  /**
   * The grid the daemon settled on, once it has said so.
   *
   * Several viewers can watch one terminal and the smallest controlling one
   * decides its size, so the number on screen is frequently not the one this
   * window would have chosen. Stating it is how that stops being a mystery.
   */
  grid?: { cols: number; rows: number } | null;
  recording?: TerminalRecordingState | null;
  onTakeControl?: () => void;
  onReleaseControl?: () => void;
  onStop?: () => void;
  onClose?: () => void;
  onStopRecording?: () => void;
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
  grid,
  recording,
  onTakeControl,
  onReleaseControl,
  onStop,
  onClose,
  onStopRecording,
}: TerminalHeaderProps) {
  const isPipe = terminal.mode === "pipe";
  const Glyph = isPipe ? FileText : TerminalSquare;
  return (
    <header
      className="flex min-h-11 flex-none items-center gap-2 border-line border-b px-3.5"
      data-testid="terminal-header"
    >
      <span className="flex min-w-0 items-center gap-2">
        <Glyph aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
        <span className="truncate font-medium text-item-title text-fg-strong">
          {terminal.title}
        </span>
        <MonoId size="sm" value={terminal.id} />
      </span>
      <span aria-hidden="true" className="min-w-2 flex-1" />
      <div className="flex flex-none items-center gap-2">
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
        {grid ? (
          <Pill data-testid="terminal-grid-chip" mono size="sm" tone="neutral">
            {grid.cols}×{grid.rows}
          </Pill>
        ) : null}
        <TerminalLeaseBadge view={lease} viewers={isPipe ? undefined : terminal.viewers} />
        <TerminalHeaderActions
          isPipe={isPipe}
          lease={lease}
          onClose={onClose}
          onReleaseControl={onReleaseControl}
          onStop={onStop}
          onStopRecording={onStopRecording}
          recording={recording}
          onTakeControl={onTakeControl}
        />
      </div>
    </header>
  );
}

/**
 * At most two trailing actions.
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
  onStopRecording,
}: {
  isPipe: boolean;
  lease: TerminalLeaseView;
  recording?: TerminalRecordingState | null;
  onTakeControl?: () => void;
  onReleaseControl?: () => void;
  onStop?: () => void;
  onClose?: () => void;
  onStopRecording?: () => void;
}) {
  if (isPipe) {
    return onClose ? (
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
  }
  // Recording does not touch the lease, so it never takes the lease action's
  // place: a watcher can still take control of a terminal that is recording,
  // and whoever holds it can still give it back. Stopping the recording is the
  // quieter of the two and stands beside it as an icon.
  return (
    <>
      {lease.canTakeControl && onTakeControl ? (
        <Button data-testid="terminal-take-control" onClick={onTakeControl} size="sm" type="button">
          Take control
        </Button>
      ) : null}
      {lease.canRelease && onReleaseControl ? (
        <Button
          data-testid="terminal-release-control"
          onClick={onReleaseControl}
          size="sm"
          type="button"
          variant="ghost"
        >
          Release control
        </Button>
      ) : null}
      {recording && onStopRecording ? (
        <Button
          aria-label="Stop recording"
          data-testid="terminal-stop-recording"
          onClick={onStopRecording}
          size="icon-sm"
          title="Stop recording"
          type="button"
          variant="ghost"
        >
          <Disc aria-hidden="true" className="size-3.5 text-danger" />
        </Button>
      ) : null}
      {onStop && !lease.canTakeControl ? (
        <Button
          aria-label="Stop"
          data-testid="terminal-stop"
          onClick={onStop}
          size="icon-sm"
          title="Stop"
          type="button"
          variant="ghost"
        >
          <CircleStop aria-hidden="true" className="size-3.5" />
        </Button>
      ) : null}
    </>
  );
}

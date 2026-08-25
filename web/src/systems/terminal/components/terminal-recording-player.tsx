"use client";

import { destroyTerminalInstance, TerminalView, type TerminalEngineLoader } from "@compozy/ui";
import { Pause, Play } from "lucide-react";
import { useEffect } from "react";

import { Button, MonoId } from "@compozy/ui";

import { useTerminalRecordingPlayback } from "../hooks/use-terminal-recording-playback";
import { formatPlaybackClock } from "../lib/asciicast";

export interface TerminalRecordingPlayerProps {
  /** The raw asciicast v2 artifact. */
  source: string;
  recordingId: string;
  /** The command this recorded, when the caller knows it. */
  title?: string;
  /** When it was recorded, already phrased. */
  recordedAtLabel?: string;
  /** How long it is kept, from `[terminal].recording_retention_days`. */
  retentionNote?: string;
  /** Jumps to the journal row this recording belongs to. */
  onOpenJournal?: () => void;
  /** Timer seam, so playback timing can be driven deterministically. */
  schedule?: (run: () => void, delayMs: number) => () => void;
  /** Replaces the emulator. Tests and playback harnesses only. */
  engineLoader?: TerminalEngineLoader;
  autoPlay?: boolean;
}

/**
 * A recording, played back.
 *
 * This is the live screen replayed, not a transcript: the same emulator paints
 * it, and the recorded timing is honoured. Secrets were filtered before the
 * artifact was written, so what plays back is exactly what people saw.
 */
export function TerminalRecordingPlayer({
  source,
  recordingId,
  title,
  recordedAtLabel,
  retentionNote,
  onOpenJournal,
  schedule,
  engineLoader,
  autoPlay = false,
}: TerminalRecordingPlayerProps) {
  const playback = useTerminalRecordingPlayback({ source, schedule, autoPlay });
  const instanceId = `recording ${recordingId}`;
  // A replay is rebuilt from the artifact every time it is opened, so its
  // emulator has nothing to preserve and is disposed when the player closes.
  useEffect(() => () => destroyTerminalInstance(instanceId), [instanceId]);

  if (playback.error) {
    return (
      <div className="p-3.5 text-form-input text-muted" data-testid="terminal-recording-error">
        {playback.error}
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid="terminal-recording-player">
      {title ? (
        <div className="flex min-h-9 min-w-0 flex-none items-center gap-2 border-line border-b bg-canvas-soft px-3">
          <Play aria-hidden="true" className="size-3 flex-none text-subtle" />
          <span className="truncate font-semibold text-form-input text-fg-strong">{title}</span>
          <MonoId size="sm" value={recordingId} />
          <div className="ml-auto flex flex-none items-center gap-2.5">
            {recordedAtLabel || retentionNote ? (
              <span className="font-mono text-micro text-faint">
                {recordedAtLabel}
                {recordedAtLabel && retentionNote ? " · " : ""}
                {retentionNote}
              </span>
            ) : null}
            {onOpenJournal ? (
              <Button
                data-testid="terminal-recording-open-journal"
                onClick={onOpenJournal}
                size="xs"
                type="button"
                variant="ghost"
              >
                Open in journal
              </Button>
            ) : null}
          </div>
        </div>
      ) : null}
      <div className="flex min-h-0 flex-1 flex-col bg-terminal-bg">
        <TerminalView
          aria-label="Recording replay"
          className="px-3.5 pt-2.5 pb-3 font-mono text-[12.5px] leading-[1.5] tracking-[0.02em]"
          {...(engineLoader ? { engineLoader } : {})}
          handleRef={playback.handleRef}
          instanceId={instanceId}
          readOnly
          screenReaderMode
        />
      </div>
      <div className="flex min-h-11.5 flex-none items-center gap-3 border-line border-t bg-canvas px-3">
        <Button
          aria-label={playback.playing ? "Pause" : "Play"}
          data-testid="terminal-recording-toggle"
          onClick={playback.toggle}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          {playback.playing ? (
            <Pause aria-hidden="true" className="size-3.5" />
          ) : (
            <Play aria-hidden="true" className="size-3.5" />
          )}
        </Button>
        <input
          aria-label="Playback position"
          aria-valuetext={`${formatPlaybackClock(playback.positionMs)} of ${formatPlaybackClock(playback.durationMs)}`}
          className="h-5.5 min-w-0 flex-1 accent-fg-strong"
          data-testid="terminal-recording-scrubber"
          max={playback.durationMs}
          min={0}
          onChange={event => playback.seek(Number(event.target.value))}
          step={100}
          type="range"
          value={playback.positionMs}
        />
        <span
          className="font-mono text-badge tabular-nums whitespace-nowrap text-subtle"
          data-testid="terminal-recording-clock"
        >
          {formatPlaybackClock(playback.positionMs)} / {formatPlaybackClock(playback.durationMs)}
        </span>
      </div>
    </div>
  );
}

"use client";

import { TerminalWriteAbandonedError, type TerminalViewHandle } from "@compozy/ui";
import { useEffect, useRef, useState } from "react";

import { AsciicastPlayback, parseAsciicast, type Asciicast } from "../lib/asciicast";

export interface UseTerminalRecordingPlaybackOptions {
  /** The raw asciicast v2 artifact. */
  source: string;
  /** Timer seam, so playback timing can be driven deterministically. */
  schedule?: (run: () => void, delayMs: number) => () => void;
  autoPlay?: boolean;
}

export interface TerminalRecordingPlaybackView {
  handleRef: React.RefObject<TerminalViewHandle | null>;
  cast: Asciicast | null;
  error: string | null;
  positionMs: number;
  durationMs: number;
  playing: boolean;
  toggle: () => void;
  seek: (positionMs: number) => void;
}

/**
 * Drives one recording against one emulator.
 *
 * The playback object owns the timing; this only bridges it to React state and
 * keeps the two in step, so the component stays a transport bar over a screen.
 */
export function useTerminalRecordingPlayback({
  source,
  schedule,
  autoPlay = false,
}: UseTerminalRecordingPlaybackOptions): TerminalRecordingPlaybackView {
  const handleRef = useRef<TerminalViewHandle>(null);
  const playbackRef = useRef<AsciicastPlayback | null>(null);
  const [state, setState] = useState<{ cast: Asciicast | null; error: string | null }>(() =>
    readRecording(source)
  );
  const [positionMs, setPositionMs] = useState(0);
  // Autoplay is the initial state, not something an effect turns on afterwards:
  // deriving it here keeps the first paint honest and avoids a second render.
  const [playing, setPlaying] = useState(autoPlay);
  const [parsedSource, setParsedSource] = useState(source);

  // Parsing is a pure read of a prop, so it happens during render rather than in
  // an effect that would paint an empty screen first and then correct it.
  if (parsedSource !== source) {
    setParsedSource(source);
    setState(readRecording(source));
    setPositionMs(0);
    setPlaying(autoPlay);
  }

  const cast = state.cast;

  useEffect(() => {
    if (!cast) return undefined;
    // The recording was made at a specific grid, and every wrap in it assumes
    // that grid. Applying it before the first byte is written is the difference
    // between a replay and an approximation of one — the pre-attach queue keeps
    // the order even when the emulator has not loaded yet.
    handleRef.current?.applyDimensions({ cols: cast.header.width, rows: cast.header.height });
    const playback = new AsciicastPlayback({
      cast,
      sink: {
        // Playback does not wait on the parse, so the rejection a closing view
        // raises has to be caught here — an abandoned write is the player being
        // closed, not a failure anyone needs to hear about.
        write: data => void handleRef.current?.write(data).catch(ignoreAbandonedWrite),
        reset: () => handleRef.current?.reset(),
      },
      onProgress: setPositionMs,
      onEnded: () => setPlaying(false),
      ...(schedule ? { schedule } : {}),
    });
    playbackRef.current = playback;
    if (autoPlay && !playback.play()) setPlaying(false);
    return () => {
      playback.dispose();
      playbackRef.current = null;
    };
  }, [autoPlay, cast, schedule]);

  return {
    handleRef,
    cast,
    error: state.error,
    positionMs,
    durationMs: cast?.durationMs ?? 0,
    playing,
    toggle: () => {
      const playback = playbackRef.current;
      if (!playback) return;
      if (playback.isPlaying) {
        playback.pause();
        setPlaying(false);
        return;
      }
      setPlaying(playback.play());
    },
    seek: next => playbackRef.current?.seek(next),
  };
}

/** Only the view going away is silent; anything else is a real failure. */
function ignoreAbandonedWrite(cause: unknown): void {
  if (cause instanceof TerminalWriteAbandonedError) return;
  throw cause;
}

function readRecording(source: string): { cast: Asciicast | null; error: string | null } {
  try {
    return { cast: parseAsciicast(source), error: null };
  } catch (cause) {
    return {
      cast: null,
      error: cause instanceof Error ? cause.message : "The recording could not be read.",
    };
  }
}

/**
 * asciicast v2 parsing and playback.
 *
 * A recording is a header line followed by one JSON array per event. Replay
 * honours the recorded timestamps rather than dumping the file: the point of a
 * recording is what the screen looked like *over time*, and a flat dump answers
 * a different question.
 */

import { renderTerminalRedactedInput } from "./terminal-wire";

export interface AsciicastHeader {
  version: number;
  width: number;
  height: number;
  title?: string;
}

interface AsciicastFrameBase {
  /** Offset from the start of the capture, in milliseconds. */
  atMs: number;
}

export interface AsciicastOutputFrame extends AsciicastFrameBase {
  kind: "output";
  data: string;
}

export interface AsciicastRedactedInputFrame extends AsciicastFrameBase {
  kind: "redacted_input";
  characters: number;
}

export type AsciicastFrame = AsciicastOutputFrame | AsciicastRedactedInputFrame;

export interface Asciicast {
  header: AsciicastHeader;
  frames: AsciicastFrame[];
  /** Length of the capture, in milliseconds. */
  durationMs: number;
}

export class AsciicastParseError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AsciicastParseError";
  }
}

/**
 * Reads one artifact.
 *
 * Output and daemon-authored redacted-input markers are replayed. Raw input
 * events, where a recorder wrote them, are not the screen and are dropped.
 */
export function parseAsciicast(source: string): Asciicast {
  const lines = source.split("\n").filter(line => line.trim() !== "");
  if (lines.length === 0) {
    throw new AsciicastParseError("The recording is empty.");
  }
  const header = parseHeader(lines[0]);
  const frames: AsciicastFrame[] = [];
  for (const line of lines.slice(1)) {
    const frame = parseFrame(line);
    if (!frame) continue;
    // Time only moves forward in a recording. A frame that goes backwards would
    // make `seek` replay a different screen than playback drew, so the artifact
    // is unreadable rather than silently reordered into something plausible.
    const previous = frames.at(-1);
    if (previous && frame.atMs < previous.atMs) {
      throw new AsciicastParseError("The recording's timestamps go backwards.");
    }
    frames.push(frame);
  }
  return { header, frames, durationMs: frames.at(-1)?.atMs ?? 0 };
}

/** A terminal dimension: a whole number of cells, at least one. */
function readDimension(value: unknown, fallback: number, field: string): number {
  if (value === undefined || value === null) return fallback;
  if (typeof value !== "number" || !Number.isInteger(value) || value <= 0) {
    throw new AsciicastParseError(`The recording's ${field} is not a usable size.`);
  }
  return value;
}

function parseHeader(line: string): AsciicastHeader {
  let parsed: unknown;
  try {
    parsed = JSON.parse(line);
  } catch {
    throw new AsciicastParseError("The recording header is not readable.");
  }
  if (typeof parsed !== "object" || parsed === null) {
    throw new AsciicastParseError("The recording header is not readable.");
  }
  const header = parsed as Record<string, unknown>;
  if (header.version !== 2) {
    throw new AsciicastParseError(`Unsupported recording format: version ${header.version}.`);
  }
  if (header.title !== undefined && typeof header.title !== "string") {
    throw new AsciicastParseError("The recording's title is not readable.");
  }
  return {
    version: 2,
    width: readDimension(header.width, 80, "width"),
    height: readDimension(header.height, 24, "height"),
    ...(header.title === undefined ? {} : { title: header.title }),
  };
}

function parseFrame(line: string): AsciicastFrame | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(line);
  } catch {
    throw new AsciicastParseError("A recording event is not readable.");
  }
  if (!Array.isArray(parsed) || parsed.length < 3) {
    throw new AsciicastParseError("A recording event is not readable.");
  }
  const [at, kind, data] = parsed as [unknown, unknown, unknown];
  if (kind !== "o" && kind !== "m") return null;
  if (typeof at !== "number" || !Number.isFinite(at) || at < 0) {
    throw new AsciicastParseError("A recording event has an invalid timestamp.");
  }
  const atMs = Math.round(at * 1000);
  if (kind === "o") {
    if (typeof data !== "string") {
      throw new AsciicastParseError("A recording output event is not readable.");
    }
    return { atMs, kind: "output", data };
  }
  if (
    typeof data !== "object" ||
    data === null ||
    (data as Record<string, unknown>).kind !== "redacted_input" ||
    typeof (data as Record<string, unknown>).characters !== "number" ||
    !Number.isSafeInteger((data as Record<string, unknown>).characters) ||
    ((data as Record<string, unknown>).characters as number) < 0
  ) {
    throw new AsciicastParseError("A recording redacted-input event is not readable.");
  }
  return {
    atMs,
    kind: "redacted_input",
    characters: (data as Record<string, unknown>).characters as number,
  };
}

function renderAsciicastFrame(frame: AsciicastFrame): string {
  return frame.kind === "output" ? frame.data : renderTerminalRedactedInput(frame.characters);
}

export interface AsciicastPlaybackSink {
  write(data: string): void;
  reset(): void;
}

export interface AsciicastPlaybackOptions {
  cast: Asciicast;
  sink: AsciicastPlaybackSink;
  onProgress(positionMs: number): void;
  onEnded?(): void;
  /** Timer seam, so playback timing can be driven deterministically. */
  schedule?: (run: () => void, delayMs: number) => () => void;
}

/**
 * Walks a recording in recorded time.
 *
 * Position is the authority: seeking replays from the start up to the target so
 * the screen is what it actually was at that moment, rather than whatever the
 * emulator happened to hold.
 */
export class AsciicastPlayback {
  private index = 0;
  private positionMs = 0;
  private cancel: (() => void) | null = null;
  private playing = false;

  constructor(private readonly options: AsciicastPlaybackOptions) {}

  get isPlaying(): boolean {
    return this.playing;
  }

  play(): boolean {
    if (this.playing) return true;
    if (this.index >= this.options.cast.frames.length) {
      this.options.onEnded?.();
      return false;
    }
    this.playing = true;
    this.scheduleNext();
    return true;
  }

  pause(): void {
    this.playing = false;
    this.cancel?.();
    this.cancel = null;
  }

  /** Rebuilds the screen at `positionMs` from the start of the capture. */
  seek(positionMs: number): void {
    const wasPlaying = this.playing;
    this.pause();
    this.options.sink.reset();
    this.index = 0;
    this.positionMs = Math.max(0, Math.min(positionMs, this.options.cast.durationMs));
    while (
      this.index < this.options.cast.frames.length &&
      this.options.cast.frames[this.index].atMs <= this.positionMs
    ) {
      this.options.sink.write(renderAsciicastFrame(this.options.cast.frames[this.index]));
      this.index += 1;
    }
    this.options.onProgress(this.positionMs);
    if (wasPlaying) this.play();
  }

  dispose(): void {
    this.pause();
  }

  private scheduleNext(): void {
    const frame = this.options.cast.frames[this.index];
    if (!frame) {
      this.playing = false;
      this.options.onEnded?.();
      return;
    }
    const schedule = this.options.schedule ?? defaultSchedule;
    const delay = Math.max(0, frame.atMs - this.positionMs);
    this.cancel = schedule(() => {
      this.cancel = null;
      if (!this.playing) return;
      this.options.sink.write(renderAsciicastFrame(frame));
      this.index += 1;
      this.positionMs = frame.atMs;
      this.options.onProgress(this.positionMs);
      this.scheduleNext();
    }, delay);
  }
}

function defaultSchedule(run: () => void, delayMs: number): () => void {
  const timer = setTimeout(run, delayMs);
  return () => clearTimeout(timer);
}

/**
 * `0:51 / 2:29` — the transport bar's only numbers.
 *
 * Seconds are floored, the way every media transport reads them: a position
 * that rounded up would show a second that has not elapsed yet.
 */
export function formatPlaybackClock(positionMs: number): string {
  const totalSeconds = Math.max(0, Math.floor(positionMs / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

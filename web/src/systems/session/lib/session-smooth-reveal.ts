// Smooth streaming reveal (ADR-008), presentation only. The transport still
// coalesces chunks (~100ms); the reveal drains each burst over a short window at
// a velocity that tracks arrival, capped so a pasted code block stays bounded,
// and snaps to the full text the moment the stream settles. Reduced motion and
// the client-local "Smooth streaming" setting render chunks directly. Expensive
// consumers (syntax highlighting) read a separately throttled value so a growing
// code block never re-tokenizes at the reveal cadence. Policy ported from the
// synara reference, never the library internals.

/** Drain the current backlog over this window; above the transport flush so the reveal never runs dry. */
export const REVEAL_DRAIN_WINDOW_SECONDS = 0.16;
/** Hard ceiling so one huge flush is fast but bounded. */
export const REVEAL_MAX_CHARS_PER_SECOND = 2_000;
/** Low-pass factor on the velocity (≈110ms time constant at 60fps). */
export const REVEAL_VELOCITY_LERP = 0.15;
/** Per-frame `dt` clamp so a backgrounded tab returning does not dump the backlog in one frame. */
export const REVEAL_MAX_FRAME_SECONDS = 0.05;
/** React commit quantization (≈25 commits/s); the reveal float still advances every frame. */
export const REVEAL_MIN_EMIT_INTERVAL_MS = 40;
/** Code blocks re-highlight on this slower cadence. */
export const HIGHLIGHT_THROTTLE_MS = 120;

export interface SmoothRevealState {
  /** Revealed character count as a float. */
  shown: number;
  /** Smoothed chars per second. */
  velocity: number;
  /** 0 = start of a fresh burst. */
  lastFrameAt: number;
  /** 0 = force the next emit. */
  lastEmitAt: number;
}

export function createSmoothRevealState(shown: number): SmoothRevealState {
  return { shown, velocity: 0, lastFrameAt: 0, lastEmitAt: 0 };
}

export interface SmoothRevealStep {
  /** Character count to commit, or `null` when this frame commits nothing. */
  emitCount: number | null;
  /** The backlog is drained; the loop may sleep until more text arrives. */
  done: boolean;
}

/**
 * Advance the reveal by one frame. Mutates `state` in place so the rAF loop
 * allocates nothing. The first frame of a burst only seeds the clock (dt = 0);
 * the last characters of a burst are never held back by the emit interval.
 */
export function stepSmoothReveal(
  state: SmoothRevealState,
  nowMs: number,
  targetLength: number,
  emittedCount: number
): SmoothRevealStep {
  const dt =
    state.lastFrameAt > 0
      ? Math.min((nowMs - state.lastFrameAt) / 1000, REVEAL_MAX_FRAME_SECONDS)
      : 0;
  state.lastFrameAt = nowMs;
  state.shown = Math.min(state.shown, targetLength);

  const backlog = targetLength - state.shown;
  if (backlog <= 0) {
    state.velocity = 0;
    state.lastFrameAt = 0;
    return { emitCount: null, done: true };
  }

  const targetVelocity = Math.min(
    REVEAL_MAX_CHARS_PER_SECOND,
    backlog / REVEAL_DRAIN_WINDOW_SECONDS
  );
  state.velocity += (targetVelocity - state.velocity) * REVEAL_VELOCITY_LERP;
  state.shown = Math.min(targetLength, state.shown + state.velocity * dt);

  const nextCount = Math.floor(state.shown);
  const caughtUp = nextCount >= targetLength;
  const emitDue =
    nextCount !== emittedCount &&
    (caughtUp || state.lastEmitAt === 0 || nowMs - state.lastEmitAt >= REVEAL_MIN_EMIT_INTERVAL_MS);
  if (emitDue) state.lastEmitAt = nowMs;

  const done = targetLength - state.shown <= 0;
  if (done) {
    state.velocity = 0;
    state.lastFrameAt = 0;
  }
  return { emitCount: emitDue ? nextCount : null, done };
}

/** Whether the new text only appended to the previous one; anything else snaps. */
export function isAppendOnlyUpdate(previous: string, next: string): boolean {
  return next.length >= previous.length && next.startsWith(previous);
}

export type ThrottledCommitPlan = { immediate: true } | { immediate: false; delayMs: number };

/** When the highlighter may next commit: first change passes, the trailing edge is always delivered. */
export function planThrottledCommit(
  lastCommitAtMs: number,
  nowMs: number,
  intervalMs: number
): ThrottledCommitPlan {
  const elapsed = lastCommitAtMs === 0 ? Number.POSITIVE_INFINITY : nowMs - lastCommitAtMs;
  return elapsed >= intervalMs
    ? { immediate: true }
    : { immediate: false, delayMs: Math.max(0, intervalMs - elapsed) };
}

const FENCE_PATTERN = /^\s{0,3}(```|~~~)/;

/** Offset of the opening fence of an unclosed code block at the end of `text`, or `-1`. */
export function openCodeFenceStart(text: string): number {
  let offset = 0;
  let openFence: { marker: string; start: number } | null = null;
  for (const line of text.split("\n")) {
    const match = FENCE_PATTERN.exec(line);
    if (match) {
      const marker = match[1]!;
      if (openFence === null) {
        openFence = { marker, start: offset };
      } else if (openFence.marker === marker) {
        openFence = null;
      }
    }
    offset += line.length + 1;
  }
  return openFence?.start ?? -1;
}

/**
 * The text handed to the markdown host while streaming: prose follows the
 * smooth reveal, but an unclosed code block reads from the throttled value so
 * the highlighter re-tokenizes on its own slower cadence. Both inputs are
 * prefixes of the same stream, so the fence offset is shared; the composition
 * equals the revealed text once the block closes or the stream settles.
 */
export function composeStreamingDisplay(revealed: string, throttled: string): string {
  const fenceStart = openCodeFenceStart(revealed);
  if (fenceStart < 0) return revealed;
  if (throttled.length <= fenceStart) return revealed.slice(0, fenceStart);
  if (throttled.length >= revealed.length) return revealed;
  return revealed.slice(0, fenceStart) + throttled.slice(fenceStart);
}

/** Client-local preference (S10 "Smooth streaming"); never a daemon key by decision (Q20). */
export const SMOOTH_STREAMING_STORAGE_KEY = "session:smooth-streaming";

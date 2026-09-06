import { describe, expect, it } from "vitest";

import {
  composeStreamingDisplay,
  createSmoothRevealState,
  HIGHLIGHT_THROTTLE_MS,
  isAppendOnlyUpdate,
  openCodeFenceStart,
  planThrottledCommit,
  REVEAL_MAX_CHARS_PER_SECOND,
  REVEAL_MIN_EMIT_INTERVAL_MS,
  stepSmoothReveal,
} from "../session-smooth-reveal";

// Suite: smooth-reveal policy (ADR-008, US-025, UT-099, UT-107).
// Invariant: the reveal tracks arrival (lerped velocity, capped), never delays
// completion (snap when the stream settles or the text is rewritten), and the
// highlighter consumer receives values on its own throttle whose final value
// equals the full text. Reduced motion and the toggle bypass the policy at the
// hook, which returns the text directly.
describe("smooth reveal stepper", () => {
  it("Should seed the clock on the first frame and then drain the backlog at a lerped velocity", () => {
    const state = createSmoothRevealState(0);
    const first = stepSmoothReveal(state, 1_000, 200, 0);
    // dt = 0 on the first frame: nothing revealed yet, but the clock is seeded.
    expect(first).toEqual({ emitCount: null, done: false });
    expect(state.velocity).toBeGreaterThan(0);

    const second = stepSmoothReveal(state, 1_016, 200, 0);
    expect(second.emitCount).not.toBeNull();
    expect(second.emitCount!).toBeGreaterThan(0);
    expect(second.emitCount!).toBeLessThan(200);
    expect(second.done).toBe(false);
  });

  it("Should cap the velocity so one huge flush stays bounded", () => {
    const state = createSmoothRevealState(0);
    stepSmoothReveal(state, 0, 1_000_000, 0);
    for (let frame = 1; frame <= 60; frame += 1) {
      stepSmoothReveal(state, frame * 16, 1_000_000, 0);
    }
    expect(state.velocity).toBeLessThanOrEqual(REVEAL_MAX_CHARS_PER_SECOND);
    // ~1s of frames at the cap reveals about 2000 chars, never the whole flush.
    expect(state.shown).toBeLessThan(3_000);
  });

  it("Should quantize commits to the emit interval but never hold back the final characters", () => {
    const state = createSmoothRevealState(0);
    stepSmoothReveal(state, 0, 40, 0);
    const commitTimes: number[] = [];
    let emitted = 0;
    let now = 0;
    for (let frame = 1; frame <= 200; frame += 1) {
      now = frame * 8;
      const step = stepSmoothReveal(state, now, 40, emitted);
      if (step.emitCount !== null) {
        commitTimes.push(now);
        emitted = step.emitCount;
      }
      if (step.done) break;
    }
    expect(emitted).toBe(40);
    for (let index = 1; index < commitTimes.length - 1; index += 1) {
      expect(commitTimes[index]! - commitTimes[index - 1]!).toBeGreaterThanOrEqual(
        REVEAL_MIN_EMIT_INTERVAL_MS
      );
    }
  });

  it("Should clamp a long frame gap so a returning tab does not dump the backlog", () => {
    const state = createSmoothRevealState(0);
    stepSmoothReveal(state, 0, 10_000, 0);
    stepSmoothReveal(state, 5_000, 10_000, 0);
    expect(state.shown).toBeLessThan(200);
  });

  it("Should treat only appends as continuations; rewrites and stream end snap", () => {
    expect(isAppendOnlyUpdate("Hello", "Hello, world")).toBe(true);
    expect(isAppendOnlyUpdate("Hello", "Help")).toBe(false);
    expect(isAppendOnlyUpdate("Hello", "Hell")).toBe(false);
  });
});

describe("highlight throttle", () => {
  it("Should let the first change through and deliver the trailing edge on schedule", () => {
    expect(planThrottledCommit(0, 5_000, HIGHLIGHT_THROTTLE_MS)).toEqual({ immediate: true });
    expect(planThrottledCommit(5_000, 5_040, HIGHLIGHT_THROTTLE_MS)).toEqual({
      immediate: false,
      delayMs: HIGHLIGHT_THROTTLE_MS - 40,
    });
    expect(planThrottledCommit(5_000, 5_200, HIGHLIGHT_THROTTLE_MS)).toEqual({ immediate: true });
  });

  it("Should hand an unclosed code block to the throttled value and prose to the reveal", () => {
    const prose = "Some prose\n";
    const fence = "```go\nfunc main() {\n";
    const revealed = `${prose}${fence}\tfmt.Println(1)\n`;
    const throttled = `${prose}${fence}`;
    expect(openCodeFenceStart(revealed)).toBe(prose.length);
    expect(composeStreamingDisplay(revealed, throttled)).toBe(throttled);
    // Prose after the block closes reads from the reveal again.
    const closed = `${revealed}}\n\`\`\`\nDone.`;
    expect(openCodeFenceStart(closed)).toBe(-1);
    expect(composeStreamingDisplay(closed, throttled)).toBe(closed);
    // Without any fence the reveal is the display.
    expect(composeStreamingDisplay("plain", "pl")).toBe("plain");
    // A throttled value that has not reached the fence yet hides the open block until it does.
    expect(composeStreamingDisplay(revealed, "Some")).toBe(prose);
  });

  it("Should equal the full text once the stream settles", () => {
    const full = "Intro\n```ts\nconst a = 1;\n```\nOutro";
    expect(composeStreamingDisplay(full, full)).toBe(full);
  });
});

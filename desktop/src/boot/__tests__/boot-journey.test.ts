import { describe, expect, it } from "vitest";

import type { ShellSnapshot } from "../../state/app-state";
import { BootJourney, LONG_AFTER_MS, STALL_AFTER_MS, formatDuration } from "../page/journey";

const UP = { from_version: "0.5.4", to_version: "0.6.0" } as const;

function migrate(completed: number, total = 2554): ShellSnapshot {
  return { state: "provisioning", stage: "migrate", completed, total, work: "sessions", ...UP };
}

// Invariant: the boot window shows only steps, counts and times observed in the snapshot stream:
// finished steps carry their duration, counted work its real numbers and an estimate once the rate
// is known, and a step that stops advancing surfaces logs and diagnostics without an error.
// Owner: boot page journey model. Canonical suite: boot-journey.test.ts.
describe("boot journey", () => {
  it("Should keep a plain start in the centered layout without steps", () => {
    const journey = new BootJourney(0);
    journey.observe({ state: "resolving" }, 0);
    const view = journey.observe({ state: "starting", stage: "start", attempt: 1 }, 500);
    expect(view).toMatchObject({ layout: "center", steps: [], current: "start", work: null });
  });

  it("Should list a provisioning journey with durations and add data upgrade only when it runs", () => {
    const journey = new BootJourney(0);
    const installing = journey.observe({ state: "provisioning", stage: "install", ...UP }, 0);
    expect(installing.layout).toBe("journey");
    expect(installing.steps.map(step => [step.id, step.status])).toEqual([
      ["verify", "done"],
      ["install", "current"],
      ["start", "pending"],
      ["connect", "pending"],
    ]);
    const upgrading = journey.observe(migrate(0), 6000);
    expect(upgrading.steps.map(step => [step.id, step.status, step.meta])).toEqual([
      ["verify", "done", undefined],
      ["install", "done", "6s"],
      ["migrate", "current", undefined],
      ["start", "pending", undefined],
      ["connect", "pending", undefined],
    ]);
  });

  it("Should report counts, percent and an estimate from the observed rate", () => {
    const journey = new BootJourney(0);
    journey.observe({ state: "provisioning", stage: "install", ...UP }, 0);
    expect(journey.observe(migrate(744), 1000).work).toEqual({
      completed: 744,
      total: 2554,
      unit: "sessions",
      percent: 29,
      minutesLeft: null,
    });
    // 75 sessions a minute: 1,810 left is about 24 minutes.
    const later = journey.observe(migrate(769), 21_000).work;
    expect(later?.percent).toBe(30);
    expect(later?.minutesLeft).toBeCloseTo(23.8, 1);
  });

  it("Should explain a long upgrade, then warn with diagnostics when it stops advancing", () => {
    const journey = new BootJourney(0);
    journey.observe(migrate(100), 0);
    expect(journey.view(LONG_AFTER_MS - 1).notice).toBeNull();
    journey.observe(migrate(900), LONG_AFTER_MS);
    expect(journey.view(LONG_AFTER_MS).notice).toMatchObject({
      tone: "info",
      text: "2,554 sessions take a while. CompozyOS is still working.",
      diagnostics: false,
    });
    const stalled = journey.view(LONG_AFTER_MS + STALL_AFTER_MS);
    expect(stalled).toMatchObject({
      stalled: true,
      notice: { tone: "warning", title: "No progress for 2 minutes", diagnostics: true },
    });
    expect(stalled.work?.minutesLeft).toBeNull();
    expect(journey.observe(migrate(901), LONG_AFTER_MS + STALL_AFTER_MS + 1).stalled).toBe(false);
  });

  it("Should warn about an uncounted step that runs too long", () => {
    const journey = new BootJourney(0);
    journey.observe({ state: "provisioning", stage: "install" }, 0);
    expect(journey.view(STALL_AFTER_MS + 60_000).notice?.title).toBe(
      "Still on this step after 3 minutes"
    );
  });

  it("Should mark the failed step with the attempt it reached", () => {
    const journey = new BootJourney(0);
    journey.observe({ state: "provisioning", stage: "install" }, 0);
    journey.observe({ state: "starting", stage: "start", attempt: 5 }, 1000);
    const failed = journey.observe(
      {
        state: "error",
        attempt: 5,
        error: { code: "runtime_start_failed", safe_message: "x", log_path: "x" },
      },
      2000
    );
    expect(failed.layout).toBe("journey");
    expect(failed.steps.find(step => step.id === "start")).toEqual({
      id: "start",
      label: "Start",
      status: "failed",
      meta: "5 of 5",
    });
    expect(failed.notice).toBeNull();
  });

  it("Should follow an app-driven update through its download", () => {
    const journey = new BootJourney(0);
    const view = journey.observe({ state: "updating", stage: "download", target: "runtime" }, 0);
    expect(view.steps.map(step => step.id)).toEqual([
      "download",
      "verify",
      "install",
      "start",
      "connect",
    ]);
    expect(view.steps[0]?.status).toBe("current");
  });

  it.each([
    [400, "<1s"],
    [18_000, "18s"],
    [247_000, "4m 07s"],
    [3_720_000, "1h 02m"],
  ])("Should format %i ms as %s", (milliseconds, text) => {
    expect(formatDuration(milliseconds)).toBe(text);
  });
});

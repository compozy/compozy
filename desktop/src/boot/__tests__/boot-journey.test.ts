import { describe, expect, it } from "vitest";

import type { ShellSnapshot } from "../../state/app-state";
import { bootstrapSnapshot } from "../bootstrap-state";
import { detailFor, statusWordFor } from "../page/copy";
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
    const starting = bootstrapSnapshot({
      type: "bootstrap",
      phase: "start",
      status: "started",
      message: "Starting",
      attempt: 5,
    });
    journey.observe(starting, 1000);
    // The real failure event carries no attempt; the projection keeps the one the start reached.
    const failure = bootstrapSnapshot(
      { type: "bootstrap", phase: "start", status: "failed", message: "x" },
      "x",
      starting
    );
    const failed = journey.observe(failure, 2000);
    expect(failed.layout).toBe("journey");
    expect(failed.steps.find(step => step.id === "start")).toEqual({
      id: "start",
      label: "Start",
      status: "failed",
      meta: "5 of 5",
    });
    expect(failed.notice).toBeNull();
  });

  it("Should start a new journey when a failed boot is retried", () => {
    const journey = new BootJourney(0);
    journey.observe({ state: "provisioning", stage: "install" }, 0);
    journey.observe({ state: "starting", stage: "start", attempt: 5 }, 1000);
    journey.observe(
      {
        state: "error",
        error: { code: "runtime_start_failed", safe_message: "x", log_path: "x" },
      },
      2000
    );
    const retry = journey.observe(
      { state: "starting", stage: "start", attempt: 1 },
      2000 + STALL_AFTER_MS
    );
    expect(retry).toMatchObject({ layout: "center", steps: [], elapsed: 0, stalled: false });
    expect(retry.notice).toBeNull();
  });

  it("Should move a stalled plain start to the journey layout with its notice", () => {
    const journey = new BootJourney(0);
    journey.observe({ state: "starting", stage: "start", attempt: 1 }, 0);
    expect(journey.view(STALL_AFTER_MS)).toMatchObject({
      layout: "journey",
      stalled: true,
      notice: { tone: "warning", diagnostics: true },
    });
  });

  it("Should show a percentage-only stage and treat a new percentage as progress", () => {
    const journey = new BootJourney(0);
    const pct = (value: number): ShellSnapshot => ({
      state: "provisioning",
      stage: "migrate",
      pct: value,
    });
    expect(journey.observe(pct(42), 0).work).toMatchObject({ total: 0, percent: 42 });
    expect(journey.observe(pct(43), STALL_AFTER_MS - 1).work?.percent).toBe(43);
    expect(journey.view(STALL_AFTER_MS + 1).stalled).toBe(false);
  });

  it("Should restart the estimate when a second migration counts from zero", () => {
    const journey = new BootJourney(0);
    journey.observe(migrate(0, 100), 0);
    journey.observe(migrate(100, 100), 600_000);
    journey.observe(migrate(0, 100), 600_001);
    // 50 records in 20 seconds: the other 50 take about 20 seconds more, not the earlier run's pace.
    expect(journey.observe(migrate(50, 100), 620_001).work?.minutesLeft).toBeCloseTo(1 / 3, 2);
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

// Invariant: the boot window says what the runtime is doing in plain words: a named step when it
// knows one, the phase otherwise, and a wait as a wait. Runtime step identifiers are never shown.
// Owner: boot page copy. Canonical suite: boot-journey.test.ts.
describe("boot page lifecycle copy", () => {
  function shown(snapshot: ShellSnapshot) {
    const view = new BootJourney(0).observe(snapshot, 0);
    return { detail: detailFor(snapshot, view), word: statusWordFor(snapshot, view) };
  }

  it("Should name the runtime step it knows and fall back to the phase for the rest", () => {
    const starting = { state: "starting", stage: "start", attempt: 1 } as const;
    const named = shown({
      ...starting,
      boot: { readiness: "serving", step: "resumeBootInterruptedSessions" },
    });
    expect(named).toEqual({ detail: "Resuming interrupted sessions.", word: "Working" });
    const unnamed = shown({ ...starting, boot: { readiness: "serving", step: "bootNewStep" } });
    expect(unnamed.detail).toBe("Resuming your sessions.");
    const phaseOnly = shown({ ...starting, boot: { readiness: "starting" } });
    expect(phaseOnly.detail).toBe("Starting the runtime with your data.");
    const unknown = shown({ ...starting, boot: { readiness: "warming", step: "bootNewStep" } });
    expect(unknown).toEqual({ detail: null, word: "Working" });
  });

  it("Should say another update or restart is running, with its process when known", () => {
    const starting = { state: "starting", stage: "start", attempt: 1 } as const;
    const owned = shown({ ...starting, boot: { wait: "runtime_mutation", owner_pid: 4242 } });
    expect(owned.word).toBe("Waiting");
    expect(owned.detail).toBe(
      "Another update or restart is in progress (process 4242). CompozyOS continues when it finishes."
    );
    const unowned = shown({ ...starting, boot: { wait: "runtime_mutation" } });
    expect(unowned.detail).toBe(
      "Another update or restart is in progress. CompozyOS continues when it finishes."
    );
  });
});

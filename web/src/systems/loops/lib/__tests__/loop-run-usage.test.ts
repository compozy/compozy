import { describe, expect, it } from "vitest";

import { loopRunDetailByRunId } from "../../mocks/fixtures";
import type { LoopRunRecord } from "../../types";
import {
  buildRunUsage,
  deriveCostEstimate,
  formatClockDuration,
  runElapsedSeconds,
  terminalRunElapsedSeconds,
  usageNote,
  usageSnapshotFacts,
} from "../loop-run-usage";

function run(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return { ...loopRunDetailByRunId.get("looprun_running")!.run, ...overrides };
}

describe("runElapsedSeconds", () => {
  it("Should tick against started_at only while running and freeze the active span otherwise", () => {
    const base = run({
      created_at: "2026-07-22T14:00:00Z",
      started_at: "2026-07-22T14:10:00Z",
      last_progress_at: "2026-07-22T14:18:00Z",
    });
    const now = Date.parse("2026-07-22T14:32:14Z");
    expect(runElapsedSeconds({ ...base, status: "running" }, now)).toBe(22 * 60 + 14);
    expect(runElapsedSeconds({ ...base, status: "watching" }, now)).toBe(18 * 60);
    expect(runElapsedSeconds({ ...base, status: "failed" }, now)).toBe(18 * 60);
  });
});

describe("terminalRunElapsedSeconds", () => {
  const base = run({
    created_at: "2026-07-22T14:00:00Z",
    started_at: "2026-07-22T14:10:00Z",
    last_progress_at: "2026-07-22T14:18:00Z",
  });
  const now = Date.parse("2026-07-22T14:32:14Z");

  it("Should measure a terminal run from the terminal frame at, not the stale last_progress span", () => {
    // Durable span (created → last_progress) is 18m; the terminal frame lands at 31m.
    expect(
      terminalRunElapsedSeconds({ ...base, status: "exhausted" }, "2026-07-22T14:31:00Z", now)
    ).toBe(31 * 60);
  });

  it("Should fall back to the durable span when the terminal frame is absent or malformed", () => {
    expect(terminalRunElapsedSeconds({ ...base, status: "exhausted" }, undefined, now)).toBe(
      18 * 60
    );
    expect(terminalRunElapsedSeconds({ ...base, status: "exhausted" }, "not-a-date", now)).toBe(
      18 * 60
    );
    // A terminal instant before creation cannot be the real end; keep the durable span.
    expect(
      terminalRunElapsedSeconds({ ...base, status: "exhausted" }, "2026-07-22T13:00:00Z", now)
    ).toBe(18 * 60);
  });

  it("Should ignore the terminal frame for a running run and keep ticking against started_at", () => {
    expect(
      terminalRunElapsedSeconds({ ...base, status: "running" }, "2026-07-22T14:31:00Z", now)
    ).toBe(22 * 60 + 14);
  });
});

describe("formatClockDuration", () => {
  it("Should speak the prototype clock voice", () => {
    expect(formatClockDuration(22 * 60 + 14)).toBe("22m 14s");
    expect(formatClockDuration(45 * 60)).toBe("45m 00s");
    expect(formatClockDuration(3_600 + 4 * 60)).toBe("1h 04m");
    expect(formatClockDuration(42)).toBe("0m 42s");
  });
});

describe("buildRunUsage", () => {
  it("Should render capped rows with ceilings and unbounded rows as ∞", () => {
    const rows = buildRunUsage(
      run({
        tokens_used: 268_000,
        budget_tokens: 1_500_000,
        budget_wall_sec: 45 * 60,
        generation: 2,
        iteration_cap: 0,
      }),
      22 * 60 + 14
    );
    expect(rows.map(row => [row.key, row.value, row.max])).toEqual([
      ["time", "22m 14s", "/ 45m"],
      ["tokens", "268K", "/ 1.5M"],
      ["cost", "~$1.34", "estimate"],
      ["rounds", "2", "/ ∞"],
    ]);
    expect(rows.every(row => row.tone === "neutral")).toBe(true);
  });

  it("Should warn near a ceiling and go danger at it (never on cost)", () => {
    const rows = buildRunUsage(
      run({
        tokens_used: 1_400_000,
        budget_tokens: 1_500_000,
        budget_wall_sec: 45 * 60,
        generation: 3,
        iteration_cap: 3,
      }),
      45 * 60
    );
    const byKey = new Map(rows.map(row => [row.key, row]));
    expect(byKey.get("time")?.tone).toBe("danger");
    expect(byKey.get("tokens")?.tone).toBe("warn");
    expect(byKey.get("rounds")?.tone).toBe("danger");
    expect(byKey.get("cost")?.tone).toBe("neutral");
  });
});

describe("deriveCostEstimate", () => {
  it("Should stay a tilde-prefixed estimate derived from the blended rate", () => {
    expect(deriveCostEstimate(268_000)).toBe("~$1.34");
    expect(deriveCostEstimate(0)).toBe("~$0.00");
  });
});

describe("usageNote", () => {
  it("Should pick the state variant, the policy sentence, and silence on terminal runs", () => {
    expect(usageNote(run({ status: "watching" }))).toContain("Watching costs nothing");
    expect(usageNote(run({ status: "paused" }))).toContain("Paused runs use nothing");
    expect(usageNote(run({ status: "running", budget_on_exceeded: "escalate" }))).toContain(
      "pauses and asks you"
    );
    expect(usageNote(run({ status: "running", budget_on_exceeded: "halt" }))).toContain(
      "stops as exhausted"
    );
    expect(usageNote(run({ status: "failed" }))).toBeNull();
  });
});

describe("usageSnapshotFacts", () => {
  it("Should stand in for missing approval facts with the usage snapshot", () => {
    const facts = usageSnapshotFacts(
      run({
        tokens_used: 412_000,
        budget_tokens: 1_500_000,
        budget_wall_sec: 45 * 60,
        generation: 3,
      }),
      45 * 60
    );
    expect(facts).toEqual([
      { label: "Time used", value: "45m 00s of 45m" },
      { label: "Tokens", value: "412K / 1.5M" },
      { label: "Cost", value: "~$2.06 est." },
      { label: "Round", value: "3" },
    ]);
  });
});

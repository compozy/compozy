import type { StateGlyphState } from "@compozy/ui";
import { describe, expect, it } from "vitest";

import {
  isLiveLoopRun,
  isLoopRunStatus,
  isTerminalLoopStatus,
  loopStatusGlyph,
  loopStatusLabel,
} from "../loop-formatters";
import type { LoopRunStatus } from "../../types";

const STATUS_TABLE: Array<{
  status: LoopRunStatus;
  glyph: StateGlyphState;
  terminal: boolean;
  label: string;
}> = [
  { status: "queued", glyph: "queued", terminal: false, label: "Queued" },
  { status: "running", glyph: "running", terminal: false, label: "Running" },
  { status: "watching", glyph: "running", terminal: false, label: "Watching" },
  { status: "needs-approval", glyph: "attention", terminal: false, label: "Needs approval" },
  { status: "paused", glyph: "stopped", terminal: false, label: "Paused" },
  { status: "done", glyph: "done", terminal: true, label: "Done" },
  { status: "no-op", glyph: "idle", terminal: true, label: "Nothing to do" },
  { status: "blocked", glyph: "attention", terminal: true, label: "Blocked" },
  { status: "failed", glyph: "failed", terminal: true, label: "Failed" },
  { status: "exhausted", glyph: "failed", terminal: true, label: "Exhausted" },
  { status: "stalled", glyph: "failed", terminal: true, label: "Stalled" },
  { status: "canceled", glyph: "stopped", terminal: true, label: "Canceled" },
];

describe("loop-formatters", () => {
  it("Should map every one of the 12 statuses to its state glyph, terminal group and label", () => {
    for (const row of STATUS_TABLE) {
      expect(loopStatusGlyph(row.status)).toBe(row.glyph);
      expect(isTerminalLoopStatus(row.status)).toBe(row.terminal);
      expect(loopStatusLabel(row.status)).toBe(row.label);
    }
  });

  it("Should classify only mutable non-terminal runs as live", () => {
    expect(isLiveLoopRun({ historical: false, status: "running" })).toBe(true);
    expect(isLiveLoopRun({ historical: false, status: "queued" })).toBe(true);
    expect(isLiveLoopRun({ historical: true, status: "running" })).toBe(false);
    expect(isLiveLoopRun({ historical: false, status: "done" })).toBe(false);
    expect(isLiveLoopRun(undefined)).toBe(false);
  });

  it("Should treat unknown or missing statuses as idle and non-terminal", () => {
    expect(isLoopRunStatus("mystery")).toBe(false);
    expect(isLoopRunStatus(null)).toBe(false);
    expect(loopStatusGlyph("mystery")).toBe("idle");
    expect(isTerminalLoopStatus("mystery")).toBe(false);
    expect(loopStatusGlyph(undefined)).toBe("idle");
  });

  it("Should label unknown statuses with the raw value and blank statuses as Unknown", () => {
    expect(loopStatusLabel("mystery")).toBe("mystery");
    expect(loopStatusLabel("   ")).toBe("Unknown");
    expect(loopStatusLabel(null)).toBe("Unknown");
  });
});

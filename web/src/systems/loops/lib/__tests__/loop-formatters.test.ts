import type { PillTone } from "@agh/ui";
import { describe, expect, it } from "vitest";

import {
  isLoopRunStatus,
  isTerminalLoopStatus,
  loopStatusLabel,
  loopStatusPulse,
  loopStatusSignal,
  loopStatusTone,
} from "../loop-formatters";
import type { LoopRunStatus } from "../../types";

// The canonical §3.3 table: every one of the 11 statuses -> tone + pulse + label +
// terminal group. Diverging from this is a truthful-UI regression (color = state).
const STATUS_TABLE: Array<{
  status: LoopRunStatus;
  tone: PillTone;
  pulse: boolean;
  terminal: boolean;
  label: string;
}> = [
  { status: "queued", tone: "neutral", pulse: false, terminal: false, label: "Queued" },
  { status: "running", tone: "accent", pulse: true, terminal: false, label: "Running" },
  { status: "watching", tone: "info", pulse: true, terminal: false, label: "Watching" },
  {
    status: "needs-approval",
    tone: "info",
    pulse: false,
    terminal: false,
    label: "Needs Approval",
  },
  { status: "paused", tone: "neutral", pulse: false, terminal: false, label: "Paused" },
  { status: "done", tone: "success", pulse: false, terminal: true, label: "Done" },
  { status: "no-op", tone: "neutral", pulse: false, terminal: true, label: "No-op" },
  { status: "blocked", tone: "warning", pulse: false, terminal: true, label: "Blocked" },
  { status: "failed", tone: "danger", pulse: false, terminal: true, label: "Failed" },
  { status: "exhausted", tone: "warning", pulse: false, terminal: true, label: "Exhausted" },
  { status: "stalled", tone: "neutral", pulse: false, terminal: true, label: "Stalled" },
];

describe("loop-formatters", () => {
  it("Should map every one of the 11 statuses to the design tone, pulse, terminal group and label", () => {
    for (const row of STATUS_TABLE) {
      expect(loopStatusTone(row.status)).toBe(row.tone);
      expect(loopStatusPulse(row.status)).toBe(row.pulse);
      expect(isTerminalLoopStatus(row.status)).toBe(row.terminal);
      expect(loopStatusLabel(row.status)).toBe(row.label);
      expect(loopStatusSignal(row.status)).toEqual({ tone: row.tone, pulse: row.pulse });
    }
  });

  it("Should pulse only the live running/watching states", () => {
    const pulsing = STATUS_TABLE.filter(row => row.pulse).map(row => row.status);
    expect(pulsing).toEqual(["running", "watching"]);
  });

  it("Should recognize the 6 terminal statuses and reject live ones", () => {
    const terminal = STATUS_TABLE.filter(row => row.terminal).map(row => row.status);
    expect(terminal).toHaveLength(6);
    expect(isTerminalLoopStatus("running")).toBe(false);
    expect(isTerminalLoopStatus("queued")).toBe(false);
  });

  it("Should treat unknown or missing statuses as neutral, non-pulsing, non-terminal", () => {
    expect(isLoopRunStatus("mystery")).toBe(false);
    expect(isLoopRunStatus(null)).toBe(false);
    expect(loopStatusTone("mystery")).toBe("neutral");
    expect(loopStatusPulse("mystery")).toBe(false);
    expect(isTerminalLoopStatus("mystery")).toBe(false);
    expect(loopStatusSignal(undefined)).toEqual({ tone: "neutral", pulse: false });
  });

  it("Should label unknown statuses with the raw value and blank statuses as Unknown", () => {
    expect(loopStatusLabel("mystery")).toBe("mystery");
    expect(loopStatusLabel("   ")).toBe("Unknown");
    expect(loopStatusLabel(null)).toBe("Unknown");
  });
});

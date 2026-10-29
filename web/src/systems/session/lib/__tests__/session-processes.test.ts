import { describe, expect, it } from "vitest";

import type { SessionProcessResult } from "../../adapters/session-processes-api";
import { primarySessionFixture } from "../../mocks/fixtures";
import { withSessionProcessSummary } from "../../mocks/process-fixtures";
import { processStopReport, sessionRunningProcesses } from "../session-processes";

// Suite: session process view model.
// Invariant: the count is background + Terminal rows of a supported scan, and a stop reads as
// one report from the daemon's per-target outcomes: exited counts as stopped, every other
// refusal is named with its reason.
// Owning layer: session lib (pure). Canonical suite: this file.

const result = (pid: number, outcome: SessionProcessResult["outcome"], error?: string) => ({
  pid,
  started_at: "2026-10-29T12:00:00Z",
  outcome,
  ...(error ? { error } : {}),
});

const command = (entry: SessionProcessResult) => `cmd-${entry.pid}`;

describe("sessionRunningProcesses", () => {
  it("Should count background and Terminal processes of a supported scan only", () => {
    const session = withSessionProcessSummary(primarySessionFixture);
    expect(sessionRunningProcesses(session)).toBe(3);
    expect(
      sessionRunningProcesses({
        process_summary: { ...session.process_summary!, supported: false },
      })
    ).toBe(0);
    expect(sessionRunningProcesses({ process_summary: null })).toBe(0);
  });
});

describe("processStopReport", () => {
  it("Should report every stop as one success, counting already exited as stopped", () => {
    expect(
      processStopReport([result(1, "terminated"), result(2, "already_exited")], command)
    ).toEqual({ tone: "success", title: "2 processes stopped" });
    expect(processStopReport([result(1, "already_exited")], command)).toEqual({
      tone: "info",
      title: "Process already exited",
    });
  });

  it("Should name each refused process with its reason on a partial stop", () => {
    expect(
      processStopReport(
        [
          result(1, "terminated"),
          result(2, "identity_mismatch"),
          result(3, "failed", "operation not permitted"),
          result(4, "unproven"),
        ],
        command
      )
    ).toEqual({
      tone: "error",
      title: "1 process stopped, 3 not stopped",
      description:
        "cmd-2: It changed since the list was read. Refresh and try again. · cmd-3: operation not permitted · cmd-4: Stop was sent, but its exit couldn't be confirmed.",
    });
    expect(processStopReport([result(5, "failed")], command)).toEqual({
      tone: "error",
      title: "Couldn't stop 1 process",
      description: "cmd-5: Couldn't stop it.",
    });
  });
});

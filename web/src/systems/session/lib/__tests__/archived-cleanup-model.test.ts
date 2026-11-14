// Invariant (#1717): the cleanup dialog asks the daemon for exactly what it shows. Its checks and
// range become one dry-run/start body (Everything always carries its locked parts; "7+ days"
// is a day count, "Before a date" an instant, "Any time" no cutoff), and the profile's run
// decides the dialog's phase: a running run shows progress, a run that kept sessions lists them.
// Confirming deletes only what the dry run counted: a relative range is pinned to the dry run's
// cutoff, and a finished run's outcome belongs to the scope it cleaned.
// Owning layer: the pure cleanup model behind the dialog and its container.
import { describe, expect, it } from "vitest";

import type { SessionHistoryCleanupRun } from "../../adapters/session-history-cleanup-api";
import {
  DEFAULT_CLEANUP_OPTIONS,
  cleanupPhase,
  cleanupRequest,
  confirmedRequest,
  runCleanupOptions,
  runInScope,
} from "../archived-cleanup-model";

const DAY_MS = 24 * 60 * 60 * 1000;

function cleanupRun(overrides: Partial<SessionHistoryCleanupRun> = {}): SessionHistoryCleanupRun {
  return {
    id: "run-1",
    profile_id: "prof-1",
    workspace_id: "ws-1",
    delete: { everything: true, children: true, subagents: true },
    state: "running",
    total: 158,
    done: 64,
    deleted: 60,
    skipped: 4,
    failed_threads: 0,
    failures: [],
    started_at: "2026-11-14T12:00:00Z",
    ...overrides,
  };
}

describe("cleanupRequest", () => {
  it("Should send the default as Everything older than 7 days in the list's workspace", () => {
    expect(cleanupRequest(DEFAULT_CLEANUP_OPTIONS, { workspaceId: "ws-1" })).toEqual({
      workspace_id: "ws-1",
      delete: { everything: true, children: true, subagents: true },
      older_than_days: 7,
    });
  });

  it("Should send a chosen day as an instant and Any time as no cutoff, across projects", () => {
    const before = new Date("2026-11-01T00:00:00Z");
    expect(
      cleanupRequest(
        { what: { everything: false, children: false, subagents: true }, range: "before", before },
        { allWorkspaces: true }
      )
    ).toEqual({
      all_workspaces: true,
      delete: { everything: false, children: false, subagents: true },
      archived_before: before.toISOString(),
    });
    expect(
      cleanupRequest({ ...DEFAULT_CLEANUP_OPTIONS, range: "all" }, { workspaceId: "@user" })
    ).toEqual({
      workspace_id: "@user",
      delete: { everything: true, children: true, subagents: true },
    });
  });
});

describe("confirmedRequest", () => {
  it("Should pin a relative range to the dry run's cutoff and leave other ranges as chosen", () => {
    const cutoff = "2026-11-07T12:00:00Z";
    expect(confirmedRequest(DEFAULT_CLEANUP_OPTIONS, { workspaceId: "ws-1" }, cutoff)).toEqual({
      workspace_id: "ws-1",
      delete: { everything: true, children: true, subagents: true },
      archived_before: cutoff,
    });
    const anyTime = { ...DEFAULT_CLEANUP_OPTIONS, range: "all" as const };
    expect(confirmedRequest(anyTime, { workspaceId: "ws-1" }, cutoff)).toEqual(
      cleanupRequest(anyTime, { workspaceId: "ws-1" })
    );
  });
});

describe("cleanup run", () => {
  it("Should belong only to the scope it cleaned", () => {
    expect(runInScope(cleanupRun(), { workspaceId: "ws-1" })).toBe(true);
    expect(runInScope(cleanupRun(), { workspaceId: "ws-2" })).toBe(false);
    expect(runInScope(cleanupRun(), { allWorkspaces: true })).toBe(false);
    const across = cleanupRun({ all_workspaces: true, workspace_id: "" });
    expect(runInScope(across, { allWorkspaces: true })).toBe(true);
    expect(runInScope(across, { workspaceId: "ws-1" })).toBe(false);
  });

  it("Should show a running run's progress and its own range", () => {
    const run = cleanupRun({
      archived_before: new Date(Date.parse("2026-11-14T12:00:00Z") - 15 * DAY_MS).toISOString(),
    });
    expect(cleanupPhase(run)).toEqual({ kind: "deleting", done: 64, total: 158 });
    expect(runCleanupOptions(run)).toEqual({
      what: { everything: true, children: true, subagents: true },
      range: "15",
      before: null,
    });
  });

  it("Should list the threads a run kept, and return to the form after a clean one", () => {
    const failure = {
      session_id: "sess-a",
      workspace_id: "ws-1",
      title: "",
      code: "delete_failed",
      message: "database is locked",
    };
    const kept = {
      kind: "partial",
      total: 158,
      deleted: 150,
      failedThreads: 1,
      failed: [{ sessionId: "sess-a", title: "sess-a", reason: "database is locked" }],
    };
    expect(
      cleanupPhase(
        cleanupRun({ state: "partial", deleted: 150, failed_threads: 1, failures: [failure] })
      )
    ).toEqual(kept);
    // A run stopped after some threads failed still offers them for retry.
    expect(
      cleanupPhase(
        cleanupRun({ state: "stopped", deleted: 150, failed_threads: 1, failures: [failure] })
      )
    ).toEqual(kept);
    expect(cleanupPhase(cleanupRun({ state: "stopped" }))).toEqual({ kind: "form" });
    expect(cleanupPhase(cleanupRun({ state: "completed" }))).toEqual({ kind: "form" });
    expect(cleanupPhase(null)).toEqual({ kind: "form" });
  });
});

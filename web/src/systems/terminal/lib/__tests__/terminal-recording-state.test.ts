// Suite: terminal recording presence (unit)
// Invariant: started adds a terminal-targeted entry, stopped removes it,
// snapshot/open clear only that stream's profile, the next started event
// rehydrates that entry, late/foreign frames are ignored, and elapsed is
// derived from `at`.
// Boundary IN: parse/reconcile/format in terminal-recording-state.
// Boundary OUT: catalog stream subscription and host timer, owned by
// use-terminal-catalog-stream / use-terminal-recordings.

import { describe, expect, it } from "vitest";

import {
  applyRecordingStopSuccess,
  applyTerminalRecordingEvent,
  clearTerminalRecordingsForProfile,
  dropTerminalRecording,
  formatRecordingElapsed,
  parseTerminalRecordingEvent,
  type TerminalRecordingMap,
} from "../terminal-recording-state";

const WORKSPACE = "ws-atlas";
const AT = "2026-08-25T12:00:00.000Z";
const STARTED = {
  event: "terminal.recording_started",
  timestamp: AT,
  workspace_id: WORKSPACE,
  profile_id: "01JB4Z2K9QW8XR3TFN6VYD5HAC",
  terminal_id: "term-4f21c9a03b7e",
  actor_kind: "human",
  actor_id: "pedro",
  at: AT,
  recording_id: "rec-1",
};

function started(overrides: Record<string, unknown> = {}) {
  return parseTerminalRecordingEvent("terminal.recording_started", {
    ...STARTED,
    ...overrides,
  });
}

describe("parseTerminalRecordingEvent", () => {
  it("Should read generated fields and strip extras from a hook envelope", () => {
    expect(parseTerminalRecordingEvent("terminal.recording_started", STARTED)).toEqual({
      name: "terminal.recording_started",
      terminalId: "term-4f21c9a03b7e",
      recordingId: "rec-1",
      at: AT,
      workspaceId: WORKSPACE,
    });
  });

  it("Should accept a thin catalog frame that only has targeting fields", () => {
    expect(
      parseTerminalRecordingEvent("terminal.recording_stopped", {
        terminal_id: "term-4f21c9a03b7e",
        recording_id: "rec-1",
        at: AT,
      })
    ).toEqual({
      name: "terminal.recording_stopped",
      terminalId: "term-4f21c9a03b7e",
      recordingId: "rec-1",
      at: AT,
    });
  });

  it.each([
    ["a missing terminal_id", { recording_id: "rec-1", at: AT }],
    ["a missing recording_id", { terminal_id: "term-4f21c9a03b7e", at: AT }],
    ["an unparseable at", { terminal_id: "term-4f21c9a03b7e", recording_id: "rec-1", at: "soon" }],
  ])("Should ignore %s rather than merge it", (_case, payload) => {
    expect(parseTerminalRecordingEvent("terminal.recording_started", payload)).toBeNull();
  });

  it("Should ignore an event name that is not a recording frame", () => {
    expect(parseTerminalRecordingEvent("terminal.created", STARTED)).toBeNull();
  });
});

describe("applyTerminalRecordingEvent", () => {
  const work = { workspaceId: WORKSPACE, streamProfile: "work", aggregate: false };

  it("Should add a started recording and drop it on stop", () => {
    const event = started();
    if (!event) throw new Error("expected a started event");
    const live = applyTerminalRecordingEvent({}, event, work);
    expect(live["term-4f21c9a03b7e"]).toEqual({
      recordingId: "rec-1",
      at: AT,
      profileKey: "work",
    });

    const stopped = parseTerminalRecordingEvent("terminal.recording_stopped", STARTED);
    if (!stopped) throw new Error("expected a stopped event");
    expect(applyTerminalRecordingEvent(live, stopped, work)).toEqual({});
  });

  it("Should ignore a frame addressed to another workspace", () => {
    const event = started({ workspace_id: "ws-other" });
    if (!event) throw new Error("expected a started event");
    expect(applyTerminalRecordingEvent({}, event, work)).toEqual({});
  });

  it("Should rehydrate a started recording after a matching profile clear", () => {
    const event = started();
    if (!event) throw new Error("expected a started event");
    const live = applyTerminalRecordingEvent({}, event, work);
    const cleared = clearTerminalRecordingsForProfile(live, "work", false);
    expect(cleared).toEqual({});

    const restored = applyTerminalRecordingEvent(cleared, event, work);
    expect(restored["term-4f21c9a03b7e"]).toEqual({
      recordingId: "rec-1",
      at: AT,
      profileKey: "work",
    });

    const foreign = started({ workspace_id: "ws-other", recording_id: "rec-foreign" });
    if (!foreign) throw new Error("expected a foreign event");
    expect(applyTerminalRecordingEvent(restored, foreign, work)).toEqual(restored);
  });
});

describe("clearTerminalRecordingsForProfile", () => {
  const map: TerminalRecordingMap = {
    "term-work": { recordingId: "rec-w", at: AT, profileKey: "work" },
    "term-personal": { recordingId: "rec-p", at: AT, profileKey: "personal" },
  };

  it("Should clear the whole single-profile cache", () => {
    expect(clearTerminalRecordingsForProfile(map, "work", false)).toEqual({});
  });

  it("Should drop only the snapshotted owner's rows in an aggregate cache", () => {
    expect(clearTerminalRecordingsForProfile(map, "work", true)).toEqual({
      "term-personal": map["term-personal"],
    });
  });
});

describe("applyRecordingStopSuccess", () => {
  const live: TerminalRecordingMap = {
    "term-4f21c9a03b7e": { recordingId: "rec-1", at: AT, profileKey: "work" },
    "term-other": { recordingId: "rec-2", at: AT, profileKey: "work" },
  };

  it("Should drop only the stopped terminal when the mutation saved it", () => {
    expect(
      applyRecordingStopSuccess(live, {
        terminal_id: "term-4f21c9a03b7e",
        state: "saved",
      })
    ).toEqual({
      "term-other": live["term-other"],
    });
  });

  it("Should leave the map when the mutation still reports recording", () => {
    expect(
      applyRecordingStopSuccess(live, {
        terminal_id: "term-4f21c9a03b7e",
        state: "recording",
      })
    ).toEqual(live);
  });
});

describe("dropTerminalRecording", () => {
  it("Should return the same map when the terminal is not recording", () => {
    const current = { "term-a": { recordingId: "rec-1", at: AT, profileKey: "work" } };
    expect(dropTerminalRecording(current, "term-missing")).toBe(current);
  });
});

describe("formatRecordingElapsed", () => {
  it("Should format elapsed time as mm:ss from at", () => {
    const start = Date.parse(AT);
    expect(formatRecordingElapsed(AT, start + 134_000)).toBe("02:14");
    expect(formatRecordingElapsed(AT, start)).toBe("00:00");
  });

  it("Should skip an unparseable at rather than invent a clock", () => {
    expect(formatRecordingElapsed("not-a-date", Date.parse(AT))).toBeNull();
  });
});

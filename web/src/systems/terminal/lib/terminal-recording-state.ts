/**
 * Live recording presence for the Terminal host.
 *
 * The catalog snapshot has no recording fields. Presence is event-driven:
 * `terminal.recording_started` / `terminal.recording_stopped` write a
 * workspace-and-profile-scoped map. A matching snapshot or reconnect open
 * drops that stream's entries so a missed stop cannot linger; the daemon then
 * emits `terminal.recording_started` for each still-active recording in this
 * workspace and profile. Elapsed time is derived from the event's `at`, never
 * stored. There is no REST reread, second cache, or guessed fallback.
 */

import { z } from "zod";

export const TERMINAL_RECORDING_EVENTS = [
  "terminal.recording_started",
  "terminal.recording_stopped",
] as const;

export type TerminalRecordingEventName = (typeof TERMINAL_RECORDING_EVENTS)[number];

const TERMINAL_RECORDING_EVENT_NAMES: ReadonlySet<string> = new Set(TERMINAL_RECORDING_EVENTS);

export interface TerminalRecordingEntry {
  recordingId: string;
  /** ISO instant from the generated payload field `at`. */
  at: string;
  /** The catalog stream that wrote this entry, for aggregate isolation. */
  profileKey: string;
}

export type TerminalRecordingMap = Record<string, TerminalRecordingEntry>;

export interface TerminalRecordingEvent {
  name: TerminalRecordingEventName;
  terminalId: string;
  recordingId: string;
  at: string;
  workspaceId?: string;
}

export interface TerminalRecordingStreamContext {
  workspaceId: string;
  streamProfile: string;
  aggregate: boolean;
}

const recordingEventFieldsSchema = z.object({
  terminal_id: z.string().min(1),
  recording_id: z.string().min(1),
  at: z.iso.datetime({ offset: true }),
  workspace_id: z.string().min(1).optional(),
});

export function isTerminalRecordingEventName(name: string): name is TerminalRecordingEventName {
  return TERMINAL_RECORDING_EVENT_NAMES.has(name);
}

/**
 * Reads generated recording fields and strips everything else.
 *
 * Hook envelopes carry extra keys the catalog frame does not. Missing
 * `terminal_id` cannot target a pane, so the frame is ignored rather than
 * merged. There is no REST reread for in-progress recordings.
 */
export function parseTerminalRecordingEvent(
  name: string,
  raw: unknown
): TerminalRecordingEvent | null {
  if (!isTerminalRecordingEventName(name)) return null;
  const parsed = recordingEventFieldsSchema.safeParse(raw);
  if (!parsed.success) return null;
  const event: TerminalRecordingEvent = {
    name,
    terminalId: parsed.data.terminal_id,
    recordingId: parsed.data.recording_id,
    at: parsed.data.at,
  };
  if (parsed.data.workspace_id !== undefined) {
    event.workspaceId = parsed.data.workspace_id;
  }
  return event;
}

export function applyTerminalRecordingEvent(
  current: TerminalRecordingMap,
  event: TerminalRecordingEvent,
  context: TerminalRecordingStreamContext
): TerminalRecordingMap {
  if (event.workspaceId !== undefined && event.workspaceId !== context.workspaceId) {
    return current;
  }
  if (event.name === "terminal.recording_stopped") {
    return dropTerminalRecording(current, event.terminalId);
  }
  return {
    ...current,
    [event.terminalId]: {
      recordingId: event.recordingId,
      at: event.at,
      profileKey: context.streamProfile,
    },
  };
}

/** Drops one stream's entries. A single-profile cache is the whole map. */
export function clearTerminalRecordingsForProfile(
  current: TerminalRecordingMap,
  streamProfile: string,
  aggregate: boolean
): TerminalRecordingMap {
  if (!aggregate) return {};
  const next: TerminalRecordingMap = {};
  for (const [terminalId, entry] of Object.entries(current)) {
    if (entry.profileKey !== streamProfile) next[terminalId] = entry;
  }
  return next;
}

export function dropTerminalRecording(
  current: TerminalRecordingMap,
  terminalId: string
): TerminalRecordingMap {
  if (current[terminalId] === undefined) return current;
  const next = { ...current };
  delete next[terminalId];
  return next;
}

/** Stop POST truth: a saved recording is no longer live. Failures never call this. */
export function applyRecordingStopSuccess(
  current: TerminalRecordingMap,
  recording: { terminal_id: string; state: "recording" | "saved" }
): TerminalRecordingMap {
  if (recording.state === "recording") return current;
  return dropTerminalRecording(current, recording.terminal_id);
}

/** Formats elapsed capture time as `mm:ss` from the event's `at`. */
export function formatRecordingElapsed(at: string, nowMs: number): string | null {
  const start = Date.parse(at);
  if (Number.isNaN(start)) return null;
  const elapsed = Math.max(0, Math.floor((nowMs - start) / 1000));
  const minutes = Math.floor(elapsed / 60);
  const seconds = elapsed % 60;
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

// The daemon's answers over the sessions-stability fixtures (task_10 matrix):
// transcript, search, and outline routes computed from the same transcript the
// thread renders, the transport snapshots the connection states inject, and
// the story loader that clears this session's persisted composer draft.
// Nothing here re-implements a production derivation — the search and outline
// helpers stand in for the daemon's routes over the fixture.

import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { SESSION_TRANSPORT_LIVE } from "@/systems/session/lib/session-transport";
import type { SessionTransportState } from "@/systems/session/lib/session-transcript-thread-context-value";
import { sessionStore } from "@/systems/session/stores/session-store";
import type {
  SessionTranscriptOutlineResponse,
  SessionTranscriptSearchMatch,
  SessionTranscriptSearchResponse,
  TranscriptMessage,
} from "@/systems/session/types";

import { STABILITY_SESSION } from "./sessions-stability-story-fixtures";

type TranscriptPart = NonNullable<TranscriptMessage["parts"]>[number];

const TRANSCRIPT_ROUTE = "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript";
const SEARCH_ROUTE = "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript/search";
const OUTLINE_ROUTE = "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript/outline";

/**
 * Composer drafts persist per session across stories in one browser; every
 * sessions-stability scene starts with this session's composer empty, the way
 * its contract shows it. Story loader — clears only this session's draft.
 */
export function discardStabilityDraft(): Record<string, never> {
  sessionStore.trigger.composerDraftDiscarded({ sessionId: STABILITY_SESSION.id });
  return {};
}

// --- The daemon's answers over the fixture --------------------------------------

function readString(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === "string" ? value : undefined;
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
}

function searchableFields(
  rawPart: TranscriptPart
): { field: string; text: string; role: string }[] {
  const source = asRecord(rawPart);
  const type = readString(source, "type") ?? "";
  if (type === "text") return [{ field: "text", text: readString(source, "text") ?? "", role: "" }];
  if (!type.startsWith("tool-")) return [];
  const inputs = Object.entries(asRecord(source.input))
    .filter((entry): entry is [string, string] => typeof entry[1] === "string")
    .map(([, text]) => ({ field: "input", text, role: "tool" }));
  const title = readString(source, "title");
  return [
    ...inputs,
    ...(title ? [{ field: "title", text: title, role: "tool" }] : []),
    { field: "tool_name", text: type.slice("tool-".length), role: "tool" },
  ];
}

function snippetAround(text: string, at: number, length: number): string {
  const start = Math.max(0, at - 40);
  const end = Math.min(text.length, at + length + 60);
  return `${start > 0 ? "…" : ""}${text.slice(start, end)}${end < text.length ? "…" : ""}`;
}

function messageTurnId(message: TranscriptMessage): string {
  return (
    readString(asRecord(message.metadata), "turn_id") ??
    readString(asRecord(message.parts?.[0]), "turnId") ??
    ""
  );
}

/**
 * The search route's answer as the daemon defines it (`transcript.SearchMatch`):
 * literal, case-insensitive, one match per materialized message, naming the
 * first matching part and field, in transcript order. `sequence` follows
 * `transcriptPayload` (1-based entry index).
 */
export function searchTranscript(
  messages: TranscriptMessage[],
  query: string
): SessionTranscriptSearchResponse {
  const needle = query.trim().toLowerCase();
  if (needle.length === 0) return { matches: [], truncated: false };
  const matches: SessionTranscriptSearchMatch[] = [];
  messages.forEach((message, index) => {
    const turnId = messageTurnId(message);
    const parts = message.parts ?? [];
    for (let partIndex = 0; partIndex < parts.length; partIndex += 1) {
      const hit = searchableFields(parts[partIndex]!)
        .map(candidate => ({ ...candidate, at: candidate.text.toLowerCase().indexOf(needle) }))
        .find(candidate => candidate.at >= 0);
      if (!hit) continue;
      matches.push({
        field: hit.field,
        part_index: partIndex,
        role: hit.role || message.role,
        sequence: index + 1,
        snippet: snippetAround(hit.text, hit.at, needle.length),
        turn_id: turnId,
      });
      break;
    }
  });
  return { matches, truncated: false };
}

/** The outline route's answer: one entry per message the operator sent, with the turn's final reply. */
export function outlineTranscript(messages: TranscriptMessage[]): SessionTranscriptOutlineResponse {
  const entries: SessionTranscriptOutlineResponse["entries"] = [];
  messages.forEach((message, index) => {
    if (message.role !== "user") return;
    const metadata = asRecord(message.metadata);
    const reply = messages[index + 1];
    const replyText =
      reply?.role === "assistant"
        ? (reply.parts ?? [])
            .map(rawPart => asRecord(rawPart))
            .filter(source => readString(source, "type") === "text")
            .map(source => readString(source, "text") ?? "")
            .at(-1)
        : undefined;
    entries.push({
      at: readString(metadata, "timestamp") ?? new Date(0).toISOString(),
      preview: readString(asRecord(message.parts?.[0]), "text") ?? "",
      reply_preview: replyText ?? "",
      sequence: index + 1,
      turn_id: messageTurnId(message),
    });
  });
  return { entries };
}

export function transcriptPayload(messages: TranscriptMessage[]) {
  return {
    entries: messages.map((message, index) => ({
      message,
      sequence: index + 1,
      start_sequence: index + 1,
    })),
    epoch: 1,
    generation: 1,
    has_older: false,
    limit: 200,
    max_sequence: messages.length,
  };
}

/** Transcript, search, and outline routes over one fixture. */
export function stabilityHandlers(messages: TranscriptMessage[]): HttpHandler[] {
  return [
    compozyApiMock.get(TRANSCRIPT_ROUTE, () => HttpResponse.json(transcriptPayload(messages))),
    compozyApiMock.get(SEARCH_ROUTE, ({ request }) =>
      HttpResponse.json(
        searchTranscript(messages, new URL(request.url).searchParams.get("q") ?? "")
      )
    ),
    compozyApiMock.get(OUTLINE_ROUTE, () => HttpResponse.json(outlineTranscript(messages))),
  ];
}

// --- Transport snapshots (connection VC-01..05) -----------------------------------

/** When the stream was last live: five minutes before the story mounted, so the chip's grace has elapsed. */
export const STREAM_LOST_AT = Date.now() - 5 * 60_000;

export function transportState(overrides: Partial<SessionTransportState>): SessionTransportState {
  return {
    ...SESSION_TRANSPORT_LIVE,
    lastLiveAt: STREAM_LOST_AT,
    retry: () => undefined,
    ...overrides,
  };
}

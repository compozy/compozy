import { HttpResponse, type HttpHandler } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { primarySessionFixture } from "@/systems/session/mocks";
import type {
  SessionInputPayload,
  SessionInputsResponse,
  SessionPayload,
  SessionPromptPayload,
  TranscriptMessage,
} from "@/systems/session/types";

import { liveToolTranscript } from "./session-thread-story-quiet-transcripts";
import { transcriptPayload } from "./session-thread-story-transcripts";

/*
 * Queue strip scenes (S2, task_03 VC-01..06): the real `SessionThread` +
 * `useSessionPageControls` over a small in-memory daemon queue served by MSW,
 * so every verb on the strip (edit / steer / remove / clear / retry) runs the
 * production mutation path and the strip re-renders from what the "daemon"
 * answers. The scene owns the entries, the cap, and how the prompt route
 * behaves (queued, acknowledgment lost, or lost then replayed).
 */

export const QUEUE_STORY_TURN_ID = "turn-story-queue";

/**
 * The primary fixture as a public user session mid-turn: `canPromptSession`
 * needs a public `type`, an active state and no archive; the running badge and
 * the active turn make the composer offer queue / steer / interrupt. The same
 * payload is served on the session detail routes so the runtime provider, the
 * page controls and the composer read one coherent session.
 */
export const queueStorySession: SessionPayload = {
  ...primarySessionFixture,
  type: "user",
  archived_at: null,
  activity: {
    current_tool: "Bash",
    elapsed_ms: 48_000,
    elapsed_seconds: 48,
    idle_seconds: 0,
    iteration_current: 1,
    iteration_max: 1,
    turn_id: QUEUE_STORY_TURN_ID,
    turn_started_at: "2026-09-06T14:05:12Z",
  },
  badge: "running",
  state: "active",
};

export interface QueueStoryEntry {
  id: string;
  text: string;
  status?: SessionInputPayload["status"];
  owner_kind?: string;
  owner_id?: string;
}

export const QUEUE_STORY_ENTRIES: QueueStoryEntry[] = [
  { id: "inp_4d8", text: "Ship it with tests" },
  { id: "inp_4e1", text: "Also update the changelog with the new retry semantics" },
  { id: "inp_4f2", text: "Bump the version and tag the release" },
];

export const QUEUE_STORY_OTHER_ACTOR_ENTRIES: QueueStoryEntry[] = [
  { id: "inp_4d8", text: "Ship it with tests" },
  {
    id: "inp_4e1",
    owner_id: "reviewer",
    owner_kind: "agent",
    text: "Also update the changelog with the new retry semantics",
  },
  {
    id: "inp_4f2",
    owner_id: "release-1.4",
    owner_kind: "agent",
    text: "Run the release checklist",
  },
];

export const QUEUE_STORY_CAP = 10;

export const QUEUE_STORY_FULL_ENTRIES: QueueStoryEntry[] = [
  ...QUEUE_STORY_ENTRIES,
  { id: "inp_501", text: "Regenerate the OpenAPI types" },
  { id: "inp_502", text: "Run make gate" },
  { id: "inp_503", text: "Open a draft PR when the gate is green" },
  { id: "inp_504", text: "Link the PR from the tracking issue" },
  { id: "inp_505", text: "Summarize the retry semantics for the release notes" },
  { id: "inp_506", text: "Ping the reviewer once CI is green" },
  { id: "inp_507", text: "Archive the spike branch" },
];

type TranscriptPart = NonNullable<TranscriptMessage["parts"]>[number];

/** The daemon's `queue_cleared` marker, recorded in the turn that was live when the queue was cleared. */
function clearedMarkerPart(entryId: string, turnId: string, at: string): TranscriptPart {
  return {
    data: {
      marker: {
        evidence: { actor_kind: "user", queue_entry_id: entryId, queue_status: "canceled" },
        kind: "transcript_marker.queue_cleared",
        occurred_at: at,
        summary: "Queued input removed by explicit clear.",
      },
      timestamp: at,
      type: "transcript_marker.created",
    },
    timestamp: at,
    turnId,
    type: "data-compozy-event",
  } as unknown as TranscriptPart;
}

/** The public projection writes standalone marker messages beside operational siblings. */
export const queueStoryClearedTranscript: TranscriptMessage[] = [
  ...liveToolTranscript,
  ...QUEUE_STORY_ENTRIES.flatMap((entry, index) => {
    const at = `2026-09-06T14:06:0${index}Z`;
    const marker = clearedMarkerPart(entry.id, QUEUE_STORY_TURN_ID, at);
    const data = (
      marker as unknown as { data: { marker: unknown; type: string; timestamp: string } }
    ).data;
    return [
      {
        id: `queue-clear:${entry.id}`,
        role: "assistant",
        parts: [{ ...marker, data: { type: data.type, timestamp: at, raw: data.marker } }],
      },
      {
        id: `queue-clear-event:${entry.id}`,
        role: "assistant",
        parts: [
          {
            type: "data-compozy-event",
            data: { type: "session.queue_cleared", raw: { queue_entry_id: entry.id } },
          },
        ],
      },
    ] as TranscriptMessage[];
  }),
];

function inputFromEntry(entry: QueueStoryEntry): SessionInputPayload {
  return {
    delivery: "after_turn",
    enqueued_at: "2026-09-06T14:05:30Z",
    id: entry.id,
    idempotency_key: `idk_${entry.id}`,
    message_id: `msg_${entry.id}`,
    mode: "queue",
    queue_generation: 0,
    session_id: primarySessionFixture.id,
    status: entry.status ?? "queued",
    text: entry.text,
    ...(entry.owner_kind ? { owner_id: entry.owner_id, owner_kind: entry.owner_kind } : {}),
  };
}

function promptPayload(
  identity: { message_id: string; idempotency_key: string },
  status: string,
  extra: Partial<SessionPromptPayload> = {}
): SessionPromptPayload {
  return {
    delivery: "after_turn",
    idempotency_key: identity.idempotency_key,
    message_id: identity.message_id,
    queue_position: 0,
    replayed: false,
    status,
    ...extra,
  };
}

export type QueueStoryPromptBehavior =
  | "queued"
  | "acknowledgment_lost"
  | "lost_then_replayed"
  | "queue_full";

export interface QueueStorySceneOptions {
  entries: QueueStoryEntry[];
  cap?: number;
  transcript?: TranscriptMessage[];
  /** How the prompt route answers a queue/steer/interrupt send from the composer. */
  prompt?: QueueStoryPromptBehavior;
  /** Whether an in-row Save is accepted or refused because the entry started dispatching. */
  replace?: "replaced" | "entry_dispatching";
}

const STREAM_ENCODER = new TextEncoder();

/**
 * The session's live stream as the daemon opens it: one `transcript_snapshot`
 * of the same transcript the REST page served, then the connection stays open.
 * Without it the transport reads "disconnected" after its grace and the
 * composer refuses every busy send before a request leaves — production truth
 * for a dead stream, not the state these scenes show.
 */
function queueStoryStreamResponse(transcript: TranscriptMessage[]): Response {
  const snapshot = {
    ...transcriptPayload(transcript),
    reset: false,
    session_id: queueStorySession.id,
    workspace_id: queueStorySession.workspace_id,
  };
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(
        STREAM_ENCODER.encode(
          [
            ": storybook session stream",
            "",
            "event: transcript_snapshot",
            `data: ${JSON.stringify(snapshot)}`,
            "",
            "",
          ].join("\n")
        )
      );
    },
  });
  return new Response(stream, {
    headers: {
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
      "Content-Type": "text/event-stream",
    },
    status: 200,
  });
}

/**
 * MSW handlers over one mutable queue. Reads and writes share the same list,
 * so the strip always shows the daemon's answer, never a frozen fixture.
 */
export function queueStoryHandlers({
  entries: initial,
  cap = QUEUE_STORY_CAP,
  transcript = liveToolTranscript,
  prompt = "queued",
  replace = "replaced",
}: QueueStorySceneOptions): HttpHandler[] {
  let entries = initial.map(entry => ({ ...entry }));
  const promptAttempts = new Map<string, number>();
  const list = (): SessionInputsResponse => ({
    inputs: entries.map(inputFromEntry),
    queue: { cap, entries: entries.length },
  });

  return [
    compozyApiMock.get("/api/sessions/{session_id}", () =>
      HttpResponse.json({ session: queueStorySession })
    ),
    compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}", () =>
      HttpResponse.json({ session: queueStorySession })
    ),
    compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
      HttpResponse.json(transcriptPayload(transcript))
    ),
    compozyApiMock.get(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/stream",
      ({ response }) => response.untyped(queueStoryStreamResponse(transcript))
    ),
    compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue", () =>
      HttpResponse.json(list())
    ),
    compozyApiMock.put(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue/{queue_entry_id}",
      async ({ params, request }) => {
        const id = String(params.queue_entry_id);
        const entry = entries.find(candidate => candidate.id === id);
        if (!entry) {
          return HttpResponse.json({ error: `Queued input not found: ${id}` }, { status: 404 });
        }
        if (replace === "entry_dispatching") {
          entry.status = "dispatching";
          return HttpResponse.json(
            { code: "entry_dispatching", error: "queued input is already dispatching" },
            { status: 409 }
          );
        }
        const body = (await request.json()) as { text: string };
        entry.text = body.text;
        return HttpResponse.json({ input: inputFromEntry(entry) });
      }
    ),
    compozyApiMock.delete(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue/{queue_entry_id}",
      ({ params }) => {
        const id = String(params.queue_entry_id);
        const entry = entries.find(candidate => candidate.id === id);
        if (!entry) {
          return HttpResponse.json({ error: `Queued input not found: ${id}` }, { status: 404 });
        }
        entries = entries.filter(candidate => candidate.id !== id);
        return HttpResponse.json({
          prompt: promptPayload(
            { idempotency_key: `idk_${id}`, message_id: `msg_${id}` },
            "canceled",
            { queue_entry_id: id }
          ),
        });
      }
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue/{queue_entry_id}/steer",
      async ({ params, request, response }) => {
        const id = String(params.queue_entry_id);
        const body = (await request.json()) as { message_id: string; idempotency_key: string };
        entries = entries.filter(candidate => candidate.id !== id);
        return response.untyped(
          HttpResponse.json({
            prompt: promptPayload(body, "steering", {
              disposition: "steering",
              queue_entry_id: id,
              steer_delivery: "injected",
              turn_id: QUEUE_STORY_TURN_ID,
            }),
          })
        );
      }
    ),
    compozyApiMock.delete(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue",
      () => {
        const removed = entries.filter(entry => (entry.status ?? "queued") === "queued");
        entries = entries.filter(entry => (entry.status ?? "queued") !== "queued");
        return HttpResponse.json({
          cleared_count: removed.length,
          inputs: [
            ...entries.map(inputFromEntry),
            ...removed.map(entry => ({ ...inputFromEntry(entry), status: "canceled" as const })),
          ],
          queue_generation: 1,
        });
      }
    ),
    compozyApiMock.post(
      "/api/workspaces/{workspace_id}/sessions/{session_id}/prompt",
      async ({ request }) => {
        const body = (await request.json()) as {
          idempotency_key: string;
          message_id: string;
          messages?: { parts?: { text?: string; type: string }[] }[];
          mode?: string;
        };
        const text = (body.messages ?? [])
          .flatMap(message => message.parts ?? [])
          .filter(part => part.type === "text" && typeof part.text === "string")
          .map(part => part.text)
          .join("\n");
        const attempt = (promptAttempts.get(body.message_id) ?? 0) + 1;
        promptAttempts.set(body.message_id, attempt);
        if (
          prompt === "acknowledgment_lost" ||
          (prompt === "lost_then_replayed" && attempt === 1)
        ) {
          // The POST never reaches an answer (timeout, reconnect): the browser
          // keeps the identity and offers Retry.
          return HttpResponse.error();
        }
        if (prompt === "queue_full") {
          entries = QUEUE_STORY_FULL_ENTRIES.map(entry => ({ ...entry }));
          return HttpResponse.json(
            {
              error: "Queue is full",
              code: "queue_full",
              diagnostic: {
                id: "session.queue.full",
                code: "session_queue_full",
                severity: "error",
                category: "session",
                title: "Session queue is full",
                message: "The session input queue is at capacity.",
                data_freshness: "live",
                evidence: { queue_cap: cap, queue_count: cap },
              },
            },
            { status: 409 }
          );
        }
        const replayed = prompt === "lost_then_replayed";
        const id = `inp_${body.message_id.slice(-3)}`;
        if (!entries.some(entry => entry.id === id)) {
          entries = [...entries, { id, text }];
        }
        return HttpResponse.json({
          prompt: promptPayload(body, "queued", {
            disposition: "queued",
            queue_entry_id: id,
            queue_position: entries.length,
            replayed,
          }),
        });
      }
    ),
  ];
}

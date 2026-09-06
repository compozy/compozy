// Story transcripts for the quiet timeline (ADR-006..009, task_07 VC-01..VC-09)
// and the 3k-entry seed the fluidity walk (E2E-018) scrolls through.

import type { TranscriptMessage } from "@/systems/session/types";

import {
  TURN,
  part,
  bashPart,
  editPart,
  readPart,
  runningPart,
  textPart,
  userMessage,
  eventPart,
} from "./session-thread-story-quiet-parts";

export const liveToolTranscript: TranscriptMessage[] = [
  userMessage("story_quiet_user_1", "Refactor the flaky manager tests"),
  {
    id: "story_quiet_settled",
    role: "assistant",
    parts: [
      readPart(
        "q_read_1",
        "internal/session/manager_lifecycle_test.go",
        "2026-07-07T12:00:00Z",
        "story-turn-settled"
      ),
      bashPart(
        "q_bash_1",
        "go test ./internal/session/... -run Lifecycle -count=5",
        "2026-07-07T12:01:10Z",
        { turnId: "story-turn-settled" }
      ),
      editPart(
        "q_edit_1",
        "internal/session/manager_lifecycle_test.go",
        "2026-07-07T12:03:00Z",
        "story-turn-settled"
      ),
      bashPart("q_bash_1b", "go vet ./internal/session/...", "2026-07-07T12:03:30Z", {
        turnId: "story-turn-settled",
      }),
      textPart(
        "Done — 14 tests updated. The retry path in `manager_lifecycle_test.go` no longer depends on wall-clock sleeps; it waits on the lifecycle channel instead.",
        "2026-07-07T12:04:12Z",
        "story-turn-settled"
      ),
    ],
  },
  userMessage("story_quiet_user_2", "Now make the same change in the store package"),
  {
    id: "story_quiet_live",
    role: "assistant",
    status: { type: "running" },
    parts: [
      textPart(
        "Looking at how `internal/store` schedules its retries before I touch anything.",
        "2026-07-07T12:05:00Z"
      ),
      part({
        type: "tool-Grep",
        toolCallId: "q_grep_1",
        state: "output-available",
        turnId: TURN,
        timestamp: "2026-07-07T12:05:04Z",
        input: { pattern: "time.Sleep", path: "internal/store" },
        output: {
          type: "tool_result",
          title: "Grep",
          raw: { stdout: "internal/store/retry.go:41\n" },
        },
      }),
      bashPart("q_bash_2", "go test ./internal/store/... -run Retry", "2026-07-07T12:05:30Z"),
      editPart("q_edit_2", "internal/store/retry_test.go", "2026-07-07T12:06:10Z"),
      bashPart(
        "q_bash_3",
        "go test ./internal/store/... -run Retry -count=3",
        "2026-07-07T12:06:40Z",
        { running: true }
      ),
    ],
  } as TranscriptMessage,
];

/** Three tools in flight at once: still one row, counted (VC-02). */
export const parallelToolsTranscript: TranscriptMessage[] = [
  userMessage("story_parallel_user", "Vet, search and read the store package in parallel"),
  {
    id: "story_parallel_live",
    role: "assistant",
    status: { type: "running" },
    parts: [
      bashPart("p_bash_1", "go vet ./...", "2026-07-07T12:00:00Z", { running: true }),
      runningPart("tool-Grep", "p_grep_1", { pattern: "time.Sleep" }, "2026-07-07T12:00:01Z"),
      runningPart(
        "tool-Read",
        "p_read_1",
        { file_path: "internal/store/schema.sql" },
        "2026-07-07T12:00:02Z"
      ),
    ],
  } as TranscriptMessage,
];

/** Six settled tools behind one sentence, the live row below (VC-03). */
export const completedGroupTranscript: TranscriptMessage[] = [
  userMessage("story_group_user", "Now make the same change in the store package"),
  {
    id: "story_group_live",
    role: "assistant",
    status: { type: "running" },
    parts: [
      readPart("g_read_1", "internal/store/retry.go", "2026-07-07T12:00:00Z"),
      editPart("g_edit_1", "internal/store/retry_test.go", "2026-07-07T12:00:10Z"),
      bashPart("g_bash_1", "go test ./internal/store/... -run Retry", "2026-07-07T12:00:20Z"),
      bashPart("g_bash_2", "go vet ./internal/store/...", "2026-07-07T12:00:30Z"),
      readPart("g_read_2", "internal/store/queue.go", "2026-07-07T12:00:40Z"),
      readPart("g_read_3", "internal/store/schema.sql", "2026-07-07T12:00:50Z"),
      bashPart("g_bash_3", "go build ./...", "2026-07-07T12:01:00Z"),
      bashPart("g_bash_4", "go test ./... -race", "2026-07-07T12:01:10Z", { running: true }),
    ],
  } as TranscriptMessage,
];

/** One absorbed failure inside the group; the turn goes on (VC-04). */
export const partialFailureTranscript: TranscriptMessage[] = [
  userMessage("story_partial_user", "Run the retry tests three times"),
  {
    id: "story_partial_live",
    role: "assistant",
    status: { type: "running" },
    parts: [
      bashPart("f_bash_1", "go test ./internal/store/... -run Retry", "2026-07-07T12:00:00Z"),
      bashPart(
        "f_bash_2",
        "go test ./internal/store/... -run Retry -count=3",
        "2026-07-07T12:00:20Z",
        {
          error:
            "--- FAIL: TestRetry_NoWallClock (0.41s)\n    retry_test.go:44: attempts = 2, want 3\nFAIL\tgithub.com/compozy/compozy/internal/store\t0.912s",
        }
      ),
      editPart("f_edit_1", "internal/store/retry_test.go", "2026-07-07T12:00:40Z"),
      bashPart("f_bash_3", "go build ./...", "2026-07-07T12:00:50Z"),
      bashPart(
        "f_bash_4",
        "go test ./internal/store/... -run Retry -count=3",
        "2026-07-07T12:01:00Z",
        { running: true }
      ),
    ],
  } as TranscriptMessage,
];

/** The turn you stopped stays open where you stopped it (VC-06). */
export const interruptedOpenTranscript: TranscriptMessage[] = [
  userMessage("story_stopped_user", "Now make the same change in the store package"),
  {
    id: "story_stopped_turn",
    role: "assistant",
    parts: [
      textPart(
        "Looking at how `internal/store` schedules its retries before I touch anything.",
        "2026-07-07T12:00:00Z"
      ),
      bashPart("s_bash_1", "go test ./internal/store/... -run Retry", "2026-07-07T12:00:30Z"),
      bashPart(
        "s_bash_2",
        "go test ./internal/store/... -run Retry -count=3",
        "2026-07-07T12:01:10Z",
        { running: true }
      ),
      eventPart(
        {
          type: "prompt_interrupted",
          stop_reason: "cancelled",
          turn_id: TURN,
          text: "Prompt interrupted",
        },
        "2026-07-07T12:01:40Z"
      ),
    ],
  },
];

/** A steer that fell back to interrupt folds normally and names its cause (VC-06 right cell). */
export const steerFallbackTranscript: TranscriptMessage[] = [
  userMessage("story_fallback_user", "Now make the same change in the store package"),
  {
    id: "story_fallback_turn",
    role: "assistant",
    parts: [
      bashPart("b_bash_1", "go test ./internal/store/... -run Retry", "2026-07-07T12:00:00Z"),
      bashPart("b_bash_2", "go vet ./internal/store/...", "2026-07-07T12:00:20Z"),
      bashPart("b_bash_3", "go build ./...", "2026-07-07T12:00:40Z", { running: true }),
      eventPart(
        {
          type: "transcript_marker.created",
          turn_id: TURN,
          marker: {
            kind: "transcript_marker.prompt_steered",
            summary: "Steer interrupted and replaced the turn.",
            occurred_at: "2026-07-07T12:00:48Z",
            evidence: {
              steer_delivery: "interrupt_fallback",
              mode: "steer",
              message_id: "story_fallback_steer",
              authored_text: "Only touch the lifecycle tests, skip the store package",
              target_turn_id: TURN,
              queue_entry_id: "inq_fallback",
            },
          },
        },
        "2026-07-07T12:00:48Z"
      ),
    ],
  },
  userMessage("story_fallback_steer", "Only touch the lifecycle tests, skip the store package"),
];

function steerMarker(
  id: string,
  evidence: Record<string, unknown>,
  timestamp: string,
  summary = "Steer delivered into the live turn."
): TranscriptMessage {
  return {
    id,
    role: "assistant",
    parts: [
      eventPart(
        {
          type: "transcript_marker.created",
          marker: {
            kind: "transcript_marker.prompt_steered",
            summary,
            occurred_at: timestamp,
            evidence,
          },
        },
        timestamp
      ),
    ],
  };
}

/**
 * Steer provenance under the operator's bubbles (VC-07), bound by the daemon's
 * `message_id` on each marker: injected, pending (a receipt bubble from the
 * marker's own text — the message was never dispatched), superseded (quieter,
 * never removed), fallback, and a prompt from the queue.
 */
export const steerMarkersTranscript: TranscriptMessage[] = [
  userMessage("msg_steer_injected", "Only touch the lifecycle tests, skip the store package"),
  steerMarker(
    "story_steer_injected_marker",
    {
      steer_delivery: "injected",
      message_id: "msg_steer_injected",
      authored_text: "Only touch the lifecycle tests, skip the store package",
      target_turn_id: TURN,
      queue_entry_id: "inq_1",
    },
    "2026-07-07T12:00:01Z"
  ),
  steerMarker(
    "story_steer_pending_marker",
    {
      steer_delivery: "pending_injection",
      message_id: "msg_steer_pending",
      authored_text: "Use the lifecycle channel, not a ticker",
      target_turn_id: TURN,
      queue_entry_id: "inq_2",
    },
    "2026-07-07T12:00:11Z",
    "Steering is waiting for the active tool to finish."
  ),
  {
    id: "story_steer_superseded_marker",
    role: "assistant",
    parts: [
      eventPart(
        {
          type: "transcript_marker.created",
          marker: {
            kind: "transcript_marker.prompt_superseded",
            summary: "Undelivered steering replaced by newer guidance.",
            occurred_at: "2026-07-07T12:00:21Z",
            evidence: {
              message_id: "msg_steer_superseded",
              authored_text: "Actually keep the ticker but make it injectable",
              queue_entry_id: "inq_3",
              replacement_entry_id: "inq_4",
            },
          },
        },
        "2026-07-07T12:00:21Z"
      ),
    ],
  },
  userMessage("msg_steer_fallback", "Skip the store package entirely"),
  steerMarker(
    "story_steer_fallback_marker",
    {
      steer_delivery: "interrupt_fallback",
      message_id: "msg_steer_fallback",
      authored_text: "Skip the store package entirely",
      target_turn_id: TURN,
      queue_entry_id: "inq_4",
    },
    "2026-07-07T12:00:31Z",
    "Steering interrupted and replaced the active turn."
  ),
  {
    id: "story_steer_queued_marker",
    role: "assistant",
    parts: [
      eventPart(
        {
          type: "transcript_marker.created",
          title: "transcript_marker.prompt_queued",
          raw: {
            kind: "transcript_marker.prompt_queued",
            summary: "Input queued while the session is busy.",
            occurred_at: "2026-07-07T12:00:35Z",
            evidence: { queue_entry_id: "inq_5", queue_position: 1, mode: "queue" },
          },
        },
        "2026-07-07T12:00:35Z"
      ),
    ],
  },
  userMessage("msg_from_queue", "Ship it with tests"),
  {
    id: "story_steer_queue_marker",
    role: "assistant",
    parts: [
      eventPart(
        {
          type: "transcript_marker.created",
          title: "transcript_marker.prompt_accepted",
          raw: {
            kind: "transcript_marker.prompt_accepted",
            summary: "Queued input accepted for dispatch.",
            occurred_at: "2026-07-07T12:00:41Z",
            evidence: {
              queue_entry_id: "inq_5",
              message_id: "msg_from_queue",
              authored_text: "Ship it with tests",
              mode: "queue",
            },
          },
        },
        "2026-07-07T12:00:41Z"
      ),
    ],
  },
];

const GIANT_LINES = 12_480;

/** A verbose test run: 12,480 lines of stdout behind the truncation strip (VC-08). */
export const giantPayloadTranscript: TranscriptMessage[] = [
  userMessage("story_giant_user", "Run the whole suite verbosely"),
  {
    id: "story_giant_turn",
    role: "assistant",
    parts: [
      bashPart("giant_bash", "go test ./... -race -v", "2026-07-07T12:00:00Z", {
        stdout: Array.from({ length: GIANT_LINES }, (_, index) =>
          index % 4 === 3
            ? `--- PASS: TestCase_${Math.floor(index / 4)} (0.0${index % 9}s)`
            : `=== RUN   TestCase_${Math.floor(index / 4)}/step_${index % 4}`
        ).join("\n"),
      }),
      textPart(
        "The suite is green; the verbose log is kept behind the strip.",
        "2026-07-07T12:04:00Z"
      ),
    ],
  },
];

/** Streaming prose mid-reveal (VC-09): a text part still streaming, no tool. */
export const streamingProseTranscript: TranscriptMessage[] = [
  userMessage("story_stream_user", "Summarize what changed"),
  {
    id: "story_stream_turn",
    role: "assistant",
    status: { type: "running" },
    parts: [
      textPart(
        "Done — 14 tests updated. The retry path in `manager_lifecycle_test.go` no longer depends on wall-clock sleeps; it waits on the lifecycle channel instead. Three of the manager tests were passing or failing depending on scheduler timing, so I replaced the fixed 50ms sleeps with a settle signal from the manager itself.",
        "2026-07-07T12:00:00Z",
        TURN,
        "streaming"
      ),
    ],
  } as TranscriptMessage,
];

/** Page size the daemon serves the transcript in. */
export const LARGE_TRANSCRIPT_PAGE = 200;

/**
 * A seeded 3k-entry session (E2E-018): alternating short prompts and settled
 * tool turns, served page by page through `before_sequence` like the daemon.
 */
export function largeTranscriptEntries(count = 3_000) {
  const messages: TranscriptMessage[] = [];
  for (let index = 0; index < count; index += 1) {
    const sequence = index + 1;
    const turnId = `large-turn-${Math.floor(index / 2)}`;
    const at = new Date(Date.UTC(2026, 6, 7, 8, 0, index * 7)).toISOString();
    if (index % 2 === 0) {
      messages.push(
        userMessage(
          `large_user_${sequence}`,
          `Check item ${Math.floor(index / 2) + 1} of the launch list`
        )
      );
    } else {
      messages.push({
        id: `large_assistant_${sequence}`,
        role: "assistant",
        parts: [
          readPart(`large_read_${sequence}`, `/workspace/app/src/item-${sequence}.ts`, at, turnId),
          bashPart(`large_bash_${sequence}`, `bun test src/item-${sequence}.test.ts`, at, {
            turnId,
            stdout: "1 pass\n",
          }),
          textPart(`Item ${Math.floor(index / 2) + 1} is green.`, at, turnId),
        ],
      });
    }
  }
  return messages.map((message, index) => ({
    message,
    sequence: index + 1,
    start_sequence: index + 1,
  }));
}

/** Serve one page of the seeded transcript the way the daemon does: newest first, older on demand. */
export function largeTranscriptPage(
  entries: ReturnType<typeof largeTranscriptEntries>,
  beforeSequence: number | null,
  limit = LARGE_TRANSCRIPT_PAGE
) {
  const end = beforeSequence === null ? entries.length : Math.max(0, beforeSequence - 1);
  const start = Math.max(0, end - limit);
  const page = entries.slice(start, end);
  const first = page[0];
  return {
    entries: page,
    epoch: 1,
    generation: 1,
    has_older: start > 0,
    limit,
    max_sequence: entries.length,
    ...(start > 0 && first ? { next_before_sequence: first.sequence } : {}),
  };
}

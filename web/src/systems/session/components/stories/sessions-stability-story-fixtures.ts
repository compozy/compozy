// Fixtures for the sessions-stability visual-state stories (task_10 matrix):
// public-wire transcripts and the session resource shapes the status row
// reads. The daemon's route answers over these fixtures, the transport
// snapshots, and the story loader live in `sessions-stability-story-routes.ts`.

import { primarySessionFixture } from "@/systems/session/mocks";
import type { SessionPayload, TranscriptMessage } from "@/systems/session/types";

type TranscriptPart = NonNullable<TranscriptMessage["parts"]>[number];

export const STABILITY_SESSION = primarySessionFixture;
export const STABILITY_WORKSPACE_ID = primarySessionFixture.workspace_id ?? "ws_alpha";

const SETTLED_TURN = "turn-story-settled";
const REPLY_TURN = "turn-story-reply";
const LIVE_TURN = "turn-story-live";

function part(value: Record<string, unknown>): TranscriptPart {
  return value as unknown as TranscriptPart;
}

function toolPart(
  type: string,
  id: string,
  input: Record<string, unknown>,
  timestamp: string,
  turnId: string,
  options: { running?: boolean; stdout?: string } = {}
): TranscriptPart {
  if (options.running) {
    return part({ type, toolCallId: id, state: "input-available", turnId, timestamp, input });
  }
  return part({
    type,
    toolCallId: id,
    state: "output-available",
    turnId,
    timestamp,
    input,
    output: {
      type: "tool_result",
      title: type.slice("tool-".length),
      raw: { stdout: options.stdout ?? "ok\n" },
    },
  });
}

function textPart(
  text: string,
  timestamp: string,
  turnId: string,
  state: "done" | "streaming" = "done"
): TranscriptPart {
  return part({ type: "text", text, state, turnId, timestamp });
}

function eventPart(
  data: Record<string, unknown>,
  timestamp: string,
  turnId: string
): TranscriptPart {
  return part({
    type: "data-compozy-event",
    turnId,
    timestamp,
    data: { ...data, turn_id: turnId, timestamp },
  });
}

function userMessage(
  id: string,
  text: string,
  turnId: string,
  timestamp: string
): TranscriptMessage {
  return {
    id,
    role: "user",
    metadata: { turn_id: turnId, message_id: id, timestamp },
    parts: [{ type: "text", text, state: "done" }],
  } as TranscriptMessage;
}

const FIRST_ASK = "Refactor the flaky manager tests — the lifecycle ones first";
const FIRST_REPLY =
  "Done — 14 tests updated. The retry path in manager_lifecycle_test.go no longer depends on wall-clock sleeps; it waits on the lifecycle channel instead.";
const SECOND_ASK = "Only touch the lifecycle tests, skip the store package";
const SECOND_REPLY =
  "Understood — the store package is untouched. The lifecycle tests now wait on the manager's settle signal instead of sleeping; 14 updated, all green with -count=5.";

/**
 * Two settled turns (timeline VC-05 both cells, find-trail VC-01..04): the
 * first folds behind "Worked for 4m 12s · Ran 6 commands, edited 2 files, read
 * 1 file" over the answer; the second has no tool work, so no fold row at all.
 * "lifecycle" occurs in every entry: the ask, the first turn (first inside the
 * fold, on the read path), and the second exchange — four matches, one folded.
 */
export const settledTurnsTranscript: TranscriptMessage[] = [
  userMessage("msg_story_ask_1", FIRST_ASK, SETTLED_TURN, "2026-09-06T13:40:00Z"),
  {
    id: "msg_story_turn_1",
    role: "assistant",
    parts: [
      toolPart(
        "tool-Read",
        "t1_read",
        { file_path: "internal/session/manager_lifecycle_test.go" },
        "2026-09-06T13:40:00Z",
        SETTLED_TURN
      ),
      toolPart(
        "tool-Bash",
        "t1_bash_1",
        { command: "go test ./internal/session/... -run Lifecycle -count=5" },
        "2026-09-06T13:40:40Z",
        SETTLED_TURN
      ),
      toolPart(
        "tool-Bash",
        "t1_bash_2",
        { command: "go vet ./internal/session/..." },
        "2026-09-06T13:41:10Z",
        SETTLED_TURN
      ),
      toolPart(
        "tool-Edit",
        "t1_edit_1",
        {
          file_path: "internal/session/manager_lifecycle_test.go",
          old_string: "\ttime.Sleep(50 * time.Millisecond)",
          new_string: "\t<-mgr.Lifecycle().Settled()",
        },
        "2026-09-06T13:41:50Z",
        SETTLED_TURN
      ),
      toolPart(
        "tool-Bash",
        "t1_bash_3",
        { command: "go test ./internal/session/... -run Lifecycle -count=5" },
        "2026-09-06T13:42:20Z",
        SETTLED_TURN
      ),
      toolPart(
        "tool-Bash",
        "t1_bash_4",
        { command: "go test ./internal/session/... -race" },
        "2026-09-06T13:42:55Z",
        SETTLED_TURN
      ),
      toolPart(
        "tool-Edit",
        "t1_edit_2",
        {
          file_path: "internal/session/manager_test.go",
          old_string: "\ttime.Sleep(20 * time.Millisecond)",
          new_string: "\t<-mgr.Lifecycle().Settled()",
        },
        "2026-09-06T13:43:20Z",
        SETTLED_TURN
      ),
      toolPart(
        "tool-Bash",
        "t1_bash_5",
        { command: "go test ./internal/session/... -count=5" },
        "2026-09-06T13:43:40Z",
        SETTLED_TURN
      ),
      toolPart(
        "tool-Bash",
        "t1_bash_6",
        { command: "gofmt -l internal/session" },
        "2026-09-06T13:44:00Z",
        SETTLED_TURN
      ),
      textPart(FIRST_REPLY, "2026-09-06T13:44:12Z", SETTLED_TURN),
    ],
  },
  userMessage("msg_story_ask_2", SECOND_ASK, REPLY_TURN, "2026-09-06T14:03:00Z"),
  {
    id: "msg_story_turn_2",
    role: "assistant",
    parts: [textPart(SECOND_REPLY, "2026-09-06T14:03:20Z", REPLY_TURN)],
  },
];

/** The operator just sent; nothing exists for the reply yet (working VC-10). */
export const thinkingTranscript: TranscriptMessage[] = [
  ...settledTurnsTranscript.slice(0, 2),
  userMessage(
    "msg_story_ask_live",
    "Now make the same change in the store package",
    LIVE_TURN,
    "2026-09-06T14:05:00Z"
  ),
];

/**
 * The daemon closed the turn for the operator after the agent ignored the stop
 * (working VC-12, second sentence): the durable turn receipt says verified and
 * escalated; the call it cut short reads "stopped".
 */
export const escalatedStopTranscript: TranscriptMessage[] = [
  userMessage(
    "msg_story_ask_esc",
    "Now make the same change in the store package",
    LIVE_TURN,
    "2026-09-06T14:05:00Z"
  ),
  {
    id: "msg_story_turn_esc",
    role: "assistant",
    parts: [
      textPart(
        "Looking at how internal/store schedules its retries before I touch anything.",
        "2026-09-06T14:05:02Z",
        LIVE_TURN
      ),
      toolPart(
        "tool-Bash",
        "esc_bash",
        { command: "go test ./internal/store/... -run Retry -count=3" },
        "2026-09-06T14:05:06Z",
        LIVE_TURN,
        { running: true }
      ),
      eventPart(
        {
          type: "session.turn_quiesced",
          raw: {
            scope: "turn",
            turn_id: LIVE_TURN,
            verified: true,
            escalated: true,
            phase: "forced",
            elapsed_ms: 10_000,
            stop_cause: "user_requested",
          },
        },
        "2026-09-06T14:05:16Z",
        LIVE_TURN
      ),
    ],
  },
];

/** Supervision stopped the session after 40 quiet minutes (working VC-12, third sentence). */
export const inactivityStopTranscript: TranscriptMessage[] = [
  ...settledTurnsTranscript.slice(0, 2),
  userMessage("msg_story_ask_idle", SECOND_ASK, REPLY_TURN, "2026-09-06T14:03:00Z"),
  {
    id: "msg_story_turn_idle",
    role: "assistant",
    parts: [
      textPart(SECOND_REPLY, "2026-09-06T14:03:20Z", REPLY_TURN),
      eventPart(
        {
          type: "session.supervision_stopped",
          raw: {
            supervision: {
              quiet_warning: {
                quiet_since: "2026-09-06T13:54:00Z",
                warned_at: "2026-09-06T14:24:00Z",
                stop_at: "2026-09-06T14:34:00Z",
              },
            },
          },
        },
        "2026-09-06T14:34:00Z",
        REPLY_TURN
      ),
    ],
  },
];

/** The session resource after supervision's inactivity stop. */
export const inactivityStoppedSession: SessionPayload = {
  ...primarySessionFixture,
  badge: "stopped",
  state: "stopped",
  stop_cause: "inactivity",
} as SessionPayload;

/** A running session resource: durable turn start, current tool, spawned children. */
export function runningSession(
  options: { currentTool?: string; agents?: number; startedAgoMs?: number; turnId?: string } = {}
): SessionPayload {
  const startedAgoMs = options.startedAgoMs ?? 134_000;
  return {
    ...primarySessionFixture,
    activity: {
      current_tool: options.currentTool,
      elapsed_ms: startedAgoMs,
      elapsed_seconds: Math.round(startedAgoMs / 1000),
      idle_seconds: 0,
      iteration_current: 1,
      iteration_max: 1,
      turn_id: options.turnId ?? LIVE_TURN,
      turn_started_at: new Date(Date.now() - startedAgoMs).toISOString(),
    },
    supervision: {
      quiet_warning: null,
      sources: [],
      work_signals: Array.from({ length: options.agents ?? 0 }, (_, index) => ({
        kind: "active_child" as const,
        since: new Date(Date.now() - 60_000).toISOString(),
        ref: `sess_child_${index + 1}`,
      })),
    },
  } as SessionPayload;
}

/**
 * The lead reply landed, two spawned agents still run (working VC-11): the
 * daemon projects a child agent as a `Task` tool call still awaiting its result
 * (`input-available`), which the timeline renders as its own live row with the
 * bot glyph — never grouped, never folded.
 */
export const childrenRunningTranscript: TranscriptMessage[] = [
  ...settledTurnsTranscript.slice(0, 2),
  userMessage("msg_story_ask_children", SECOND_ASK, LIVE_TURN, "2026-09-06T14:03:00Z"),
  {
    id: "msg_story_turn_children",
    role: "assistant",
    status: { type: "running" },
    parts: [
      textPart(
        "The lifecycle tests are green locally. I've asked reviewer to look at the diff and scout to check whether the store package has the same sleep pattern.",
        "2026-09-06T14:03:20Z",
        LIVE_TURN
      ),
      toolPart(
        "tool-Task",
        "child_reviewer",
        {
          subagent_type: "reviewer",
          description: "reviewer — reviewing the diff in HEAD~1..HEAD",
          prompt: "Review the lifecycle test changes in HEAD~1..HEAD for timing assumptions.",
        },
        "2026-09-06T14:04:00Z",
        LIVE_TURN,
        { running: true }
      ),
      toolPart(
        "tool-Task",
        "child_scout",
        {
          subagent_type: "scout",
          description: "scout — searching internal/store for time.Sleep",
          prompt: "Find every time.Sleep in internal/store and report the callers.",
        },
        "2026-09-06T14:04:22Z",
        LIVE_TURN,
        { running: true }
      ),
    ],
  } as TranscriptMessage,
];

const GIANT_LINES = 12_480;

/**
 * A completed 12,480-line command inside the live turn (timeline VC-08, row
 * collapsed on load): the reply after it is still streaming, so the turn is
 * live — a single settled call of the live turn stays a row, no fold, no
 * group — and the collapsed row and its expanded body are both reachable
 * without opening a disclosure first.
 */
export const giantPayloadLiveTranscript: TranscriptMessage[] = [
  userMessage(
    "msg_story_ask_giant",
    "Run the whole suite verbosely",
    LIVE_TURN,
    "2026-09-06T14:05:00Z"
  ),
  {
    id: "msg_story_turn_giant",
    role: "assistant",
    status: { type: "running" },
    parts: [
      toolPart(
        "tool-Bash",
        "giant_live_bash",
        { command: "go test ./... -race -v" },
        "2026-09-06T14:05:04Z",
        LIVE_TURN,
        {
          stdout: Array.from({ length: GIANT_LINES }, (_, index) =>
            index % 4 === 3
              ? `--- PASS: TestCase_${Math.floor(index / 4)} (0.0${index % 9}s)`
              : `=== RUN   TestCase_${Math.floor(index / 4)}/step_${index % 4}`
          ).join("\n"),
        }
      ),
      textPart(
        "The suite is green; reading the verbose log for flakes next.",
        "2026-09-06T14:09:00Z",
        LIVE_TURN,
        "streaming"
      ),
    ],
  } as TranscriptMessage,
];

/** Sixty exchanges for the dense trail (find-trail VC-05). */
export function denseTranscript(exchanges = 60): TranscriptMessage[] {
  const messages: TranscriptMessage[] = [];
  for (let index = 0; index < exchanges; index += 1) {
    const turnId = `turn-story-dense-${index + 1}`;
    const at = new Date(Date.UTC(2026, 8, 6, 13, index)).toISOString();
    messages.push(
      userMessage(
        `msg_story_dense_ask_${index + 1}`,
        `Step ${index + 1}: ${[FIRST_ASK, SECOND_ASK][index % 2]}`,
        turnId,
        at
      ),
      {
        id: `msg_story_dense_turn_${index + 1}`,
        role: "assistant",
        parts: [textPart([FIRST_REPLY, SECOND_REPLY][index % 2]!, at, turnId)],
      }
    );
  }
  return messages;
}

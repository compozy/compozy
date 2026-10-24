// Live turn steered by queued subagent wakes (issue #923), sanitized from a
// real session's transcript projection: the daemon splits one turn into its
// first chunk, one marker message per wake delivered into the live turn (both
// landing between a pending tool call and its result), and the turn's `-2`
// continuation. Public-wire shapes only — markers keep the persisted `raw`.

import type { TranscriptMessage } from "@/systems/session/types";

type TranscriptPart = NonNullable<TranscriptMessage["parts"]>[number];

export const STEER_WAKE_TURN = "turn-story-steer-wakes";

function part(value: Record<string, unknown>): TranscriptPart {
  return value as unknown as TranscriptPart;
}

function event(data: Record<string, unknown>, timestamp: string): TranscriptPart {
  return part({
    type: "data-compozy-event",
    data: { ...data, turn_id: STEER_WAKE_TURN, timestamp },
  });
}

function toolCall(
  name: string,
  id: string,
  input: Record<string, unknown>,
  timestamp: string,
  stdout: string | null
): TranscriptPart[] {
  const tool =
    stdout === null
      ? part({ type: `tool-${name}`, toolCallId: id, title: name, state: "input-available", input })
      : part({
          type: `tool-${name}`,
          toolCallId: id,
          title: name,
          state: "output-available",
          input,
          output: { type: "tool_result", title: name, raw: { stdout } },
        });
  return [
    tool,
    event({ type: "tool_call", tool_name: name, tool_call_id: id, status: "pending" }, timestamp),
    // The provider's input update carries no status (the real stream's shape).
    event({ type: "tool_call", tool_name: name, tool_call_id: id, tool_input: input }, timestamp),
  ];
}

const STATUS_TOOL = "mcp__compozy-hosted-tools__compozy__subagent_status";
const STATUS_RESULT = '{"status":"completed","result":"VERDICT: ship"}';

function usage(timestamp: string): TranscriptPart {
  return event(
    { type: "usage", usage: { context_used: 551_073, context_size: 1_000_000 } },
    timestamp
  );
}

function wakeMarker(id: string, messageId: string, title: string, at: string): TranscriptMessage {
  const summary = `Subagent "${title}" finished: completed.\nCall compozy__subagent_status to read its result.`;
  return {
    id,
    role: "assistant",
    parts: [
      event(
        {
          type: "transcript_marker.created",
          text: summary,
          title: "transcript_marker.prompt_steered",
          raw: {
            kind: "transcript_marker.prompt_steered",
            occurred_at: at,
            summary,
            evidence: { kind: "subagent_wake", message_id: messageId },
          },
        },
        at
      ),
    ],
  } as TranscriptMessage;
}

/**
 * The live turn the operator reported: "continue" after provider-limit
 * failures, then subagent_status calls, two wakes steered in mid-call, a
 * thought followed by a whitespace-only thought, and the turn still running.
 * `wakeIds` are the identities the markers name: distinct wake ids as the
 * daemon now records them, or the collapsed `[REDACTED]` that older events
 * persisted when evidence ids went through the free-text entropy heuristic.
 */
export function steeredWakesTranscript(wakeIds: readonly [string, string]): TranscriptMessage[] {
  return [
    {
      id: "msg_story_steer_continue",
      role: "user",
      metadata: {
        turn_id: STEER_WAKE_TURN,
        message_id: "msg_story_steer_continue",
        timestamp: "2026-10-23T17:02:37Z",
      },
      parts: [{ type: "text", text: "continue, the limits are back", state: "done" }],
    } as TranscriptMessage,
    {
      id: STEER_WAKE_TURN,
      role: "assistant",
      parts: [
        event(
          { type: "available_commands_update", available_commands: [] },
          "2026-10-23T17:02:46Z"
        ),
        usage("2026-10-23T17:02:59Z"),
        ...toolCall(
          STATUS_TOOL,
          "toolu_status_1",
          { subagent_id: "sub-fix" },
          "2026-10-23T17:03:00Z",
          STATUS_RESULT
        ),
        ...toolCall(
          STATUS_TOOL,
          "toolu_status_2",
          { subagent_id: "sub-qa" },
          "2026-10-23T17:03:01Z",
          STATUS_RESULT
        ),
        ...toolCall(
          STATUS_TOOL,
          "toolu_status_3",
          { subagent_id: "sub-review" },
          "2026-10-23T17:03:01Z",
          null
        ),
      ],
    } as TranscriptMessage,
    wakeMarker(
      "ev-story-steer-wake-1",
      wakeIds[0],
      "review Astra xhigh",
      "2026-10-23T17:03:02.133Z"
    ),
    wakeMarker(
      "ev-story-steer-wake-2",
      wakeIds[1],
      "review Fable high",
      "2026-10-23T17:03:02.141Z"
    ),
    {
      id: `${STEER_WAKE_TURN}-2`,
      role: "assistant",
      parts: [
        usage("2026-10-23T17:03:02Z"),
        usage("2026-10-23T17:03:12Z"),
        ...toolCall(
          STATUS_TOOL,
          "toolu_status_4",
          { subagent_id: "sub-astra" },
          "2026-10-23T17:03:12Z",
          STATUS_RESULT
        ),
        ...toolCall(
          STATUS_TOOL,
          "toolu_status_5",
          { subagent_id: "sub-fable" },
          "2026-10-23T17:03:13Z",
          STATUS_RESULT
        ),
        usage("2026-10-23T17:03:23Z"),
        part({
          type: "reasoning",
          text: "The limits are back, so I resume the pending work: the doctor fix worker, the Fable review, and the visual QA.",
          state: "done",
        }),
        // The provider's trailing whitespace-only thought, as its own update.
        part({ type: "reasoning", text: "\n\n", state: "done" }),
        ...toolCall(
          "Bash",
          "toolu_bash_1",
          { command: "git add -A && git commit -m wip" },
          "2026-10-23T17:03:31Z",
          "ok\n"
        ),
        ...toolCall(
          "Bash",
          "toolu_bash_2",
          { command: "git log --oneline -1" },
          "2026-10-23T17:03:37Z",
          "3cba6e3 wip\n"
        ),
        ...toolCall(
          "Bash",
          "toolu_bash_3",
          { command: "git status --short" },
          "2026-10-23T17:03:52Z",
          null
        ),
      ],
    } as TranscriptMessage,
  ];
}

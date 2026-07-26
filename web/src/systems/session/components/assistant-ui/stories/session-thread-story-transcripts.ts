import type { TranscriptMessage } from "@/systems/session/types";

type TranscriptPart = NonNullable<TranscriptMessage["parts"]>[number];

function readToolPart(
  index: number,
  options: { turnId?: string; timestamp?: string } = {}
): TranscriptPart {
  return {
    type: "tool-Read",
    toolCallId: `story_tool_read_${index}`,
    state: "output-available",
    ...(options.turnId ? { turnId: options.turnId } : {}),
    ...(options.timestamp ? { timestamp: options.timestamp } : {}),
    input: {
      file_path: `/workspace/app/src/file-${index}.ts`,
    },
    output: {
      type: "tool_result",
      title: "Read",
      raw: {
        stdout: `export const fixture${index} = true;\n`,
      },
    },
  } as unknown as TranscriptPart;
}

function textTurnPart(text: string, turnId: string, timestamp: string): TranscriptPart {
  return {
    type: "text",
    text,
    state: "done",
    turnId,
    timestamp,
  } as unknown as TranscriptPart;
}

function reasoningTurnPart(text: string, turnId: string, timestamp: string): TranscriptPart {
  return {
    type: "reasoning",
    text,
    state: "done",
    turnId,
    timestamp,
  } as unknown as TranscriptPart;
}

export const mixedStreamingTranscript: TranscriptMessage[] = [
  {
    id: "story_user_mixed",
    role: "user",
    parts: [
      {
        type: "text",
        text: "Check the launch note, then summarize the risk.",
        state: "done",
      },
    ],
  },
  {
    id: "story_assistant_mixed",
    role: "assistant",
    parts: [
      {
        type: "text",
        text: "I will inspect the current launch note first.",
        state: "done",
      },
      {
        type: "reasoning",
        text: "Need the live note and the latest verification output before giving the operator a final answer.",
        state: "streaming",
      },
      {
        type: "data-provider-note",
        data: {
          title: "Provider note",
          detail: "Unregistered data event kept inline.",
        },
      },
      ...Array.from({ length: 8 }, (_, index) => readToolPart(index + 1)),
      {
        type: "text",
        text: [
          "The launch note is present and the web checks are green.",
          "",
          "| Area | Status |",
          "| --- | --- |",
          "| Chat stream | Fixed |",
          "| Tool timeline | Inline |",
          "",
          "```ts",
          'const nextAction = "keep monitoring";',
          "```",
        ].join("\n"),
        state: "streaming",
      },
    ],
  },
];

export const foldedTurnsTranscript: TranscriptMessage[] = [
  {
    id: "story_user_folded",
    role: "user",
    parts: [
      {
        type: "text",
        text: "Review three work turns and leave the final answer readable.",
        state: "done",
      },
    ],
  },
  {
    id: "story_assistant_folded_1",
    role: "assistant",
    parts: [
      reasoningTurnPart(
        "First turn: inspect the launch checklist before touching copy.",
        "story-turn-1",
        "2026-07-07T12:00:00Z"
      ),
      readToolPart(1, {
        turnId: "story-turn-1",
        timestamp: "2026-07-07T12:00:03Z",
      }),
      textTurnPart(
        "Launch checklist is present and current.",
        "story-turn-1",
        "2026-07-07T12:00:05Z"
      ),
    ],
  },
  {
    id: "story_assistant_folded_2",
    role: "assistant",
    parts: [
      ...Array.from({ length: 8 }, (_, index) =>
        readToolPart(index + 2, {
          turnId: "story-turn-2",
          timestamp: `2026-07-07T12:00:${10 + index}Z`,
        })
      ),
      textTurnPart(
        "The eight referenced files agree on the launch-state language.",
        "story-turn-2",
        "2026-07-07T12:00:20Z"
      ),
    ],
  },
  {
    id: "story_assistant_folded_3",
    role: "assistant",
    parts: [
      reasoningTurnPart(
        "Final turn: verify the command output and patch summary.",
        "story-turn-3",
        "2026-07-07T12:01:00Z"
      ),
      {
        type: "tool-Bash",
        toolCallId: "story_tool_bash_folded",
        state: "output-available",
        turnId: "story-turn-3",
        timestamp: "2026-07-07T12:01:05Z",
        input: {
          command: "bunx turbo run test --filter=./web",
        },
        output: {
          type: "tool_result",
          title: "Bash",
          raw: {
            stdout: "web tests passed\n",
          },
        },
      } as unknown as TranscriptPart,
      {
        type: "tool-Edit",
        toolCallId: "story_tool_edit_folded",
        state: "output-available",
        turnId: "story-turn-3",
        timestamp: "2026-07-07T12:01:08Z",
        input: {
          file_path: "/workspace/app/src/launch-note.ts",
          old_string: "const status = 'pending';",
          new_string: "const status = 'ready';",
        },
        output: {
          type: "tool_result",
          title: "Edit",
          raw: {
            content: "Applied patch successfully.",
          },
        },
      } as unknown as TranscriptPart,
      textTurnPart(
        "All three work turns are complete; the final launch note is ready.",
        "story-turn-3",
        "2026-07-07T12:01:12Z"
      ),
    ],
  },
];

function editToolPart(
  index: number,
  filePath: string,
  oldString: string,
  newString: string,
  timestamp: string
): TranscriptPart {
  return {
    type: "tool-Edit",
    toolCallId: `story_tool_edit_${index}`,
    state: "output-available",
    turnId: "story-turn-edits",
    timestamp,
    input: { file_path: filePath, old_string: oldString, new_string: newString },
    output: { type: "tool_result", title: "Edit", raw: { content: "Applied patch." } },
  } as unknown as TranscriptPart;
}

export const changedFilesTranscript: TranscriptMessage[] = [
  {
    id: "story_user_changed_files",
    role: "user",
    parts: [
      {
        type: "text",
        text: "Flip the launch flag to ready and refresh the release notes.",
        state: "done",
      },
    ],
  },
  {
    id: "story_assistant_changed_files",
    role: "assistant",
    parts: [
      editToolPart(
        1,
        "/workspace/app/src/launch-note.ts",
        "const status = 'pending';",
        "const status = 'ready';",
        "2026-07-07T12:00:00Z"
      ),
      editToolPart(
        2,
        "/workspace/app/src/config/flags.ts",
        "launchReady: false,",
        "launchReady: true,\n  launchReadyAt: Date.now(),",
        "2026-07-07T12:00:03Z"
      ),
      {
        type: "tool-Write",
        toolCallId: "story_tool_write_release",
        state: "output-available",
        turnId: "story-turn-edits",
        timestamp: "2026-07-07T12:00:06Z",
        input: {
          file_path: "/workspace/RELEASE.md",
          content: ["# Release", "", "- Launch flag set to ready.", "- Notes refreshed."].join(
            "\n"
          ),
        },
        output: { type: "tool_result", title: "Write", raw: { content: "Wrote RELEASE.md." } },
      } as unknown as TranscriptPart,
      textTurnPart(
        "Launch flag is ready and the release notes are refreshed.",
        "story-turn-edits",
        "2026-07-07T12:00:09Z"
      ),
    ],
  },
];

export const hoverToolbarTranscript: TranscriptMessage[] = [
  {
    id: "story_user_hover",
    role: "user",
    parts: [
      {
        type: "text",
        text: "Summarize the launch readiness in one line.",
        state: "done",
      },
    ],
  },
  {
    id: "story_assistant_hover",
    role: "assistant",
    parts: [
      textTurnPart(
        "Launch readiness is green: the launch note is present, web checks pass, and no blocking risks remain.",
        "story-turn-hover",
        "2026-07-07T12:00:05Z"
      ),
    ],
  },
];

function transcriptEntries(messages: TranscriptMessage[]) {
  return messages.map((message, index) => ({
    message,
    sequence: index + 1,
    start_sequence: index + 1,
  }));
}

export function transcriptPayload(messages: TranscriptMessage[]) {
  return {
    entries: transcriptEntries(messages),
    epoch: 1,
    generation: 1,
    has_older: false,
    limit: 200,
    max_sequence: messages.length,
  };
}

import type { TranscriptMessage } from "@/systems/session/types";

export type TranscriptPart = NonNullable<TranscriptMessage["parts"]>[number];

export const TURN = "story-turn-quiet";

export function part(value: Record<string, unknown>): TranscriptPart {
  return value as unknown as TranscriptPart;
}

export function bashPart(
  id: string,
  command: string,
  timestamp: string,
  options: { stdout?: string; error?: string; running?: boolean; turnId?: string } = {}
): TranscriptPart {
  const turnId = options.turnId ?? TURN;
  if (options.running) {
    return part({
      type: "tool-Bash",
      toolCallId: id,
      state: "input-available",
      turnId,
      timestamp,
      input: { command },
    });
  }
  return part({
    type: "tool-Bash",
    toolCallId: id,
    state: options.error ? "output-error" : "output-available",
    turnId,
    timestamp,
    input: { command },
    ...(options.error ? { errorText: options.error } : {}),
    output: {
      type: "tool_result",
      title: "Bash",
      raw: options.error ? { error: options.error } : { stdout: options.stdout ?? "ok\n" },
    },
  });
}

export function editPart(
  id: string,
  filePath: string,
  timestamp: string,
  turnId = TURN
): TranscriptPart {
  return part({
    type: "tool-Edit",
    toolCallId: id,
    state: "output-available",
    turnId,
    timestamp,
    input: {
      file_path: filePath,
      old_string: "\ttime.Sleep(50 * time.Millisecond)",
      new_string: "\t<-mgr.Lifecycle().Settled()",
    },
    output: { type: "tool_result", title: "Edit", raw: { content: "Applied patch." } },
  });
}

export function readPart(
  id: string,
  filePath: string,
  timestamp: string,
  turnId = TURN
): TranscriptPart {
  return part({
    type: "tool-Read",
    toolCallId: id,
    state: "output-available",
    turnId,
    timestamp,
    input: { file_path: filePath },
    output: { type: "tool_result", title: "Read", raw: { stdout: "package store\n" } },
  });
}

export function runningPart(
  type: string,
  id: string,
  input: Record<string, unknown>,
  timestamp: string
) {
  return part({ type, toolCallId: id, state: "input-available", turnId: TURN, timestamp, input });
}

export function textPart(
  text: string,
  timestamp: string,
  turnId = TURN,
  state = "done"
): TranscriptPart {
  return part({ type: "text", text, state, turnId, timestamp });
}

export function userMessage(id: string, text: string): TranscriptMessage {
  return { id, role: "user", parts: [{ type: "text", text, state: "done" }] };
}

export function eventPart(data: Record<string, unknown>, timestamp: string): TranscriptPart {
  return part({
    type: "data-compozy-event",
    turnId: TURN,
    timestamp,
    data: { ...data, timestamp },
  });
}

/** A settled turn followed by a live one: completed group + the one live row (VC-01, §01 anatomy). */

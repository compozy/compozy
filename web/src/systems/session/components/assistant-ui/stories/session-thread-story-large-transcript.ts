// The 3k-entry seed the fluidity walk (E2E-018) scrolls through.

import type { TranscriptMessage } from "@/systems/session/types";

import { bashPart, readPart, textPart, userMessage } from "./session-thread-story-quiet-parts";

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

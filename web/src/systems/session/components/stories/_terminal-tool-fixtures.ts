import type { UIMessage } from "@/systems/session/types";

const ESC = String.fromCharCode(27);
const CRLF = String.fromCharCode(13, 10);

/** Supervised terminal tool calls: the envelope rides `rawOutput`, as the runtime sends it. */
function terminalToolMessage(
  id: string,
  envelope: Record<string, unknown>,
  input: Record<string, unknown> = {}
): UIMessage {
  return {
    id,
    role: "tool_result",
    content: "",
    toolName: "compozy__terminal_exec",
    toolInput: { command: "bun run test", ...input },
    toolResult: {
      content: JSON.stringify(envelope),
      rawOutput: { structured: envelope },
    },
    timestamp: Date.parse("2026-04-17T16:09:00Z"),
  };
}

export const terminalPassedFixture = terminalToolMessage(
  "tool_terminal_passed",
  {
    terminal_id: "term-4f21c9a03b7e",
    command_id: "cmd-01",
    exit_code: 0,
    duration_ms: 161_000,
    still_running: false,
    output: [
      `web:test: ${ESC}[32m227 passed${ESC}[0m (14.2s)`,
      "go: ok  internal/terminal  2.148s",
      `${ESC}[1;32mgate: PASS${ESC}[0m`,
    ].join(CRLF),
  },
  { title: "make gate" }
);

export const terminalFailedFixture = terminalToolMessage(
  "tool_terminal_failed",
  {
    terminal_id: "term-9a01c3d4e5f6",
    command_id: "cmd-02",
    exit_code: 1,
    duration_ms: 4_200,
    still_running: false,
    output: `FAIL src/session.test.ts${CRLF}  expected 2, received 3`,
  },
  { title: "unit tests" }
);

/** The reported case: the agent stopped waiting before any output arrived. */
export const terminalStillRunningEmptyFixture = terminalToolMessage("tool_terminal_waiting", {
  terminal_id: "term-77c1d0e94ab3",
  command_id: "cmd-03",
  exit_code: null,
  duration_ms: 0,
  still_running: true,
  output: "",
  signal: null,
  truncated: false,
  untrusted: true,
});

export const terminalSignaledFixture = terminalToolMessage(
  "tool_terminal_signaled",
  {
    terminal_id: "term-1b2c3d4e5f60",
    command_id: "cmd-04",
    exit_code: null,
    signal: "TERM",
    duration_ms: 63_000,
    still_running: false,
    output: "watching for changes…",
  },
  { title: "dev server" }
);

import { Suspense, lazy, use } from "react";

import { OsShellContext } from "@/systems/os";
import { useSessionTerminalScope } from "../../hooks/use-session-terminal-scope";
import type { UIMessage } from "../../types";
import { DetailPre } from "./detail-pre";

/**
 * The emulator arrives only when a terminal block is actually on screen.
 *
 * A session that never runs a terminal command must not download the emulator,
 * so the block sits behind a lazy boundary and the plain output stands in until
 * it lands.
 */
const SessionTerminalBlock = lazy(async () => {
  const { SessionTerminalBlock: Block } = await import("@/systems/terminal");
  return { default: Block };
});

interface TerminalToolFacts {
  terminalId: string;
  title: string;
  preview: string;
  exitCode: number | null;
  signal: string | null;
  stillRunning: boolean;
}

/**
 * Reads the tool call's own record.
 *
 * The terminal fields ride the raw tool envelope rather than the normalized
 * `ToolUseResult`, so they are read from there and narrowed one field at a time.
 * Nothing is inferred: a field the runtime did not send stays absent.
 */
function readTerminalFacts(message: UIMessage): TerminalToolFacts | null {
  const result = message.toolResult;
  const envelope = asRecord(result?.rawOutput);
  const raw = asRecord(
    envelope.structured ?? envelope.raw_output ?? envelope.rawOutput ?? result?.rawOutput
  );
  const terminalId = readString(raw.terminal_id);
  if (!terminalId) return null;
  const command = readString(asRecord(message.toolInput).command) ?? message.toolName ?? "terminal";
  const exitCode = typeof raw.exit_code === "number" ? raw.exit_code : null;
  const signal = readString(raw.signal);
  return {
    terminalId,
    title: command,
    preview: readString(raw.output) ?? result?.stdout ?? result?.content ?? "",
    exitCode,
    signal,
    stillRunning:
      raw.still_running === true ||
      (message.toolName === "compozy__terminal_open" && exitCode === null && signal === null),
  };
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
}

function readString(value: unknown): string | null {
  return typeof value === "string" && value.trim() !== "" ? value : null;
}

/**
 * A terminal tool call, rendered as the terminal it ran in.
 *
 * Deliberate terminal use is a window into a live surface, not a second
 * transcript — so the block shows the screen and points at the terminal, and the
 * outcome is stated exactly as the runtime reported it.
 */
export function TerminalContent({ message }: { message: UIMessage }) {
  const shell = use(OsShellContext);
  const facts = readTerminalFacts(message);
  // The terminal's scope is not in the tool result — exec and open return a
  // `terminal_id` and nothing about where it lives. It comes from the session
  // this block is inside: its workspace, and *its* profile, read from the
  // session itself rather than from whichever profile the shell happens to be
  // showing. A conversation opened from a link, or read under the all-profiles
  // view, still belongs to the profile that started it, and a same-named
  // terminal in another profile is a different terminal.
  const scope = useSessionTerminalScope();
  if (!facts) {
    // A pipe exec finishes without a terminal object at all; that is a plain
    // command result and the generic pre keeps it readable rather than dressing
    // it as a window that never existed.
    const envelope = asRecord(message.toolResult?.rawOutput);
    const raw = asRecord(
      envelope.structured ??
        envelope.raw_output ??
        envelope.rawOutput ??
        message.toolResult?.rawOutput
    );
    const output =
      readString(raw.output) ?? message.toolResult?.stdout ?? message.toolResult?.content ?? "";
    return output ? <DetailPre data-testid="terminal-content-plain">{output}</DetailPre> : null;
  }
  const exit = terminalExitFor(facts);
  return (
    <div className="flex min-w-0 flex-col gap-1" data-testid="terminal-content">
      <Suspense
        fallback={<DetailPre data-testid="terminal-content-fallback">{facts.preview}</DetailPre>}
      >
        <SessionTerminalBlock
          blockId={message.id}
          exit={exit}
          preview={facts.preview}
          stillRunning={facts.stillRunning}
          terminalId={facts.terminalId}
          title={facts.title}
          {...(shell
            ? {
                onOpenTerminal: () => {
                  void shell.coordinator.userOpen({
                    app: "terminal",
                    instanceKey: facts.terminalId,
                    route: {
                      pathname: `/terminal/${encodeURIComponent(facts.terminalId)}`,
                      search: {},
                    },
                  });
                },
              }
            : {})}
          {...(scope ? { scope } : {})}
        />
      </Suspense>
    </div>
  );
}

/** The runtime's own outcome. A cause it could not verify stays unknown. */
function terminalExitFor(facts: TerminalToolFacts) {
  if (facts.stillRunning) return null;
  if (facts.signal) {
    return { cause: "signaled" as const, signal: facts.signal, at: "" };
  }
  if (facts.exitCode === null) {
    return { cause: "unknown" as const, at: "" };
  }
  return { cause: "exited" as const, code: facts.exitCode, at: "" };
}

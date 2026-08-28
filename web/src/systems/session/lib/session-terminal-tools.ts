import type { TerminalExit, TerminalInfo, TerminalSignal } from "@/systems/terminal/parts";

/** Native tools that open or run a supervised terminal — not agent-reported output. */
const DELIBERATE_TERMINAL_TOOLS = new Set(["compozy__terminal_exec", "compozy__terminal_open"]);

export function isDeliberateTerminalTool(toolName: string | undefined): boolean {
  return toolName !== undefined && DELIBERATE_TERMINAL_TOOLS.has(toolName);
}

export function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
}

/** The supervised terminal this tool call named, when the runtime created one. */
export function readSupervisedTerminalId(rawOutput: unknown): string | null {
  const envelope = asRecord(rawOutput);
  const raw = asRecord(
    envelope.structured ?? envelope.raw_output ?? envelope.rawOutput ?? rawOutput
  );
  return readNonEmptyString(raw.terminal_id);
}

export function readNonEmptyString(value: unknown): string | null {
  return typeof value === "string" && value.trim() !== "" ? value : null;
}

export function readTerminalEnvelope(rawOutput: unknown): Record<string, unknown> {
  const envelope = asRecord(rawOutput);
  return asRecord(envelope.structured ?? envelope.raw_output ?? envelope.rawOutput ?? rawOutput);
}

export interface SessionTerminalEnvelopeRun {
  stillRunning: boolean;
  exitCode: number | null;
  signal: TerminalSignal | null;
}

/**
 * Live catalog exit wins a frozen ACP envelope.
 *
 * `still_running` and an open-without-exit record the agent's wait, not the
 * terminal's life. Once the catalog row is `exited` or carries `exit`, the
 * block must drop live attach and show the finished foot.
 */
export function resolveSessionTerminalRun(
  envelope: SessionTerminalEnvelopeRun,
  catalog: Pick<TerminalInfo, "state" | "exit"> | undefined
): { stillRunning: boolean; exit: TerminalExit | null } {
  if (catalog !== undefined && (catalog.state === "exited" || catalog.exit != null)) {
    return {
      stillRunning: false,
      exit: catalog.exit ?? envelopeExit(envelope, ""),
    };
  }
  if (envelope.stillRunning) {
    return { stillRunning: true, exit: null };
  }
  return { stillRunning: false, exit: envelopeExit(envelope, catalog?.exit?.at ?? "") };
}

function envelopeExit(envelope: SessionTerminalEnvelopeRun, at: string): TerminalExit {
  if (envelope.signal) {
    return { cause: "signaled", signal: envelope.signal, at };
  }
  if (envelope.exitCode === null) {
    return { cause: "unknown", at };
  }
  return { cause: "exited", code: envelope.exitCode, at };
}

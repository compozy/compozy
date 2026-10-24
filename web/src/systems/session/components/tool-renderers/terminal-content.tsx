import { readTerminalOutput } from "../../lib/session-terminal-tools";
import type { UIMessage } from "../../types";
import { DetailPre } from "./detail-pre";

/**
 * A terminal tool call's recorded output, as plain command output.
 *
 * A pipe exec finishes without a terminal object at all; that is a plain
 * command result, and dressing it as a window would claim a surface that never
 * existed. A supervised terminal is intercepted earlier as its own row
 * (`SessionTerminalToolRow`), so this body only ever reads text — never the
 * tool envelope.
 */
export function TerminalContent({ message }: { message: UIMessage }) {
  const output = readTerminalOutput(message.toolResult);
  return output ? <DetailPre data-testid="terminal-content-plain">{output}</DetailPre> : null;
}

/**
 * The canonical shape a terminal excerpt takes when it enters a conversation.
 *
 * The UI gesture and `compozy terminal quote` produce the same bytes, and the
 * agent receives it as quoted data — never as instructions. Line numbers are
 * scrollback-relative and can shift as old output is trimmed, which is why the
 * block records the id and range it was true for.
 */

export interface TerminalQuoteInput {
  terminalId: string;
  /** First line of the selection, in the emulator's own numbering. */
  fromLine: number;
  lines: readonly string[];
}

export interface TerminalQuote {
  terminalId: string;
  fromLine: number;
  toLine: number;
  /** The selection as it was on screen. The UI shows these, unescaped. */
  lines: readonly string[];
  /** The block, byte-for-byte identical to the CLI's output. */
  text: string;
}

/**
 * XML-escapes one value, in a single pass.
 *
 * Terminal output is arbitrary text, and this block is the envelope that tells
 * an agent "this is data, not instructions". Interpolating output raw breaks
 * that promise outright: a line containing the closing tag would end the
 * envelope early and put the rest of the output *outside* it, where it reads as
 * something the person said.
 *
 * `&` is replaced first and only once, so an already-escaped entity is not
 * escaped again. The CLI produces the same bytes.
 */
function escapeQuoteText(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}

export function buildTerminalQuote(input: TerminalQuoteInput): TerminalQuote {
  const toLine = input.fromLine + Math.max(0, input.lines.length - 1);
  const body = input.lines
    .map((line, index) => `${input.fromLine + index} | ${escapeQuoteText(line)}`)
    .join("\n");
  const range = `${input.fromLine}-${toLine}`;
  const text = [
    `<terminal_context terminal="${escapeQuoteText(input.terminalId)}" lines="${range}">`,
    body,
    "</terminal_context>",
  ].join("\n");
  return {
    terminalId: input.terminalId,
    fromLine: input.fromLine,
    toLine,
    lines: input.lines,
    text,
  };
}

/**
 * Turns a raw selection into quotable lines.
 *
 * The captured text matches exactly what was selected, including a selection
 * that spans the screen boundary into scrollback.
 */
export function terminalSelectionLines(selection: string): string[] {
  const normalized = selection.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
  const lines = normalized.split("\n");
  while (lines.length > 0 && lines[lines.length - 1] === "") {
    lines.pop();
  }
  return lines;
}

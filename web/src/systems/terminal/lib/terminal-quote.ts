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
 * `&` is replaced first in this single pass. Existing entity-looking text is
 * still literal terminal output, so its ampersand is escaped too. The CLI
 * produces the same bytes.
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

/**
 * Builds the canonical quote from a grid selection.
 *
 * `startLine` is the emulator's 1-based inclusive origin — the same number
 * `compozy terminal quote --lines` and a pipe log's `firstLineNumber` use.
 */
export function terminalQuoteFromSelection(
  terminalId: string,
  selection: { startLine: number; text: string }
): TerminalQuote {
  return buildTerminalQuote({
    terminalId,
    fromLine: selection.startLine,
    lines: terminalSelectionLines(selection.text),
  });
}

/** The sourced block a clipboard write must use — never the raw selection. */
export function sourcedTerminalQuoteText(
  terminalId: string,
  selection: { startLine: number; text: string }
): string {
  return terminalQuoteFromSelection(terminalId, selection).text;
}

export async function copySourcedTerminalQuote(
  terminalId: string,
  selection: { startLine: number; text: string }
): Promise<void> {
  await navigator.clipboard.writeText(sourcedTerminalQuoteText(terminalId, selection));
}

const ENVELOPE =
  /^<terminal_context terminal="([^"]+)" lines="(\d+)-(\d+)">\n([\s\S]*)\n<\/terminal_context>$/;

/**
 * Recovers a quote from the canonical envelope this module (and the CLI) emit.
 *
 * Used at the session boundary when a first message *is* that envelope, so the
 * chip can carry it instead of showing the XML as a draft.
 */
export function parseTerminalQuote(text: string): TerminalQuote | null {
  const match = ENVELOPE.exec(text);
  if (!match) return null;
  const terminalId = unescapeQuoteText(match[1] ?? "");
  const fromLine = Number.parseInt(match[2] ?? "", 10);
  const toLine = Number.parseInt(match[3] ?? "", 10);
  if (!Number.isSafeInteger(fromLine) || !Number.isSafeInteger(toLine) || terminalId === "") {
    return null;
  }
  const lines: string[] = [];
  for (const raw of (match[4] ?? "").split("\n")) {
    const lineMatch = /^(\d+) \| (.*)$/.exec(raw);
    if (!lineMatch) return null;
    lines.push(unescapeQuoteText(lineMatch[2] ?? ""));
  }
  if (lines.length === 0) return null;
  const quote = buildTerminalQuote({ terminalId, fromLine, lines });
  if (quote.toLine !== toLine || quote.text !== text) return null;
  return quote;
}

function unescapeQuoteText(value: string): string {
  return value
    .replaceAll("&apos;", "'")
    .replaceAll("&quot;", '"')
    .replaceAll("&gt;", ">")
    .replaceAll("&lt;", "<")
    .replaceAll("&amp;", "&");
}

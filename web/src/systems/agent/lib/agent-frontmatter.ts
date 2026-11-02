/**
 * The small YAML subset SOUL.md and HEARTBEAT.md use: mappings, lists and plain or quoted
 * scalars. There's no YAML library in `web`, and the forms only need this much. Anything
 * outside it reads as `null`, so a file the subset can't hold stays in Advanced, untouched.
 */
export type FrontmatterValue =
  | string
  | number
  | boolean
  | null
  | readonly FrontmatterValue[]
  | { readonly [key: string]: FrontmatterValue };

export type FrontmatterMap = { readonly [key: string]: FrontmatterValue };

interface Line {
  indent: number;
  text: string;
}

class Unsupported extends Error {}

function parseFlowList(text: string): FrontmatterValue[] {
  const inner = text.slice(1, -1).trim();
  if (inner === "") return [];
  return (inner.match(/(?:"(?:[^"\\]|\\.)*"|'[^']*'|[^,])+/g) ?? []).map(item =>
    parseScalar(item.trim())
  );
}

function parseScalar(text: string): FrontmatterValue {
  if (text === "" || text === "null" || text === "~") return null;
  if (text === "true") return true;
  if (text === "false") return false;
  if (text.startsWith("[") && text.endsWith("]")) return parseFlowList(text);
  if (text.startsWith('"')) {
    try {
      return JSON.parse(text) as string;
    } catch {
      throw new Unsupported();
    }
  }
  if (text.startsWith("'")) {
    if (!text.endsWith("'") || text.length < 2) throw new Unsupported();
    return text.slice(1, -1).replaceAll("''", "'");
  }
  if (/^[[{|>&*!%@`]/.test(text) || / #/.test(text)) throw new Unsupported();
  if (/^-?\d+(\.\d+)?$/.test(text)) return Number(text);
  return text;
}

const KEY = /^([A-Za-z_][\w-]*):(?:\s+(.*))?$/;

function isListItem(text: string) {
  return text === "-" || text.startsWith("- ");
}

function parseBlock(lines: Line[], start: number, indent: number) {
  return isListItem(lines[start]?.text ?? "")
    ? parseList(lines, start, indent)
    : parseMap(lines, start, indent);
}

function parseMap(lines: Line[], start: number, indent: number) {
  const map: Record<string, FrontmatterValue> = {};
  let index = start;
  while (index < lines.length) {
    const line = lines[index]!;
    if (line.indent < indent) break;
    const match = line.indent === indent ? KEY.exec(line.text) : null;
    if (!match) throw new Unsupported();
    const key = match[1]!;
    if (key in map) throw new Unsupported();
    index += 1;
    if (match[2] !== undefined && match[2] !== "") {
      map[key] = parseScalar(match[2]);
      continue;
    }
    const next = lines[index];
    const nested =
      next !== undefined &&
      (next.indent > indent || (next.indent === indent && isListItem(next.text)));
    if (!nested) {
      map[key] = null;
      continue;
    }
    const block = parseBlock(lines, index, next.indent);
    map[key] = block.value;
    index = block.next;
  }
  return { value: map, next: index };
}

function parseList(lines: Line[], start: number, indent: number) {
  const list: FrontmatterValue[] = [];
  let index = start;
  while (index < lines.length) {
    const line = lines[index]!;
    if (line.indent !== indent || !isListItem(line.text)) break;
    const text = line.text.slice(1).trim();
    if (KEY.test(text)) {
      // `- key: value` opens a mapping whose keys line up with the first one.
      const inner = line.indent + (line.text.length - text.length);
      const rest = [...lines];
      rest[index] = { indent: inner, text };
      const block = parseMap(rest, index, inner);
      list.push(block.value);
      index = block.next;
      continue;
    }
    list.push(parseScalar(text));
    index += 1;
  }
  return { value: list, next: index };
}

function readLines(metadata: string): Line[] {
  const lines: Line[] = [];
  for (const raw of metadata.split("\n")) {
    const text = raw.trim();
    if (text === "") continue;
    // A comment would be dropped by the next save, so the form can't take this file.
    if (text.startsWith("#") || raw.includes("\t")) throw new Unsupported();
    lines.push({ indent: raw.length - raw.trimStart().length, text });
  }
  return lines;
}

/** The metadata mapping, or `null` when it leaves the supported subset. */
export function parseFrontmatter(metadata: string): FrontmatterMap | null {
  try {
    const lines = readLines(metadata);
    if (lines.length === 0) return {};
    const { value, next } = parseMap(lines, 0, lines[0]!.indent);
    return next === lines.length ? value : null;
  } catch (error) {
    if (error instanceof Unsupported) return null;
    throw error;
  }
}

export interface FrontmatterFile {
  /** `null` when the file has no frontmatter block. */
  metadata: string | null;
  body: string;
}

/** Splits a file on its `---` fences; an unterminated block reads as `null`. */
export function splitFrontmatter(source: string): FrontmatterFile | null {
  const text = source.replaceAll("\r\n", "\n");
  if (!text.startsWith("---\n")) return { metadata: null, body: text };
  const end = text.indexOf("\n---", 3);
  if (end < 0) return null;
  const after = text.slice(end + 4);
  if (after !== "" && !after.startsWith("\n")) return null;
  return { metadata: text.slice(4, end), body: after.replace(/^\n+/, "") };
}

/** Why a file opens in Advanced only. */
export type FileBlock =
  | "table"
  | "heading"
  | "quote"
  | "code"
  | "rule"
  | "html"
  | "settings"
  | "mistake";

const BODY_BLOCKS: readonly (readonly [Exclude<FileBlock, "settings" | "mistake">, RegExp])[] = [
  ["code", /^\s*(```|~~~)/m],
  ["table", /^.*\|.*\n\s*\|?\s*:?-{2,}[\s:|-]*$/m],
  ["heading", /^#{1,6}\s/m],
  ["quote", /^\s*>/m],
  ["rule", /^\s*([-*_])(\s*\1){2,}\s*$/m],
  ["html", /^\s*<[a-zA-Z!/][^>]*>/m],
];

/** What `MarkdownEditor` holds: prose marks, lists and links. Anything else it would flatten. */
export function markdownBlock(body: string): FileBlock | null {
  return BODY_BLOCKS.find(([, pattern]) => pattern.test(body))?.[0] ?? null;
}

const BLOCK_NAME: Record<FileBlock, string> = {
  table: "a table",
  heading: "a heading",
  quote: "a quote",
  code: "a code block",
  rule: "a divider",
  html: "some HTML",
  settings: "settings",
  mistake: "a mistake",
};

export function fileBlockCopy(block: FileBlock) {
  if (block === "mistake") return "This file has a mistake, so it opens here. Fix it to save.";
  return `This file has ${BLOCK_NAME[block]} the editor can't show, so it opens here. Nothing gets lost.`;
}

export interface FileDiagnostic {
  message: string;
  line?: number;
  severity?: string;
}

/** The first blocking diagnostic, with the server's file and field jargon taken out. */
export function fileProblem(diagnostics: readonly FileDiagnostic[] | null | undefined) {
  const first = diagnostics?.find(item => item.severity !== "warning" && item.severity !== "info");
  if (!first) return undefined;
  const text = first.message
    .replace(/(SOUL|HEARTBEAT)\.md( frontmatter)?( field)?/g, "This file")
    .replace(/^./, letter => letter.toUpperCase());
  return first.line ? `Line ${first.line}: ${text}` : text;
}

function lineChanges(from: string, to: string) {
  const before = new Set(from.split("\n").map(line => line.trim()));
  const after = new Set(to.split("\n").map(line => line.trim()));
  const changes: string[] = [];
  for (const line of after) if (line && !before.has(line)) changes.push(`Adds "${line}"`);
  for (const line of before) if (line && !after.has(line)) changes.push(`Removes "${line}"`);
  return changes;
}

/** Up to `limit` plain lines saying what one version changes against the other. */
export function describeChanges(from: string, to: string, limit = 2) {
  const lines = lineChanges(from, to);
  const shown = lines.slice(0, limit);
  return lines.length > limit ? [...shown, `And ${lines.length - limit} more`] : shown;
}

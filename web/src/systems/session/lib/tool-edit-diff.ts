import type { UIMessage } from "../types";

const TRUNCATE_THRESHOLD = 1500;

export interface EditToolInput {
  filePath: string;
  oldStr: string;
  newStr: string;
}

export interface EditDiffView {
  removed: string[];
  added: string[];
  /** More content exists than the collapsed view renders. */
  truncated: boolean;
}

function optionalText(value: unknown): string {
  return value != null ? String(value) : "";
}

/** Reads the edited path and the replaced/replacement text from an Edit tool call. */
export function readEditToolInput(message: UIMessage): EditToolInput {
  const input = message.toolInput;
  return {
    filePath: String(input?.file_path ?? input?.filePath ?? message.toolResult?.filePath ?? ""),
    oldStr: optionalText(input?.old_string),
    newStr: optionalText(input?.new_string),
  };
}

function diffLines(source: string, marker: "-" | "+"): string[] {
  if (source.length === 0) return [];
  return source.split("\n").map(line => `${marker} ${line}`);
}

function visibleText(source: string, showFull: boolean): string {
  if (showFull || source.length <= TRUNCATE_THRESHOLD) return source;
  return `${source.slice(0, TRUNCATE_THRESHOLD)}…`;
}

/**
 * Builds the unified-diff lines for an edit: `- ` deletions and `+ ` additions,
 * each side capped at the threshold (with an ellipsis) until `showFull`.
 */
export function editDiffView(oldStr: string, newStr: string, showFull: boolean): EditDiffView {
  const visibleOld = visibleText(oldStr, showFull);
  const visibleNew = visibleText(newStr, showFull);
  return {
    removed: diffLines(visibleOld, "-"),
    added: diffLines(visibleNew, "+"),
    truncated: visibleOld !== oldStr || visibleNew !== newStr,
  };
}

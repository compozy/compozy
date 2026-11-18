/** Number of lines `text` renders as; a trailing newline does not add a line. */
export function countLines(text: string): number {
  if (text === "") return 1;
  let lines = 1;
  for (let index = 0; index < text.length; index += 1) {
    if (text.charCodeAt(index) === 10) lines += 1;
  }
  return text.endsWith("\n") ? lines - 1 : lines;
}

export interface ResolvedTargetLine {
  /** Line to scroll to and highlight, clamped into the file. */
  line: number;
  /** The requested line was past the end; `line` is the last line. */
  pastEnd: boolean;
  requested: number;
}

export function resolveTargetLine(
  targetLine: number | null | undefined,
  lineCount: number
): ResolvedTargetLine | null {
  if (targetLine == null || !Number.isFinite(targetLine)) return null;
  const requested = Math.max(1, Math.floor(targetLine));
  if (requested > lineCount) return { line: lineCount, pastEnd: true, requested };
  return { line: requested, pastEnd: false, requested };
}

/** Cheap content fingerprint (FNV-1a) so a refreshed file bumps its item version. */
export function contentVersion(text: string): number {
  let hash = 0x811c9dc5;
  for (let index = 0; index < text.length; index += 1) {
    hash ^= text.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}

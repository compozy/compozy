/*
 * Text matching that agrees with the daemon's search. The daemon lower-cases
 * query and content with Go's `strings.ToLower`, which maps each code point on
 * its own (simple case mapping, no context). JavaScript's `toLowerCase` works
 * on the whole string: it expands U+0130 (İ) into two code points and picks a
 * final sigma by context, so offsets into a lower-cased copy do not line up
 * with the original text. Fold per code point instead and keep, for every
 * folded UTF-16 unit, where it came from — ranges are then cut in the original.
 */

export interface FoldedText {
  folded: string;
  /** Original UTF-16 offset where folded unit `i` starts. */
  starts: number[];
  /** Original UTF-16 offset (exclusive) where the code point behind folded unit `i` ends. */
  ends: number[];
}

export interface TextSpan {
  start: number;
  end: number;
}

/** One code point lower-cased the way the daemon does it: one code point in, one out. */
export function foldCodePoint(codePoint: number): string {
  const lower = String.fromCodePoint(codePoint).toLowerCase();
  const first = lower.codePointAt(0);
  if (first === undefined) return "";
  const single = String.fromCodePoint(first);
  // U+0130 lower-cases to "i" + U+0307 in JavaScript; the daemon keeps "i" alone.
  return lower.length === single.length ? lower : single;
}

export function caseFoldForSearch(text: string): FoldedText {
  let folded = "";
  const starts: number[] = [];
  const ends: number[] = [];
  for (let index = 0; index < text.length;) {
    const codePoint = text.codePointAt(index)!;
    const width = codePoint > 0xffff ? 2 : 1;
    const unit = foldCodePoint(codePoint);
    for (let offset = 0; offset < unit.length; offset += 1) {
      starts.push(index);
      ends.push(index + width);
    }
    folded += unit;
    index += width;
  }
  return { ends, folded, starts };
}

/** Every occurrence of `needle` in `text` under the daemon's folding, as original UTF-16 spans. */
export function findFoldedOccurrences(text: string, needle: string): TextSpan[] {
  const needleFolded = caseFoldForSearch(needle).folded;
  if (needleFolded.length === 0 || text.length === 0) return [];
  const { folded, starts, ends } = caseFoldForSearch(text);
  const spans: TextSpan[] = [];
  let at = folded.indexOf(needleFolded);
  while (at >= 0) {
    spans.push({ end: ends[at + needleFolded.length - 1]!, start: starts[at]! });
    at = folded.indexOf(needleFolded, at + needleFolded.length);
  }
  return spans;
}

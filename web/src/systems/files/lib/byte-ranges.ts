/** A run of text and whether it is inside a highlighted range. */
export interface TextSegment {
  text: string;
  match: boolean;
}

/** A half-open UTF-16 range `[start, end)` into a JavaScript string. */
export type CharRange = readonly [number, number];

/**
 * The daemon reports match ranges as half-open UTF-8 byte offsets (`_dx.md`); strings here are
 * UTF-16. Converts and clamps them into `text`, dropping empty ranges. An offset that lands
 * inside a multi-byte character rounds down to that character's start.
 */
export function byteRangesToCharRanges(
  text: string,
  ranges: readonly (readonly number[])[]
): CharRange[] {
  if (ranges.length === 0) return [];
  // charAt[byte] for every character boundary; the final entry is the text's byte length.
  const boundaries: number[] = [];
  const indexes: number[] = [];
  let bytes = 0;
  let index = 0;
  for (const char of text) {
    boundaries.push(bytes);
    indexes.push(index);
    const code = char.codePointAt(0) ?? 0;
    bytes += code < 0x80 ? 1 : code < 0x800 ? 2 : code < 0x10000 ? 3 : 4;
    index += char.length;
  }
  boundaries.push(bytes);
  indexes.push(index);

  const toChar = (byte: number): number => {
    if (byte <= 0) return 0;
    if (byte >= bytes) return text.length;
    let low = 0;
    let high = boundaries.length - 1;
    while (low < high) {
      const middle = Math.ceil((low + high) / 2);
      if ((boundaries[middle] ?? 0) <= byte) low = middle;
      else high = middle - 1;
    }
    return indexes[low] ?? 0;
  };

  const result: CharRange[] = [];
  for (const range of ranges) {
    const [startByte = 0, endByte = 0] = range;
    const start = toChar(startByte);
    const end = toChar(endByte);
    if (end > start) result.push([start, end]);
  }
  return result.sort((a, b) => a[0] - b[0]);
}

/** Splits `text` into plain and highlighted runs; overlapping ranges merge. */
export function highlightSegments(text: string, ranges: readonly CharRange[]): TextSegment[] {
  const segments: TextSegment[] = [];
  let cursor = 0;
  for (const [rawStart, rawEnd] of ranges) {
    const start = Math.max(cursor, Math.min(rawStart, text.length));
    const end = Math.min(rawEnd, text.length);
    if (end <= start) continue;
    if (start > cursor) segments.push({ text: text.slice(cursor, start), match: false });
    segments.push({ text: text.slice(start, end), match: true });
    cursor = end;
  }
  if (cursor < text.length) segments.push({ text: text.slice(cursor), match: false });
  return segments;
}

import type { FileSearchMatch } from "../adapters/files-api";
import { byteRangesToCharRanges, type CharRange } from "./byte-ranges";

/** The daemon cuts excerpts to this many characters around the first match (`_dx.md`). */
export const SEARCH_EXCERPT_CHARS = 300;

export interface SearchExcerpt {
  text: string;
  ranges: CharRange[];
  /** The excerpt starts after the line does. */
  leading: boolean;
  /** The excerpt reached the cap, so the line likely continues. */
  trailing: boolean;
}

/**
 * A match row's excerpt: byte ranges converted and clamped into the excerpt (US-014.EC-8),
 * leading indentation dropped, and ellipses kept outside the text so ranges never shift.
 */
export function searchExcerpt(match: FileSearchMatch): SearchExcerpt {
  const ranges = byteRangesToCharRanges(match.text, match.ranges);
  const indent = match.text.length - match.text.trimStart().length;
  const firstMatch = ranges[0]?.[0] ?? indent;
  const cut = Math.min(indent, firstMatch);
  return {
    text: match.text.slice(cut),
    ranges: ranges.map(([start, end]): CharRange => [start - cut, end - cut]),
    leading: match.text_offset > 0,
    trailing: [...match.text].length >= SEARCH_EXCERPT_CHARS,
  };
}

/** The text of a match's first range, for seeding Find in file from a regex search. */
export function firstMatchedText(match: FileSearchMatch): string {
  const [range] = byteRangesToCharRanges(match.text, match.ranges);
  return range ? match.text.slice(range[0], range[1]) : "";
}

export interface NormalizedText {
  readonly text: string;
  readonly tokens: readonly string[];
  readonly compact: string;
}

const COMBINING_MARKS = /\p{M}+/gu;
const SEPARATORS = /[^\p{L}\p{N}]+/gu;

export function normalizeRankingText(value: string): NormalizedText {
  const text = value
    .normalize("NFKD")
    .replace(COMBINING_MARKS, "")
    .toLocaleLowerCase()
    .replace(SEPARATORS, " ")
    .trim()
    .replace(/\s+/gu, " ");
  const tokens = text === "" ? [] : text.split(" ");
  return { text, tokens, compact: tokens.join("") };
}

export interface SubsequenceMatch {
  readonly density: number;
  readonly boundaryMatches: number;
}

export function subsequenceMatch(needle: string, haystack: string): SubsequenceMatch | null {
  if (needle === "" || haystack === "") return null;
  let needleIndex = 0;
  let firstIndex = -1;
  let lastIndex = -1;
  let boundaryMatches = 0;
  for (let index = 0; index < haystack.length && needleIndex < needle.length; index += 1) {
    if (haystack[index] !== needle[needleIndex]) continue;
    if (firstIndex === -1) firstIndex = index;
    lastIndex = index;
    if (index === 0 || haystack[index - 1] === " ") boundaryMatches += 1;
    needleIndex += 1;
  }
  if (needleIndex !== needle.length || firstIndex === -1) return null;
  return {
    density: needle.length / (lastIndex - firstIndex + 1),
    boundaryMatches,
  };
}

// Stable identities for fixed text the manual renders piece by piece (code samples, help output):
// a line's number and a run's character offset name its position, so React keys come from the
// text itself rather than from loop counters.

/** Splits fixed text into lines numbered from 1. */
export function numberLines(text: string): { n: number; text: string }[] {
  return text.split("\n").map((line, index) => ({ n: index + 1, text: line }));
}

/** Tags each [text, ...] run with the offset where it starts inside the joined text. */
export function withOffsets<Run extends readonly [text: string, ...rest: unknown[]]>(
  runs: readonly Run[]
): { at: number; run: Run }[] {
  let at = 0;
  return runs.map(run => {
    const start = at;
    at += run[0].length;
    return { at: start, run };
  });
}

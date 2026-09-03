const DURATION_UNITS_MS: Readonly<Record<string, number>> = {
  ns: 1 / 1_000_000,
  us: 1 / 1_000,
  µs: 1 / 1_000,
  μs: 1 / 1_000,
  ms: 1,
  s: 1_000,
  m: 60_000,
  h: 3_600_000,
};

const DURATION_SEGMENT = /(?:\d+(?:\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h)/gy;

/** Parses the positive Go duration syntax accepted by terminal settings. */
export function parsePositiveDurationMilliseconds(value: string | undefined): number | undefined {
  if (value === undefined) return undefined;
  let offset = 0;
  let total = 0;
  DURATION_SEGMENT.lastIndex = 0;
  for (const match of value.matchAll(DURATION_SEGMENT)) {
    if (match.index !== offset) return undefined;
    offset += match[0].length;
    total += Number.parseFloat(match[0]) * DURATION_UNITS_MS[match[1]];
  }
  return offset === value.length && total > 0 ? total : undefined;
}

export function validatePositiveDuration(value: string): string | null {
  return parsePositiveDurationMilliseconds(value) === undefined
    ? "Enter a positive duration such as 15m or 24h."
    : null;
}

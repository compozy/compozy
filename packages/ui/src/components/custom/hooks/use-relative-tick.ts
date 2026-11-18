import * as React from "react";

interface ClockReading {
  key: string;
  now: number;
}

function readClock(key: string): ClockReading {
  return { key, now: Date.now() };
}

/**
 * Returns the clock reading of the latest tick. Callers pass it to their formatters so the
 * React Compiler sees the changing input; a bare re-render would reuse the memoized output.
 * A new `key` (the formatted timestamp) reads the clock again, so a value that arrives between
 * ticks is not measured against an old reading.
 */
export function useRelativeTick(refreshMs: number, key: string): number {
  const [reading, setReading] = React.useState(() => readClock(key));
  // Adjusting state during render: React discards this render and re-renders at once.
  if (reading.key !== key) setReading(readClock(key));
  React.useEffect(() => {
    const id = setInterval(() => {
      setReading(current => readClock(current.key));
    }, refreshMs);
    return () => clearInterval(id);
  }, [refreshMs]);
  return reading.now;
}

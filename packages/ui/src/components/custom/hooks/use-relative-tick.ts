import * as React from "react";

/**
 * Returns the clock reading of the latest tick. Callers pass it to their formatters so the
 * React Compiler sees the changing input; a bare re-render would reuse the memoized output.
 */
export function useRelativeTick(active: boolean, refreshMs: number): number {
  const [now, setNow] = React.useState(Date.now);
  React.useEffect(() => {
    if (!active) return undefined;
    const id = setInterval(() => {
      setNow(Date.now());
    }, refreshMs);
    return () => clearInterval(id);
  }, [active, refreshMs]);
  return now;
}

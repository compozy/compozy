import { useEffect, useState } from "react";

/**
 * One shared 1s clock while the run ticks; parked and terminal spans stay
 * timestamp-frozen. Returns the current epoch ms, re-rendering once per second
 * only while `enabled`.
 */
export function useNowTick(enabled: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!enabled) return undefined;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [enabled]);
  return now;
}

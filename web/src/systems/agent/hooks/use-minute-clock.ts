import { useEffect, useState } from "react";

const MINUTE = 60_000;

/** The real time of day, refreshed each minute; `initial` pins it for stories and tests. */
export function useMinuteClock(initial?: Date) {
  const [now, setNow] = useState(() => initial ?? new Date());
  useEffect(() => {
    if (initial) return undefined;
    const timer = setInterval(() => setNow(new Date()), MINUTE);
    return () => clearInterval(timer);
  }, [initial]);
  return now;
}

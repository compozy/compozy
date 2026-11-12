import { useEffect, useState } from "react";

import type { BootWork } from "../scenes";

/** Counts whole seconds from `start`, so every scene opens mid-flight and keeps moving. */
export function useElapsed(start: number | undefined) {
  const [seconds, setSeconds] = useState(start ?? 0);
  useEffect(() => {
    if (start === undefined) return;
    const timer = setInterval(() => setSeconds(value => value + 1), 1000);
    return () => clearInterval(timer);
  }, [start]);
  return start === undefined ? null : seconds;
}

/** Advances the fixture at the runtime's reported rate, as the real stream would. */
export function useWork(work: BootWork | undefined) {
  const [completed, setCompleted] = useState(work?.completed ?? 0);
  const rate = work?.rate;
  const total = work?.total ?? 0;
  const moving = work?.moving !== false;
  useEffect(() => {
    if (!rate || !moving) return;
    const timer = setInterval(
      () => setCompleted(value => Math.min(total, value + 1)),
      Math.max(120, 60000 / rate)
    );
    return () => clearInterval(timer);
  }, [rate, total, moving]);
  return work ? { ...work, completed } : undefined;
}

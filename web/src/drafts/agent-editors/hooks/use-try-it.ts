import { useEffect, useRef, useState } from "react";

import type { TryPhase } from "../checkins-model";

/** Try it now: a short wait standing in for the real check, then the result. */
export function useTryIt(initial: TryPhase) {
  const [phase, setPhase] = useState(initial);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  const run = (next: TryPhase) => {
    setPhase("running");
    timer.current = setTimeout(() => setPhase(next), 1400);
  };
  return { phase, run, reset: () => setPhase("idle") };
}

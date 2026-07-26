import * as React from "react";

export function useRelativeTick(active: boolean, refreshMs: number): void {
  const [, setTick] = React.useState(0);
  React.useEffect(() => {
    if (!active) return undefined;
    const id = setInterval(() => {
      setTick(n => n + 1);
    }, refreshMs);
    return () => clearInterval(id);
  }, [active, refreshMs]);
}

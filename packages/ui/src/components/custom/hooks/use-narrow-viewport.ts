import * as React from "react";

export function useNarrowViewport(breakpoint: number): boolean {
  const [narrow, setNarrow] = React.useState(false);
  React.useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
    const query = window.matchMedia(`(max-width: ${Math.max(0, breakpoint - 1)}px)`);
    const handler = (event: MediaQueryListEvent | MediaQueryList) => {
      setNarrow(event.matches);
    };
    handler(query);
    query.addEventListener("change", handler);
    return () => query.removeEventListener("change", handler);
  }, [breakpoint]);
  return narrow;
}

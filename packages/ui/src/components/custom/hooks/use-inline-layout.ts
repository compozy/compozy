import * as React from "react";

export function useInlineLayout(breakpoint: number): boolean {
  const [inline, setInline] = React.useState<boolean>(() => {
    if (typeof window === "undefined") return true;
    return window.matchMedia(`(min-width: ${breakpoint}px)`).matches;
  });
  React.useEffect(() => {
    if (typeof window === "undefined") return undefined;
    const query = window.matchMedia(`(min-width: ${breakpoint}px)`);
    const handler = (event: MediaQueryListEvent) => {
      setInline(event.matches);
    };
    setInline(query.matches);
    query.addEventListener("change", handler);
    return () => {
      query.removeEventListener("change", handler);
    };
  }, [breakpoint]);
  return inline;
}

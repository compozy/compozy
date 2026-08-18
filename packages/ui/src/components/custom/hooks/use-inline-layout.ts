import * as React from "react";

export function useInlineLayout(breakpoint: number): boolean {
  const query = `(min-width: ${breakpoint}px)`;
  const subscribe = (onStoreChange: () => void) => {
    if (typeof window === "undefined") return () => {};
    const mediaQuery = window.matchMedia(query);
    mediaQuery.addEventListener("change", onStoreChange);
    return () => {
      mediaQuery.removeEventListener("change", onStoreChange);
    };
  };
  const getSnapshot = () => {
    if (typeof window === "undefined") return true;
    return window.matchMedia(query).matches;
  };

  return React.useSyncExternalStore(subscribe, getSnapshot, getServerInlineSnapshot);
}

function getServerInlineSnapshot(): boolean {
  return true;
}

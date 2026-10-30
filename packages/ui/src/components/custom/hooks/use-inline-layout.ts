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

/**
 * The same decision against an element's own width: a pane inside a window follows the
 * window, not the screen. `null` until the element is measured, so a caller never commits
 * to one layout and then remounts its content into the other.
 */
export function useContainerInlineLayout(
  element: HTMLElement | null,
  breakpoint: number
): boolean | null {
  const subscribe = (onStoreChange: () => void) => {
    if (!element || typeof ResizeObserver === "undefined") return () => {};
    const observer = new ResizeObserver(onStoreChange);
    observer.observe(element);
    return () => observer.disconnect();
  };
  const getSnapshot = () => {
    if (!element) return null;
    // A host without layout (no ResizeObserver, detached) reads as wide.
    if (typeof ResizeObserver === "undefined") return true;
    return element.clientWidth >= breakpoint;
  };
  return React.useSyncExternalStore(subscribe, getSnapshot, getServerContainerSnapshot);
}

function getServerContainerSnapshot(): null {
  return null;
}

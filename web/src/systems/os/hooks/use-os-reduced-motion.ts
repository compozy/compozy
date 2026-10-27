import { useLayoutEffect, useSyncExternalStore } from "react";

import { getSystemReducedMotion, subscribeSystemReducedMotion } from "../lib/reduced-motion";
import { useDesktop } from "./use-desktop";

/**
 * Motion collapse signal: the system `prefers-reduced-motion` preference OR
 * the desktop-doc in-product toggle (`desktop.reduceMotion`, agent-writable).
 */
export function useOsReducedMotion(): boolean {
  const docPreference = useDesktop(state => state.reduceMotion);
  const systemPreference = useSyncExternalStore(
    subscribeSystemReducedMotion,
    getSystemReducedMotion,
    () => false
  );
  return docPreference || systemPreference;
}

/** The desktop shell publishes its effective preference to document-level CSS animations. */
export function useOsDocumentReducedMotion(): boolean {
  const reduced = useOsReducedMotion();
  useLayoutEffect(() => {
    const root = document.documentElement;
    const previous = root.getAttribute("data-reduce-motion");
    root.setAttribute("data-reduce-motion", String(reduced));
    return () => {
      if (previous === null) root.removeAttribute("data-reduce-motion");
      else root.setAttribute("data-reduce-motion", previous);
    };
  }, [reduced]);
  return reduced;
}

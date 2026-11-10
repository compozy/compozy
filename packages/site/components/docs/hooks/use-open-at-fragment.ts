"use client";

import { useEffect, useRef } from "react";

import { fragmentId } from "@/lib/url-fragment";

/**
 * Opens a closed `hiddenUntilFound` panel when the page was loaded with a fragment to an id inside
 * it. After hydration the browser opens such a panel itself on find-in-page and on a fragment
 * navigation, but the server can only send plain `hidden` (React renders no `until-found`), so the
 * fragment of the first load finds nothing. Put the returned ref on an element around the panel.
 */
export function useOpenAtFragment<T extends HTMLElement>(
  open: boolean,
  setOpen: (open: boolean) => void
) {
  const ref = useRef<T>(null);
  const target = useRef<HTMLElement | null>(null);
  useEffect(() => {
    const id = fragmentId(window.location.hash);
    const element = id ? document.getElementById(id) : null;
    if (!element || !ref.current?.contains(element)) return;
    target.current = element;
    setOpen(true);
  }, [setOpen]);
  // Scrolls once the panel is open, so the target has its box.
  useEffect(() => {
    if (!open || !target.current) return;
    target.current.scrollIntoView();
    target.current = null;
  }, [open]);
  return ref;
}

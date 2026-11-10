"use client";

import { useEffect, useRef } from "react";

/**
 * Opens a `<details>` when `open` turns true, and never closes it. The element stays uncontrolled:
 * a browser may open it for a URL fragment before React hydrates, so React does not render `open`.
 */
export function useOpenDetails(open: boolean) {
  const ref = useRef<HTMLDetailsElement>(null);
  useEffect(() => {
    if (open && ref.current) ref.current.open = true;
  }, [open]);
  return ref;
}

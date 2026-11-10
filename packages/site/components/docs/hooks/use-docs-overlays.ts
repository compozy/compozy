"use client";

import { useEffect, useRef, useState } from "react";

import type { DocsChromeValue } from "@/components/docs/docs-chrome-context";

/** The drawer and palette state for one pathname, with the ⌘K and Escape shortcuts. */
export function useDocsOverlays(pathname: string): DocsChromeValue {
  const [overlay, setOverlay] = useState<{ kind: "menu" | "search" | null; path: string }>({
    kind: null,
    path: pathname,
  });
  // An overlay belongs to the page it was opened on: navigating closes it for good, so coming
  // back to that page (browser Back) does not find it open again.
  if (overlay.path !== pathname && overlay.kind !== null) {
    setOverlay({ kind: null, path: pathname });
  }
  const kind = overlay.path === pathname ? overlay.kind : null;
  const current = useRef(pathname);
  const searchOpener = useRef<HTMLElement | null>(null);
  const showSearch = (path: string) => {
    const active = document.activeElement;
    // Pressing the shortcut again inside the open palette keeps the original opener.
    if (!(active instanceof HTMLElement) || !active.closest('[role="dialog"]')) {
      searchOpener.current =
        active instanceof HTMLElement && active !== document.body ? active : null;
    }
    setOverlay({ kind: "search", path });
  };
  useEffect(() => {
    current.current = pathname;
  }, [pathname]);
  useEffect(() => {
    const handle = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        showSearch(current.current);
      } else if (event.key === "Escape") {
        setOverlay(state => ({ ...state, kind: null }));
      }
    };
    window.addEventListener("keydown", handle);
    return () => window.removeEventListener("keydown", handle);
  }, []);
  return {
    menu: kind === "menu",
    search: kind === "search",
    openMenu: () => setOverlay({ kind: "menu", path: pathname }),
    openSearch: () => showSearch(pathname),
    close: () => setOverlay({ kind: null, path: pathname }),
    searchOpener,
  };
}

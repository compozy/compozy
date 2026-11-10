"use client";

/** The docs overlays' shared state: the mobile drawer and the search palette, with ⌘K and Escape. */
import { cn } from "@compozy/ui";
import { XIcon } from "@compozy/ui/icons";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { DocsChromeContext, type DocsChromeValue } from "@/components/docs/docs-chrome-context";
import { FOCUS, LINE } from "@/components/docs/docs-classes";
import { useDocsChrome } from "@/components/docs/hooks/use-docs-chrome";

export function DocsChromeProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [overlay, setOverlay] = useState<{ kind: "menu" | "search" | null; path: string }>({
    kind: null,
    path: pathname,
  });
  // An overlay belongs to the page it was opened on: navigating closes it.
  const kind = overlay.path === pathname ? overlay.kind : null;
  const current = useRef(pathname);
  useEffect(() => {
    current.current = pathname;
  }, [pathname]);
  useEffect(() => {
    const handle = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setOverlay({ kind: "search", path: current.current });
      } else if (event.key === "Escape") {
        setOverlay(state => ({ ...state, kind: null }));
      }
    };
    window.addEventListener("keydown", handle);
    return () => window.removeEventListener("keydown", handle);
  }, []);
  const value: DocsChromeValue = {
    menu: kind === "menu",
    search: kind === "search",
    openMenu: () => setOverlay({ kind: "menu", path: pathname }),
    openSearch: () => setOverlay({ kind: "search", path: pathname }),
    close: () => setOverlay({ kind: null, path: pathname }),
  };
  return <DocsChromeContext value={value}>{children}</DocsChromeContext>;
}

/** The sidebar as a drawer below `lg`, opened from the header button. */
export function DocsDrawer({ children }: { children: ReactNode }) {
  const { menu, close } = useDocsChrome();
  if (!menu) return null;
  return (
    <div className="fixed inset-0 z-50 lg:hidden">
      <button
        type="button"
        aria-label="Close navigation"
        onClick={close}
        className="absolute inset-0 cursor-default bg-site-dark-700 backdrop-blur-sm"
      />
      <dialog
        open
        aria-modal="true"
        aria-label="Docs navigation"
        className={cn(
          "docs-rail absolute inset-y-0 right-auto left-0 m-0 h-full max-h-none w-[min(340px,88vw)] overflow-y-auto border-y-0 border-r border-l-0 bg-[#0c0c0c] px-4 pt-4 pb-10 text-fg",
          LINE
        )}
      >
        <button
          type="button"
          aria-label="Close navigation"
          onClick={close}
          className={cn(
            "mb-4 ml-auto flex size-9 cursor-pointer items-center justify-center rounded-[10px] border border-site-light-120 bg-transparent text-fg/80",
            FOCUS
          )}
        >
          <XIcon className="size-4" aria-hidden />
        </button>
        {children}
      </dialog>
    </div>
  );
}

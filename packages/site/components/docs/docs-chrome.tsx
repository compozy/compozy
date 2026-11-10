"use client";

/** The docs overlays' shared state: the mobile drawer and the search palette, with ⌘K and Escape. */
import { Sheet, SheetClose, SheetContent, SheetTitle, cn } from "@compozy/ui";
import { XIcon } from "@compozy/ui/icons";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

import { DocsChromeContext } from "@/components/docs/docs-chrome-context";
import { FOCUS, LINE } from "@/components/docs/docs-classes";
import { useDocsChrome } from "@/components/docs/hooks/use-docs-chrome";
import { useDocsOverlays } from "@/components/docs/hooks/use-docs-overlays";

export function DocsChromeProvider({ children }: { children: ReactNode }) {
  const value = useDocsOverlays(usePathname());
  return <DocsChromeContext value={value}>{children}</DocsChromeContext>;
}

/** The sidebar as a drawer below `lg`, opened from the header button. */
export function DocsDrawer({ children }: { children: ReactNode }) {
  const { menu, close } = useDocsChrome();
  return (
    <Sheet open={menu} onOpenChange={open => !open && close()}>
      <SheetContent
        side="left"
        showCloseButton={false}
        className={cn(
          "docs-rail w-[min(340px,88vw)] gap-0 overflow-y-auto rounded-none border-r bg-[#0c0c0c] px-4 pt-4 pb-10 sm:max-w-none lg:hidden",
          LINE
        )}
      >
        <SheetTitle className="sr-only">Docs navigation</SheetTitle>
        <SheetClose
          aria-label="Close navigation"
          className={cn(
            "mb-4 ml-auto flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-[10px] border border-site-light-120 bg-transparent text-fg/80",
            FOCUS
          )}
        >
          <XIcon className="size-4" aria-hidden />
        </SheetClose>
        {children}
      </SheetContent>
    </Sheet>
  );
}

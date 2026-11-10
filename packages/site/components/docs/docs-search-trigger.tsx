"use client";

/** The docs home's large search field: it opens the same palette as ⌘K. */
import { Kbd, cn } from "@compozy/ui";
import { Search } from "@compozy/ui/icons";

import { FOCUS } from "@/components/docs/docs-classes";
import { useDocsChrome } from "@/components/docs/hooks/use-docs-chrome";

export function DocsHomeSearch() {
  const { openSearch } = useDocsChrome();
  return (
    <button
      type="button"
      onClick={openSearch}
      className={cn(
        "flex h-[52px] w-full max-w-[420px] cursor-pointer items-center gap-3 rounded-pill border border-site-light-150 bg-site-light-050 pr-4 pl-5 text-left text-[15.5px] text-muted-foreground transition-colors hover:border-site-light-250 hover:text-fg",
        FOCUS
      )}
    >
      <Search className="size-[18px] shrink-0" aria-hidden />
      <span className="flex-1">Search the docs</span>
      <Kbd>⌘K</Kbd>
    </button>
  );
}

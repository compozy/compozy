"use client";

/** The docs chrome: brand and search on the first row, the seven sections as tabs on the second. */
import { Kbd, Logo, cn } from "@compozy/ui";
import { PanelLeft, Search, Star } from "@compozy/ui/icons";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { BEAM_X, FOCUS, LINE } from "@/components/docs/docs-classes";
import { useDocsChrome } from "@/components/docs/hooks/use-docs-chrome";
import { STAR_FORMAT, useGithubStarCount } from "@/components/site/hooks/use-github-star-count";
import { currentSectionId } from "@/lib/docs-navigation";
import type { DocsTab } from "@/lib/docs-shell";
import { siteConfig } from "@/lib/site-config";
import { MARKETING_NAV_LINKS } from "@/lib/site-nav";

const SITE_LINKS = MARKETING_NAV_LINKS.filter(link => link.href !== "/docs");
const ICON_BUTTON = cn(
  "inline-flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-[10px] border bg-transparent text-fg/80 transition-colors hover:text-fg",
  "border-site-light-120",
  FOCUS
);

function SearchTrigger({ onSearch }: { onSearch: () => void }) {
  return (
    <button
      type="button"
      onClick={onSearch}
      className={cn(
        "ml-1 hidden h-9 w-full max-w-[360px] cursor-pointer items-center gap-2.5 rounded-[11px] border bg-site-light-040 px-3 text-left text-[14px] text-muted-foreground transition-colors hover:border-site-light-200 hover:text-fg md:ml-5 md:flex",
        "border-site-light-100",
        FOCUS
      )}
    >
      <Search className="size-4 shrink-0" aria-hidden />
      <span className="flex-1 truncate">Search the docs</span>
      <Kbd>⌘K</Kbd>
    </button>
  );
}

function SectionTabs({ tabs, current }: { tabs: readonly DocsTab[]; current: string | null }) {
  return (
    <nav
      aria-label="Sections"
      className="docs-tabs mx-auto flex h-12 max-w-[1600px] items-stretch gap-1 overflow-x-auto px-3 md:px-6"
    >
      {tabs.map(tab => (
        <Link
          key={tab.id}
          href={tab.href}
          aria-current={current === tab.id ? "page" : undefined}
          className={cn(
            "relative inline-flex shrink-0 cursor-pointer items-center gap-2 rounded-[8px] bg-transparent px-3 text-[14px] transition-colors",
            current === tab.id ? "text-fg" : "text-muted-foreground hover:text-fg",
            FOCUS
          )}
        >
          {tab.label}
          <span className="font-mono text-[11.5px] text-subtle tabular-nums">{tab.count}</span>
          {current === tab.id ? (
            <span
              aria-hidden
              className={cn("absolute inset-x-3 bottom-0 h-0.5 rounded-full", BEAM_X)}
            />
          ) : null}
        </Link>
      ))}
    </nav>
  );
}

function StarLink() {
  const stars = useGithubStarCount();
  return (
    <a
      href={siteConfig.repoUrl}
      target="_blank"
      rel="noopener noreferrer"
      className={cn(
        "ml-auto hidden h-9 shrink-0 items-center gap-2 rounded-[11px] border border-site-light-150 bg-site-light-050 px-4 text-[14px] text-fg transition-colors hover:bg-site-light-100 md:inline-flex xl:ml-4",
        FOCUS
      )}
    >
      <Star className="size-3.5" aria-hidden />
      <span className="ml-0.5">Star on GitHub</span>
      {stars === null ? null : (
        <span className="ml-2 border-l border-site-light-150 pl-3 font-mono text-[12.5px] text-muted-foreground tabular-nums">
          {STAR_FORMAT.format(stars)}
        </span>
      )}
    </a>
  );
}

export function DocsHeader({ tabs }: { tabs: readonly DocsTab[] }) {
  const pathname = usePathname();
  const { openMenu, openSearch } = useDocsChrome();
  return (
    <header
      className={cn("fixed inset-x-0 top-0 z-40 border-b bg-[#0a0a0a]/90 backdrop-blur-xl", LINE)}
    >
      <div className="mx-auto flex h-14 max-w-[1600px] items-center gap-4 px-4 md:gap-5 md:px-8">
        <button
          type="button"
          aria-label="Open navigation"
          onClick={openMenu}
          className={cn(ICON_BUTTON, "lg:hidden")}
        >
          <PanelLeft className="size-4" aria-hidden />
        </button>
        <Link
          href="/"
          aria-label="CompozyOS home"
          className={cn("flex shrink-0 rounded-[8px]", FOCUS)}
        >
          <Logo variant="logo" decorative className="h-[22px]" />
        </Link>
        <SearchTrigger onSearch={openSearch} />
        <nav aria-label="Site" className="ml-auto hidden items-center gap-9 self-stretch xl:flex">
          <Link
            href="/docs"
            aria-current="page"
            className={cn(
              "relative flex cursor-pointer items-center self-stretch rounded-[6px] bg-transparent text-[14px] text-fg",
              FOCUS
            )}
          >
            Docs
            <span
              aria-hidden
              className={cn("absolute inset-x-0 -bottom-px h-0.5 rounded-full", BEAM_X)}
            />
          </Link>
          {SITE_LINKS.map(link => (
            <Link
              key={link.href}
              href={link.href}
              className={cn(
                "rounded-[6px] text-[14px] text-fg/70 transition-colors hover:text-fg",
                FOCUS
              )}
            >
              {link.label}
            </Link>
          ))}
        </nav>
        <button
          type="button"
          aria-label="Search the docs"
          onClick={openSearch}
          className={cn(ICON_BUTTON, "ml-auto md:hidden")}
        >
          <Search className="size-4" aria-hidden />
        </button>
        <StarLink />
      </div>
      <div className={cn("border-t", LINE)}>
        <SectionTabs tabs={tabs} current={currentSectionId(tabs, pathname)} />
      </div>
    </header>
  );
}

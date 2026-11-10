/** The docs chrome: brand and search on the first row, the seven sections as tabs on the second. */
import { Kbd, Logo, cn } from "@compozy/ui";
import { PanelLeft, Search, Star } from "@compozy/ui/icons";

import { LIVE } from "../landing/content";
import { FOCUS, BEAM_X, LINE } from "./text";
import { GROUPS, firstSlug } from "./nav";

const SITE_LINKS = ["Marketplace", "Blog", "Changelog", "Download"];
/** A snapshot; production reads the live count, as the current site does. */
const STARS = new Intl.NumberFormat("en-US").format(LIVE.stars);
const ICON_BUTTON = cn(
  "inline-flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-[10px] border bg-transparent text-fg/80 transition-colors hover:text-fg",
  "border-[color:var(--light-120)]",
  FOCUS
);

function SearchTrigger({ onSearch }: { onSearch: () => void }) {
  return (
    <button
      type="button"
      onClick={onSearch}
      className={cn(
        "ml-1 hidden h-9 w-full max-w-[360px] md:ml-5 cursor-pointer items-center gap-2.5 rounded-[11px] border bg-[color:var(--light-040)] px-3 text-left text-[14px] text-muted-foreground transition-colors hover:border-[color:var(--light-200)] hover:text-fg md:flex",
        "border-[color:var(--light-100)]",
        FOCUS
      )}
    >
      <Search className="size-4 shrink-0" aria-hidden />
      <span className="flex-1 truncate">Search the docs</span>
      <Kbd>⌘K</Kbd>
    </button>
  );
}

function SectionTabs({
  groupId,
  onOpen,
}: {
  groupId: string | null;
  onOpen: (slug: string) => void;
}) {
  return (
    <nav
      aria-label="Sections"
      className="docs-tabs mx-auto flex h-12 max-w-[1600px] items-stretch gap-1 overflow-x-auto px-3 md:px-6"
    >
      {GROUPS.map(group => (
        <button
          key={group.id}
          type="button"
          aria-current={groupId === group.id ? "page" : undefined}
          onClick={() => onOpen(firstSlug(group))}
          className={cn(
            "relative inline-flex shrink-0 cursor-pointer items-center gap-2 rounded-[8px] bg-transparent px-3 text-[14px] transition-colors",
            groupId === group.id ? "text-fg" : "text-muted-foreground hover:text-fg",
            FOCUS
          )}
        >
          {group.label}
          <span className="font-mono text-[11.5px] text-subtle tabular-nums">{group.count}</span>
          {groupId === group.id ? (
            <span
              aria-hidden
              className={cn("absolute inset-x-3 bottom-0 h-0.5 rounded-full", BEAM_X)}
            />
          ) : null}
        </button>
      ))}
    </nav>
  );
}

export function DocsHeader(props: {
  groupId: string | null;
  onOpen: (slug: string) => void;
  onSearch: () => void;
  onMenu: () => void;
}) {
  return (
    <header
      className={cn("fixed inset-x-0 top-0 z-40 border-b bg-[#0a0a0a]/90 backdrop-blur-xl", LINE)}
    >
      <div className="mx-auto flex h-14 max-w-[1600px] items-center gap-4 px-4 md:gap-5 md:px-8">
        <button
          type="button"
          aria-label="Open navigation"
          onClick={props.onMenu}
          className={cn(ICON_BUTTON, "lg:hidden")}
        >
          <PanelLeft className="size-4" aria-hidden />
        </button>
        <a
          href="#"
          aria-label="CompozyOS home"
          className={cn("flex shrink-0 rounded-[8px]", FOCUS)}
        >
          <Logo variant="logo" className="h-[22px]" />
        </a>
        <SearchTrigger onSearch={props.onSearch} />
        <nav aria-label="Site" className="ml-auto hidden items-center gap-9 self-stretch xl:flex">
          <button
            type="button"
            aria-current="page"
            onClick={() => props.onOpen("")}
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
          </button>
          {SITE_LINKS.map(link => (
            <a
              key={link}
              href="#"
              className="text-[14px] text-fg/70 transition-colors hover:text-fg"
            >
              {link}
            </a>
          ))}
        </nav>
        <button
          type="button"
          aria-label="Search the docs"
          onClick={props.onSearch}
          className={cn(ICON_BUTTON, "ml-auto md:hidden")}
        >
          <Search className="size-4" aria-hidden />
        </button>
        <a
          href="#"
          className="ml-auto hidden h-9 shrink-0 items-center gap-2 rounded-[11px] border border-[color:var(--light-150)] bg-[color:var(--light-050)] px-4 text-[14px] text-fg transition-colors hover:bg-[color:var(--light-100)] md:inline-flex xl:ml-4"
        >
          <Star className="size-3.5" aria-hidden />
          <span className="ml-0.5">Star on GitHub</span>
          <span className="ml-2 border-l border-[color:var(--light-150)] pl-3 font-mono text-[12.5px] text-muted-foreground tabular-nums">
            {STARS}
          </span>
        </a>
      </div>
      <div className={cn("border-t", LINE)}>
        <SectionTabs groupId={props.groupId} onOpen={props.onOpen} />
      </div>
    </header>
  );
}

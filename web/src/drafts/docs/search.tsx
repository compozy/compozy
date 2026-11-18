/** Search across all pages: grouped by section, filterable, keyboard-driven. */
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  Kbd,
  cn,
} from "@compozy/ui";
import { CornerDownRight } from "@compozy/ui/icons";
import { useState } from "react";

import { DocsGlyph } from "./glyph";
import { GROUPS, PAGES, TOTAL_PAGES, searchPages, sectionOf, type DocPage } from "./nav";
import { FOCUS, LINE } from "./text";

const LIMIT = 8;
const SUGGESTED = PAGES.filter(page => page.group?.id === "start-here").slice(0, 6);
const CHIP = cn(
  "inline-flex h-7 shrink-0 cursor-pointer items-center gap-1.5 rounded-pill border px-3 text-[13px] transition-colors",
  FOCUS
);

function pathOf(page: DocPage) {
  return [page.group?.label, ...page.trail.map(node => node.t)].filter(Boolean).join(" / ");
}

function ResultRow({ page, onOpen }: { page: DocPage; onOpen: (slug: string) => void }) {
  return (
    <CommandItem
      value={page.slug || "docs-home"}
      onSelect={() => onOpen(page.slug)}
      className="items-start gap-3 rounded-[10px] px-3 py-2.5"
    >
      <DocsGlyph name={sectionOf(page)?.i} className="mt-0.5 size-4 shrink-0 text-subtle" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[14.5px] text-fg">{page.title}</span>
        <span className="block truncate text-[13px] text-muted-foreground">
          {page.description || pathOf(page)}
        </span>
      </span>
      <span className="mt-0.5 hidden max-w-[200px] shrink-0 truncate font-mono text-[11.5px] text-subtle sm:block">
        {pathOf(page)}
      </span>
    </CommandItem>
  );
}

function Results({
  query,
  groupId,
  onOpen,
}: {
  query: string;
  groupId: string | null;
  onOpen: (slug: string) => void;
}) {
  const hits = searchPages(query, groupId);
  if (!query.trim()) {
    return (
      <CommandGroup heading="Start here">
        {SUGGESTED.map(page => (
          <ResultRow key={page.slug} page={page} onOpen={onOpen} />
        ))}
      </CommandGroup>
    );
  }
  const sections = [];
  for (const group of GROUPS) {
    const matched = hits.filter(hit => hit.page.group?.id === group.id);
    if (matched.length > 0) sections.push({ group, hits: matched });
  }
  return (
    <>
      <CommandEmpty className="px-4 py-10 text-center text-[14px] text-muted-foreground">
        No page matches “{query.trim()}”.
      </CommandEmpty>
      {sections.map(section => (
        <CommandGroup
          key={section.group.id}
          heading={`${section.group.label} · ${section.hits.length}`}
        >
          {section.hits.slice(0, LIMIT).map(hit => (
            <ResultRow key={hit.page.slug} page={hit.page} onOpen={onOpen} />
          ))}
        </CommandGroup>
      ))}
    </>
  );
}

export function DocsSearch(props: {
  query: string;
  onQuery: (query: string) => void;
  onClose: () => void;
  onOpen: (slug: string) => void;
}) {
  const [groupId, setGroupId] = useState<string | null>(null);
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center px-4 pt-[12vh]">
      <button
        type="button"
        aria-label="Close search"
        onClick={props.onClose}
        className="absolute inset-0 cursor-default bg-[color:var(--dark-700)] backdrop-blur-sm"
      />
      <dialog
        open
        aria-label="Search the docs"
        className="relative m-0 w-full max-w-[680px] overflow-hidden rounded-[18px] border border-[color:var(--light-150)] bg-[#111111] p-0 text-fg shadow-[0_40px_120px_rgba(0,0,0,0.6)]"
      >
        <Command shouldFilter={false} className="bg-transparent">
          <CommandInput
            autoFocus
            variant="quiet"
            value={props.query}
            onValueChange={props.onQuery}
            placeholder={`Search ${TOTAL_PAGES} pages`}
            className="h-12 text-[16px]"
          />
          <div
            role="group"
            aria-label="Section"
            className={cn("docs-tabs flex gap-1.5 overflow-x-auto border-y px-3 py-2.5", LINE)}
          >
            <button
              type="button"
              aria-pressed={groupId === null}
              onClick={() => setGroupId(null)}
              className={cn(
                CHIP,
                groupId === null
                  ? "border-[color:var(--light-350)] bg-[color:var(--light-100)] text-fg"
                  : "border-[color:var(--light-100)] bg-transparent text-muted-foreground hover:text-fg"
              )}
            >
              All
            </button>
            {GROUPS.map(group => (
              <button
                key={group.id}
                type="button"
                aria-pressed={groupId === group.id}
                onClick={() => setGroupId(group.id)}
                className={cn(
                  CHIP,
                  groupId === group.id
                    ? "border-[color:var(--light-350)] bg-[color:var(--light-100)] text-fg"
                    : "border-[color:var(--light-100)] bg-transparent text-muted-foreground hover:text-fg"
                )}
              >
                {group.label}
              </button>
            ))}
          </div>
          <CommandList className="max-h-[min(440px,52vh)] p-2">
            <Results query={props.query} groupId={groupId} onOpen={props.onOpen} />
          </CommandList>
          <div
            className={cn(
              "flex items-center gap-5 border-t px-4 py-2.5 text-[12.5px] text-subtle",
              LINE
            )}
          >
            <span className="inline-flex items-center gap-1.5">
              <Kbd>↑</Kbd>
              <Kbd>↓</Kbd> Navigate
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Kbd>
                <CornerDownRight className="size-3" aria-hidden />
              </Kbd>
              Open
            </span>
            <span className="ml-auto inline-flex items-center gap-1.5">
              <Kbd>esc</Kbd> Close
            </span>
          </div>
        </Command>
      </dialog>
    </div>
  );
}

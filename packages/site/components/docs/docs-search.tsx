"use client";

/** Search across every docs page: grouped by section, filterable, keyboard-driven, on the site's index. */
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
import { useRouter } from "next/navigation";
import { useState, type ReactNode } from "react";

import { FOCUS, LINE } from "@/components/docs/docs-classes";
import { useDocsChrome } from "@/components/docs/hooks/use-docs-chrome";
import { useDocsSearch, type SearchPageHit } from "@/components/docs/hooks/use-docs-search";
import type { DocsTab } from "@/lib/docs-shell";

const LIMIT = 8;
const CHIP = cn(
  "inline-flex h-7 shrink-0 cursor-pointer items-center gap-1.5 rounded-pill border px-3 text-[13px] transition-colors",
  FOCUS
);
const CHIP_ON = "border-site-light-350 bg-site-light-100 text-fg";
const CHIP_OFF = "border-site-light-100 bg-transparent text-muted-foreground hover:text-fg";

export interface DocsSearchSuggestion {
  url: string;
  title: string;
  description: string;
}

interface DocsSearchProps {
  tabs: readonly DocsTab[];
  total: number;
  suggested: readonly DocsSearchSuggestion[];
  /** Top-level folder icons by path segment, as the sidebar draws them. */
  icons: Readonly<Record<string, ReactNode>>;
}

function headOf(url: string) {
  return url.replace(/^\/docs\/?/, "").split("/")[0] ?? "";
}

function ResultRow(props: {
  url: string;
  title: string;
  detail: string;
  path: string;
  icon: ReactNode;
  onOpen: (url: string) => void;
}) {
  return (
    <CommandItem
      value={props.url}
      onSelect={() => props.onOpen(props.url)}
      className="items-start gap-3 rounded-[10px] px-3 py-2.5 data-[selected=false]:bg-transparent data-[selected=true]:bg-site-light-060"
    >
      <span aria-hidden className="mt-0.5 flex size-4 shrink-0 text-subtle [&_svg]:size-4">
        {props.icon}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[14.5px] text-fg">{props.title}</span>
        <span className="block truncate text-[13px] text-muted-foreground">{props.detail}</span>
      </span>
      <span className="mt-0.5 hidden max-w-[200px] shrink-0 truncate font-mono text-[11.5px] text-subtle sm:block">
        {props.path}
      </span>
    </CommandItem>
  );
}

function pathOf(hit: SearchPageHit) {
  return hit.breadcrumbs.join(" / ");
}

function Results(
  props: DocsSearchProps & { query: string; tag: string | null; onOpen: (url: string) => void }
) {
  const { hits, loading } = useDocsSearch(props.query, props.tag);
  if (!props.query.trim()) {
    return (
      <CommandGroup heading="Start here">
        {props.suggested.map(page => (
          <ResultRow
            key={page.url}
            url={page.url}
            title={page.title}
            detail={page.description}
            path=""
            icon={props.icons[headOf(page.url)]}
            onOpen={props.onOpen}
          />
        ))}
      </CommandGroup>
    );
  }
  const sections = props.tabs.flatMap(tab => {
    const matched = hits.filter(hit => tab.heads.includes(headOf(hit.url)));
    return matched.length > 0 ? [{ tab, hits: matched }] : [];
  });
  return (
    <>
      {loading && hits.length === 0 ? null : (
        <CommandEmpty className="px-4 py-10 text-center text-[14px] text-muted-foreground">
          No page matches “{props.query.trim()}”.
        </CommandEmpty>
      )}
      {sections.map(section => (
        <CommandGroup
          key={section.tab.id}
          heading={`${section.tab.label} · ${section.hits.length}`}
        >
          {section.hits.slice(0, LIMIT).map(hit => (
            <ResultRow
              key={hit.url}
              url={hit.url}
              title={hit.title}
              detail={hit.excerpt ?? pathOf(hit)}
              path={pathOf(hit)}
              icon={props.icons[headOf(hit.url)]}
              onOpen={props.onOpen}
            />
          ))}
        </CommandGroup>
      ))}
    </>
  );
}

function Palette(props: DocsSearchProps & { onClose: () => void }) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [tag, setTag] = useState<DocsTab | null>(null);
  const open = (url: string) => {
    props.onClose();
    router.push(url);
  };
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center px-4 pt-[12vh]">
      <button
        type="button"
        aria-label="Close search"
        onClick={props.onClose}
        className="absolute inset-0 cursor-default bg-site-dark-700 backdrop-blur-sm"
      />
      <dialog
        open
        aria-modal="true"
        aria-label="Search the docs"
        className="relative m-0 w-full max-w-[680px] overflow-hidden rounded-[18px] border border-site-light-150 bg-[#111111] p-0 text-fg shadow-[0_40px_120px_var(--site-dark-600)]"
      >
        <Command shouldFilter={false} className="bg-transparent">
          <CommandInput
            autoFocus
            variant="quiet"
            value={query}
            onValueChange={setQuery}
            placeholder={`Search ${props.total} pages`}
            className="h-12 text-[16px]"
          />
          <div
            role="group"
            aria-label="Section"
            className={cn("docs-tabs flex gap-1.5 overflow-x-auto border-y px-3 py-2.5", LINE)}
          >
            <button
              type="button"
              aria-pressed={tag === null}
              onClick={() => setTag(null)}
              className={cn(CHIP, tag === null ? CHIP_ON : CHIP_OFF)}
            >
              All
            </button>
            {props.tabs.map(tab => (
              <button
                key={tab.id}
                type="button"
                aria-pressed={tag?.id === tab.id}
                onClick={() => setTag(tab)}
                className={cn(CHIP, tag?.id === tab.id ? CHIP_ON : CHIP_OFF)}
              >
                {tab.label}
              </button>
            ))}
          </div>
          <CommandList className="max-h-[min(440px,52vh)] p-2">
            <Results {...props} query={query} tag={tag?.label ?? null} onOpen={open} />
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

/** The palette, mounted only while open so every opening starts from an empty query. */
export function DocsSearch(props: DocsSearchProps) {
  const { search, close } = useDocsChrome();
  if (!search) return null;
  return <Palette {...props} onClose={close} />;
}

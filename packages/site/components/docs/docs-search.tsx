"use client";

/** Search across every docs page: grouped by section, filterable, keyboard-driven, on the site's index. */
import {
  Button,
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
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

interface Row {
  url: string;
  title: string;
  detail: string;
  path: string;
}

interface ResultGroup {
  key: string;
  heading: string;
  rows: Row[];
}

function rowOf(hit: SearchPageHit): Row {
  return { url: hit.url, title: hit.title, detail: hit.excerpt ?? pathOf(hit), path: pathOf(hit) };
}

/** The palette's groups, in display order: suggestions for an empty query, else hits by section. */
function resultGroups(
  props: DocsSearchProps,
  query: string,
  hits: readonly SearchPageHit[],
  tag: string | null
): ResultGroup[] {
  if (!query.trim()) {
    const rows = props.suggested.map(page => ({
      url: page.url,
      title: page.title,
      detail: page.description,
      path: "",
    }));
    return [{ key: "suggested", heading: "Start here", rows }];
  }
  // Pages that sit before the first section (the docs home, "How to use these docs") get their own group.
  const sectioned = new Set(props.tabs.flatMap(tab => tab.heads));
  const intro = tag === null ? hits.filter(hit => !sectioned.has(headOf(hit.url))) : [];
  const groups: ResultGroup[] = [];
  if (intro.length > 0)
    groups.push({
      key: "intro",
      heading: `Docs · ${intro.length}`,
      rows: intro.slice(0, LIMIT).map(rowOf),
    });
  for (const tab of props.tabs) {
    const matched = hits.filter(hit => tab.heads.includes(headOf(hit.url)));
    if (matched.length === 0) continue;
    groups.push({
      key: tab.id,
      heading: `${tab.label} · ${matched.length}`,
      rows: matched.slice(0, LIMIT).map(rowOf),
    });
  }
  return groups;
}

function Results(props: {
  groups: readonly ResultGroup[];
  query: string;
  loading: boolean;
  failed: boolean;
  retry: () => void;
  icons: DocsSearchProps["icons"];
  onOpen: (url: string) => void;
}) {
  if (props.failed && props.query.trim()) {
    return (
      <div role="status" className="flex flex-col items-center gap-4 px-4 py-10 text-center">
        <p className="m-0 text-[14px] text-muted-foreground">Search is unavailable right now.</p>
        <Button type="button" variant="outline" size="sm" onClick={props.retry}>
          Try again
        </Button>
      </div>
    );
  }
  return (
    <>
      {props.loading && props.groups.length === 0 ? null : (
        <CommandEmpty className="px-4 py-10 text-center text-[14px] text-muted-foreground">
          No page matches “{props.query.trim()}”.
        </CommandEmpty>
      )}
      {props.groups.map(group => (
        <CommandGroup key={group.key} heading={group.heading}>
          {group.rows.map(row => (
            <ResultRow
              key={row.url}
              {...row}
              icon={props.icons[headOf(row.url)]}
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
  const { hits, loading, failed, retry } = useDocsSearch(query, tag?.label ?? null);
  const groups = resultGroups(props, query, hits, tag?.label ?? null);
  const urls = groups.flatMap(group => group.rows.map(row => row.url));
  // The highlighted row. cmdk moves it only when the query text changes, so a section filter that
  // swaps the results would leave nothing for Enter to open: fall back to the first row shown.
  const [picked, setPicked] = useState("");
  const active = urls.includes(picked) ? picked : (urls[0] ?? "");
  const open = (url: string) => {
    props.onClose();
    router.push(url);
  };
  return (
    <Command
      shouldFilter={false}
      value={active}
      onValueChange={setPicked}
      className="bg-transparent"
    >
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
        <Results
          groups={groups}
          query={query}
          loading={loading}
          failed={failed}
          retry={retry}
          icons={props.icons}
          onOpen={open}
        />
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
  );
}

/** The palette. Its body mounts only while open, so every opening starts from an empty query. */
export function DocsSearch(props: DocsSearchProps) {
  const { search, close, searchOpener } = useDocsChrome();
  return (
    <Dialog open={search} onOpenChange={open => !open && close()}>
      <DialogContent
        unframed
        showCloseButton={false}
        finalFocus={searchOpener}
        className="site-docs top-[12vh] w-[calc(100%-2rem)] max-w-[680px] translate-y-0 overflow-hidden rounded-[18px] border border-site-light-150 bg-[#111111] p-0 shadow-[0_40px_120px_var(--site-dark-600)] sm:max-w-[680px]"
      >
        <DialogHeader className="sr-only">
          <DialogTitle>Search the docs</DialogTitle>
          <DialogDescription>Search {props.total} pages</DialogDescription>
        </DialogHeader>
        <Palette {...props} onClose={close} />
      </DialogContent>
    </Dialog>
  );
}

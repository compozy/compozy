"use client";

/**
 * The list: a line of every release, then the releases themselves, newest first. The channel and
 * the loaded pages live in the URL (`?betas=1`, `?page=2`) and the search is mirrored to `?q=`, so
 * coming back from a release page restores the view it left.
 */
import { Button, Empty, SearchInput, cn } from "@compozy/ui";
import { SearchX } from "@compozy/ui/icons";
import { useSearchParams } from "next/navigation";

import { useUrlBackedValue } from "@/components/site/hooks/use-url-backed-value";
import { CONTAINER } from "@/components/site/layout";
import { H1, Lead, SecondaryCta } from "@/components/site/primitives";
import {
  CHANGELOG_PAGE,
  groupEras,
  listState,
  matchesSearch,
  stableLine,
  type ListState,
  type TimelineRelease,
} from "@/lib/changelog/timeline";
import { ReleasePulse } from "./pulse";
import { ReleaseTimeline } from "./timeline";

/** Native `replaceState` keeps `useSearchParams` in sync without a navigation or a scroll reset. */
function writeState(state: ListState) {
  const url = new URL(window.location.href);
  const set = (name: string, value: string | null) =>
    value ? url.searchParams.set(name, value) : url.searchParams.delete(name);
  set("betas", state.betas ? "1" : null);
  set("q", state.query);
  set("page", state.pages > 1 ? String(state.pages) : null);
  window.history.replaceState(null, "", url);
}

/**
 * Controlled by the URL for the channel and the loaded pages; the search box stays local while
 * typing and the URL only carries it for the way back.
 */
export function ChangelogList({
  releases,
  state = { betas: false, query: "", pages: 1 },
}: {
  releases: readonly TimelineRelease[];
  /** The view the URL asks for. */
  state?: ListState;
}) {
  const { betas: showBetas, pages } = state;
  const [query, setQuery] = useUrlBackedValue(state.query);
  const update = (next: Partial<ListState>) =>
    writeState({ betas: showBetas, pages, query, ...next });
  const setShowBetas = (betas: boolean) => update({ betas });
  const shown = pages * CHANGELOG_PAGE;
  const needle = query.trim().toLowerCase();
  const channel = showBetas ? releases : releases.filter(release => !release.pre);
  const found = needle ? channel.filter(release => matchesSearch(release, needle)) : channel;
  const items =
    showBetas || needle
      ? groupEras(found.slice(0, shown)).map(era => ({ kind: "era" as const, era }))
      : stableLine(releases, shown);
  return (
    <div className="relative overflow-x-clip">
      <div className={cn(CONTAINER, "relative pt-32 pb-24 md:pt-40")}>
        <div className="flex flex-wrap items-end justify-between gap-6">
          <div>
            <div className="[&_h1]:md:text-[80px]">
              <H1>Changelog</H1>
            </div>
            <Lead className="mt-4 max-w-[560px] md:text-[19px]">
              What changed in CompozyOS, release by release.
            </Lead>
          </div>
          <SecondaryCta href="/changelog/feed.xml">RSS feed</SecondaryCta>
        </div>
        <div className="mt-12">
          <ReleasePulse releases={releases} showBetas={showBetas} />
        </div>
        <div className="mt-10 flex flex-wrap items-center justify-between gap-4">
          <div
            role="group"
            aria-label="Release channel"
            className="inline-flex gap-1 rounded-[12px] border border-site-light-100 p-1"
          >
            {([false, true] as const).map(value => (
              <button
                key={String(value)}
                type="button"
                aria-pressed={showBetas === value}
                onClick={() => setShowBetas(value)}
                className={cn(
                  "h-8 cursor-pointer rounded-[9px] bg-transparent px-3 text-[14px] outline-none transition-colors focus-visible:ring-2 focus-visible:ring-brand",
                  showBetas === value
                    ? "bg-site-light-100 text-fg"
                    : "text-muted-foreground hover:text-fg"
                )}
              >
                {value ? "Include betas" : "Stable"}
              </button>
            ))}
          </div>
          <SearchInput
            value={query}
            onChange={value => {
              setQuery(value);
              update({ query: value });
            }}
            placeholder="Search releases"
            aria-label="Search releases"
            containerClassName="h-10 w-full sm:w-[280px] [&_input]:text-[14px]"
          />
        </div>
        <div className="mt-8">
          {found.length === 0 ? (
            <Empty
              framed
              icon={SearchX}
              title={
                needle
                  ? `No release mentions “${query.trim()}”`
                  : "Releases from v0.3.0-beta.1 onward will appear here."
              }
              description={needle ? "Try a feature name or a version number." : undefined}
            />
          ) : (
            <ReleaseTimeline
              key={`${showBetas}-${needle}-${shown}`}
              items={items}
              onShowBetas={() => setShowBetas(true)}
            />
          )}
        </div>
        {found.length > shown ? (
          <div className="mt-6 flex justify-center">
            <Button variant="outline" onClick={() => update({ pages: pages + 1 })}>
              Older releases
            </Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}

/** Reads the view from the URL on every change; render it inside a Suspense boundary. */
export function ChangelogListFromUrl({ releases }: { releases: readonly TimelineRelease[] }) {
  return <ChangelogList releases={releases} state={listState(useSearchParams())} />;
}

"use client";

/** The list: a line of every release, then the releases themselves, newest first. */
import { Button, Empty, SearchInput, cn } from "@compozy/ui";
import { SearchX } from "@compozy/ui/icons";
import { useState } from "react";

import { CONTAINER } from "@/components/site/layout";
import { H1, Lead, SecondaryCta } from "@/components/site/primitives";
import { matchesSearch, type TimelineRelease } from "@/lib/changelog/timeline";
import { ReleasePulse } from "./pulse";
import { ReleaseTimeline, type BetaBranch } from "./timeline";

const PAGE = 12;

export function ChangelogList({
  releases,
  betas,
}: {
  releases: readonly TimelineRelease[];
  betas: BetaBranch;
}) {
  const [showBetas, setShowBetas] = useState(false);
  const [query, setQuery] = useState("");
  const [shown, setShown] = useState(PAGE);
  const needle = query.trim().toLowerCase();
  const channel = showBetas ? releases : releases.filter(release => !release.pre);
  const found = needle ? channel.filter(release => matchesSearch(release, needle)) : channel;
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
            onChange={setQuery}
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
              releases={found.slice(0, shown)}
              showBetas={showBetas || Boolean(needle)}
              betas={betas}
              onShowBetas={() => setShowBetas(true)}
            />
          )}
        </div>
        {found.length > shown ? (
          <div className="mt-6 flex justify-center">
            <Button variant="outline" onClick={() => setShown(value => value + PAGE)}>
              Older releases
            </Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}

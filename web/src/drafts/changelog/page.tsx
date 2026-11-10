/** The list: a line of every release, then the releases themselves, newest first. */
import { Button, Empty, SearchInput, cn } from "@compozy/ui";
import { CloudOff, SearchX } from "@compozy/ui/icons";
import { useState } from "react";

import { CONTAINER } from "../landing/layout";
import { H1, Lead, Muted, SecondaryCta } from "../landing/primitives";
import { ReleasePulse } from "./pulse";
import { ReleaseTimeline } from "./timeline";
import { REPO, RELEASES, matchesSearch } from "./releases";

const PAGE = 12;

export function ChangelogList({
  showBetas,
  query,
  onBetas,
  onQuery,
  onOpen,
}: {
  showBetas: boolean;
  query: string;
  onBetas: (value: boolean) => void;
  onQuery: (value: string) => void;
  onOpen: (version: string) => void;
}) {
  const [shown, setShown] = useState(PAGE);
  const needle = query.trim().toLowerCase();
  const channel = showBetas ? RELEASES : RELEASES.filter(release => !release.pre);
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
          <a href="/changelog/feed.xml">
            <SecondaryCta>RSS feed</SecondaryCta>
          </a>
        </div>
        <div className="mt-12">
          <ReleasePulse showBetas={showBetas} onOpen={onOpen} />
        </div>
        <div className="mt-10 flex flex-wrap items-center justify-between gap-4">
          <div
            role="group"
            aria-label="Release channel"
            className="inline-flex gap-1 rounded-[12px] border border-[color:var(--light-100)] p-1"
          >
            {([false, true] as const).map(value => (
              <button
                key={String(value)}
                type="button"
                aria-pressed={showBetas === value}
                onClick={() => onBetas(value)}
                className={cn(
                  "h-8 cursor-pointer rounded-[9px] bg-transparent px-3 text-[14px] outline-none transition-colors focus-visible:ring-2 focus-visible:ring-brand",
                  showBetas === value
                    ? "bg-[color:var(--light-100)] text-fg"
                    : "text-muted-foreground hover:text-fg"
                )}
              >
                {value ? "Include betas" : "Stable"}
              </button>
            ))}
          </div>
          <SearchInput
            value={query}
            onChange={onQuery}
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
              title={`No release mentions “${query.trim()}”`}
              description="Try a feature name or a version number."
            />
          ) : (
            <ReleaseTimeline
              key={`${showBetas}-${needle}-${shown}`}
              releases={found.slice(0, shown)}
              showBetas={showBetas || Boolean(needle)}
              onShowBetas={() => onBetas(true)}
              onOpen={onOpen}
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

export function ChangelogUnavailable() {
  return (
    <div className={cn(CONTAINER, "pt-32 pb-28 md:pt-40")}>
      <div className="[&_h1]:md:text-[80px]">
        <H1>Changelog</H1>
      </div>
      <div className="mt-12">
        <Empty
          framed
          icon={CloudOff}
          title="The release notes couldn't be loaded"
          description="They come from GitHub, which isn't responding right now."
        />
      </div>
      <Muted className="mt-6">
        You can read them on{" "}
        <a href={`${REPO}/releases`} className="text-fg underline underline-offset-4">
          GitHub Releases
        </a>
        .
      </Muted>
    </div>
  );
}

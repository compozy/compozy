/** The release page's side rail: nearby releases, sections, contributors and GitHub links. */
import { cn } from "@compozy/ui";
import { ArrowUpRight } from "@compozy/ui/icons";
import Link from "next/link";

import { HAIRLINE } from "@/components/site/layout";
import { Small } from "@/components/site/primitives";
import { sectionSlug } from "@/lib/changelog/timeline";
import type { ChangelogContributor } from "@/lib/changelog/types";

export interface TrailRelease {
  version: string;
  pre: boolean;
}

const RAIL_LINK =
  "inline-flex items-center gap-1 text-[14px] text-muted-foreground transition-colors hover:text-fg";

function Trail({ current, line }: { current: string; line: readonly TrailRelease[] }) {
  const index = line.findIndex(item => item.version === current);
  const near = line.slice(Math.max(0, index - 2), index + 3);
  return (
    <ol aria-label="Nearby releases" className="relative m-0 mb-8 list-none p-0">
      <span aria-hidden className="absolute top-2 bottom-2 left-[5px] w-0.5 bg-site-light-120" />
      {near.map(item => {
        const here = item.version === current;
        return (
          <li key={item.version} className="relative py-1.5 pl-7">
            <span
              aria-hidden
              className={cn(
                "absolute top-[13px] left-0 size-3 rounded-full border-2 border-[#0a0a0a]",
                here
                  ? "bg-[image:var(--gradient-brand)] shadow-[0_0_14px_2px_rgba(29,240,215,0.5)]"
                  : "bg-[#3a3a3a]"
              )}
            />
            <Link
              href={`/changelog/${encodeURIComponent(item.version)}`}
              aria-current={here ? "true" : undefined}
              className={cn(
                "cursor-pointer bg-transparent p-0 font-mono text-[13.5px] outline-none transition-colors hover:text-fg focus-visible:text-fg",
                here ? "text-fg" : "text-muted-foreground"
              )}
            >
              {item.version}
            </Link>
          </li>
        );
      })}
    </ol>
  );
}

export function ReleaseRail({
  version,
  line,
  groups,
  contributors,
  url,
  comparison,
}: {
  version: string;
  line: readonly TrailRelease[];
  groups: string[];
  contributors: readonly ChangelogContributor[];
  url: string;
  comparison: { version: string; url: string } | null;
}) {
  return (
    <aside className="lg:sticky lg:top-28 lg:self-start">
      <Trail current={version} line={line} />
      {groups.length > 0 ? (
        <nav aria-label="In this release" className="hidden flex-col gap-2.5 lg:flex">
          {groups.map(group => (
            <a
              key={group}
              href={`#${sectionSlug(group)}`}
              className="text-[14px] text-muted-foreground transition-colors hover:text-fg"
            >
              {group}
            </a>
          ))}
          <a
            href="#all-changes"
            className="text-[14px] text-muted-foreground transition-colors hover:text-fg"
          >
            Every merged change
          </a>
        </nav>
      ) : null}
      {contributors.length > 0 ? (
        <div className={cn(groups.length > 0 && "lg:mt-8 lg:border-t lg:pt-6", HAIRLINE)}>
          <Small>Contributors</Small>
          <div className="mt-3 flex flex-wrap gap-2">
            {contributors.map(person => (
              <a
                key={person.login}
                href={person.url}
                className="inline-flex items-center gap-2 text-[14px] text-fg"
              >
                <img src={person.avatarUrl} alt="" className="size-6 rounded-full" />
                {person.login}
              </a>
            ))}
          </div>
        </div>
      ) : null}
      <a href={url} className={cn("mt-6", RAIL_LINK)}>
        Release files on GitHub <ArrowUpRight aria-hidden className="size-3.5" />
      </a>
      {comparison ? (
        <a href={comparison.url} className={cn("mt-2 flex", RAIL_LINK)}>
          Compare with {comparison.version} <ArrowUpRight aria-hidden className="size-3.5" />
        </a>
      ) : null}
    </aside>
  );
}

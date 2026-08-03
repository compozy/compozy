import { Eyebrow } from "@compozy/ui";
import type { ChangelogRelease } from "@/lib/changelog/types";
import Link from "next/link";
import { DateStamp } from "./date-stamp";
import { MonoBadge } from "./mono-badge";

export interface ChangelogRailProps {
  releases: ChangelogRelease[];
}

export function ChangelogRail({ releases }: ChangelogRailProps) {
  const items = releases.slice(0, 4);
  return (
    <aside
      aria-label="Recent changelog releases"
      className="rounded-xl border border-line bg-canvas-soft p-5"
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="inline-block size-1.5 rounded-full bg-success" />
          <Eyebrow className="text-muted tracking-badge!">Changelog</Eyebrow>
        </div>
        <Link href="/changelog" className="text-xs text-subtle hover:text-fg">
          all versions →
        </Link>
      </div>
      <ul className="mt-5 flex flex-col">
        {items.map((release, idx) => (
          <li
            key={release.version}
            className={`flex items-start gap-3 py-3 ${
              idx < items.length - 1 ? "border-b border-line" : ""
            }`}
          >
            <Link
              href={`/changelog/${encodeURIComponent(release.version)}`}
              className="flex min-w-0 flex-1 gap-3"
            >
              <span className="w-20 shrink-0">
                <MonoBadge>{release.channel.toUpperCase()}</MonoBadge>
              </span>
              <span className="min-w-0 flex-1">
                <span className="block font-mono text-small-body leading-5 text-fg">
                  {release.version}
                </span>
                <DateStamp
                  date={release.publishedAt}
                  format="compact-year"
                  className="mt-1 block text-badge"
                />
              </span>
            </Link>
          </li>
        ))}
      </ul>
      <Link
        href="/changelog"
        className="mt-4 inline-flex h-8 w-full items-center justify-center rounded-lg border border-line font-sans text-xs font-medium text-fg transition-colors hover:bg-hover"
      >
        Open the changelog
      </Link>
    </aside>
  );
}
